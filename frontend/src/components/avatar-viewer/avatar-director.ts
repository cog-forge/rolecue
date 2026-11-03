import {
  AdditiveAnimationBlendMode,
  AnimationMixer,
  Euler,
  LoopOnce,
  LoopRepeat,
  Object3D,
  Quaternion,
  type AnimationAction,
} from "three";
import {
  VRMExpression,
  VRMExpressionMorphTargetBind,
  VRMLookAtExpressionApplier,
  type VRM,
  type VRMLookAtRangeMap,
} from "@pixiv/three-vrm";
import {
  createVRMAnimationClip,
  type VRMAnimation,
} from "@pixiv/three-vrm-animation";
import type { InterviewerTimelines } from "./lipsync/timelines";
import {
  planSpeechMotion,
  sampleSpeechMotion,
  type SpeechMotionFrame,
  type SpeechMotionPlan,
} from "./lipsync/speech-motion";
import { CueCursor, STALE_GESTURE_MS } from "./motions/cue-timeline";
import {
  sharedMotionLibrary,
  type MotionLibrary,
} from "./motions/motion-library";
import { EMOTION_PRESETS, type PresetEmotion } from "./motions/presets";
import {
  MOTION_CLIPS,
  MOTION_NAMES,
  isMotionName,
  type CueEmotion,
  type MotionName,
  type PerformanceCue,
} from "./motions/vocabulary";

export type PerformanceState = "idle" | "speaking" | "listening" | "processing";
export type Emotion = PresetEmotion;
export type MouthSample = Readonly<Record<string, number>>;
/**
 * Sampled once per frame while speaking (lipsync/createInterviewerMouth reads audio.currentTime; no clock
 * here). Mouth, jaw and tongue channels win over emotion; brow, cheek and nose channels add lightly on top.
 * Any other name (eyes, blink) is ignored: those belong to the blink and gaze layers.
 */
export interface MouthSource {
  sample: () => MouthSample | null;
}
/**
 * The turn being spoken; drives subtle head, gaze and brow motion from its word timings, and plays the
 * server's performance cues (KAN-250) on the audio clock.
 */
export interface SpeechPerformance {
  audio: HTMLMediaElement;
  words: InterviewerTimelines["words"];
  text: string;
  cues?: readonly PerformanceCue[];
}
export interface AvatarPerformance {
  state: PerformanceState;
  candidateSpeaking?: boolean;
  /** Baseline preset weights, used when no cue or state decides the emotion. */
  emotion?: Partial<Record<Emotion, number>>;
  mouth?: MouthSource;
  speech?: SpeechPerformance;
}
export type ClipSlot = "idle" | "gesture" | "loop";

const emotions: readonly Emotion[] = [
  "happy",
  "relaxed",
  "sad",
  "angry",
  "surprised",
];
const vowels = new Set(["aa", "ih", "ou", "ee", "oh"]);
export const isMouthChannel = (name: string) =>
  vowels.has(name) || /^(viseme_|mouth|jaw|tongue)/.test(name);
const isAccentChannel = (name: string) => /^(brow|cheek|nose)/.test(name);
const speechBrow: Readonly<Record<string, number>> = {
  browInnerUp: 1,
  browOuterUpLeft: 0.6,
  browOuterUpRight: 0.6,
};
// Reduced motion keeps speech motion but at a fraction of its amplitude.
const reducedSpeechMotion = 0.3;
const clamp = (n: number) =>
  Number.isFinite(n) ? Math.max(0, Math.min(1, n)) : 0;
/**
 * The eye angle, in degrees, at which an expression-type lookAt reaches its full look shape. Our
 * Avaturn → VRM exports declare `outputScale: 10` over 90°, which saturates the look expressions at 9°:
 * the small head sway of the idle then swings the eyes to their extremes. A weight is at most 1.
 */
const EYE_RANGE_DEG = 30;
const GESTURE_FADE = 0.3;
const HOLD_FADE = 0.45;
/** Listening: a nod every few seconds while the Candidate speaks, a lean-in once they have talked a while. */
const LISTEN_FIRST_NOD = [2.5, 4] as const;
const LISTEN_NOD_GAP = [4, 8] as const;
const LISTEN_LEAN_IN_AFTER = 9;
const LISTEN_WEIGHT = 0.6;
/** Procedural nod when the clip is missing: a single dip, amplitude under the speech nod. */
const PROCEDURAL_NOD_SECONDS = 0.6;
const PROCEDURAL_NOD_PITCH = 0.045;

/** Random wait → close → open, inspired by Amica autoBlink (MIT); see docs/avatar-lip-sync.md. No timers. */
export class BlinkScheduler {
  private elapsed = 0;
  private next: number;
  constructor(private readonly random: () => number = Math.random) {
    this.next = this.wait();
  }
  private wait() {
    return 2.5 + clamp(this.random()) * 4;
  }
  update(delta: number) {
    this.elapsed += delta;
    if (this.elapsed < this.next) return 0;
    const t = this.elapsed - this.next;
    if (t >= 0.18) {
      this.elapsed = 0;
      this.next = this.wait();
      return 0;
    }
    return t < 0.06 ? t / 0.06 : 1 - (t - 0.06) / 0.12;
  }
}

/**
 * One instance per loaded VRM. Owns animation state, never the model's GPU resources.
 *
 * Layers, in order, every frame (docs/avatar-performance.md): the base pose; the idle loop (absolute);
 * one held loop (listening, thinking) and one-shot gestures, both additive on top of the idle; procedural
 * breathing, listening nods, speech head motion and gaze; then the face: emotion from the cues or the
 * state, blink, and the mouth, which lip-sync owns outright. Clips carry no expression tracks.
 */
export class AvatarDirector {
  private readonly mixer: AnimationMixer;
  private readonly clips = new Map<string, AnimationAction>();
  private idle: AnimationAction | undefined;
  private gesture: AnimationAction | undefined;
  private held: { name: string; action: AnimationAction } | undefined;
  private readonly requested = new Set<MotionName>();
  /** Gesture cues that fired while their clip was still loading, with the director time they were due. */
  private readonly pendingGestures = new Map<MotionName, number>();
  private prefetched = false;
  private readonly blink: BlinkScheduler;
  private readonly gaze = new Object3D();
  private readonly previousLookAt: Object3D | null | undefined;
  private readonly eyeRanges: {
    map: VRMLookAtRangeMap;
    inputMaxValue: number;
    outputScale: number;
  }[] = [];
  private time = 0;
  private gazeTime = 0;
  private gazeX = 0;
  private gazeY = 0;
  private disposed = false;
  private gestureFading = false;
  private readonly mouthNames: string[];
  private readonly accentNames: string[];
  private speechPlan:
    | {
        words: SpeechPerformance["words"];
        text: string;
        plan: SpeechMotionPlan;
      }
    | undefined;
  private cueTrack:
    { speech: SpeechPerformance; cursor: CueCursor } | undefined;
  private cueEmotion: CueEmotion | null = null;
  /** A looping clip a cue asked for (listen) is held until the next gesture cue or the end of the turn. */
  private cueHold: MotionName | null = null;
  private listening = false;
  private listenClock = 0;
  private nextNod = 0;
  private leanedIn = false;
  private proceduralNod = Number.NEGATIVE_INFINITY;
  private readonly upperEmotions = new Map<Emotion, VRMExpression>();
  private readonly emotionWeights = new Map<Emotion, number>();
  private readonly accentWeights = new Map<string, number>();
  private readonly overrides = new Map<
    string,
    { mouth: "none" | "block" | "blend"; blink: "none" | "block" | "blend" }
  >();
  private readonly basePose: ReturnType<VRM["humanoid"]["getNormalizedPose"]>;
  /** Procedural rotations applied this frame, undone before the next mixer update. */
  private readonly offsets = new Map<Object3D, Quaternion>();
  private readonly scratch = {
    euler: new Euler(),
    quaternion: new Quaternion(),
  };
  /**
   * `motions` loads clips on demand; `null` plays no clips (tests, previews). The shared library is the
   * default so every director on the page reuses one download of each clip.
   */
  constructor(
    private readonly vrm: VRM,
    private readonly random: () => number = Math.random,
    private readonly motions: MotionLibrary | null = sharedMotionLibrary(),
  ) {
    this.mixer = new AnimationMixer(vrm.scene);
    this.blink = new BlinkScheduler(random);
    this.basePose = vrm.humanoid.getNormalizedPose();
    this.previousLookAt = vrm.lookAt?.target;
    if (vrm.lookAt) vrm.lookAt.target = this.gaze;
    this.normalizeEyeRange();
    const names = Object.keys(vrm.expressionManager?.expressionMap ?? {});
    this.mouthNames = names.filter(isMouthChannel);
    this.accentNames = names.filter(isAccentChannel);
    for (const name of emotions) {
      const expression = vrm.expressionManager?.getExpression(name);
      if (expression) {
        const upper = new VRMExpression(`kan58:upper:${name}`);
        for (const bind of expression.binds) {
          if (bind instanceof VRMExpressionMorphTargetBind) {
            const primitives = bind.primitives.filter((primitive) => {
              const target = Object.entries(
                primitive.morphTargetDictionary ?? {},
              ).find(([, index]) => index === bind.index)?.[0];
              return target !== undefined && !isMouthChannel(target);
            });
            if (primitives.length)
              upper.addBind(
                new VRMExpressionMorphTargetBind({
                  primitives,
                  index: bind.index,
                  weight: bind.weight,
                }),
              );
          }
        }
        vrm.expressionManager?.registerExpression(upper);
        this.upperEmotions.set(name, upper);
        this.overrides.set(name, {
          mouth: expression.overrideMouth,
          blink: expression.overrideBlink,
        });
      }
    }
    this.request("idle");
  }
  /**
   * Registers a clip. `idle` is the absolute base loop. `loop` clips hold a state (listening, thinking)
   * and `gesture` clips play once; both are authored as deltas from the base pose and played additively,
   * so they layer over the idle instead of replacing it. A registered name is replaced in place.
   */
  registerClip(
    name: string,
    animation: VRMAnimation,
    slot: ClipSlot = "gesture",
  ) {
    if (this.disposed) return;
    const old = this.clips.get(name);
    if (old) {
      old.stop();
      this.mixer.uncacheAction(old.getClip());
      if (this.gesture === old) this.gesture = undefined;
      if (this.held?.action === old) this.held = undefined;
    }
    const clip = createVRMAnimationClip(animation, this.vrm);
    // Gestures never touch the face: expression tracks stay with lip-sync and the emotion layer.
    clip.tracks = clip.tracks.filter(
      (track) => !/\.(weight|morphTargetInfluences)/.test(track.name),
    );
    const action = this.mixer.clipAction(
      clip,
      undefined,
      slot === "idle" ? undefined : AdditiveAnimationBlendMode,
    );
    this.clips.set(name, action);
    if (slot === "idle") {
      this.idle?.fadeOut(HOLD_FADE);
      this.idle = action;
      action.setLoop(LoopRepeat, Infinity).reset().fadeIn(HOLD_FADE).play();
      return;
    }
    if (slot === "loop") action.setLoop(LoopRepeat, Infinity);
    else {
      action.setLoop(LoopOnce, 1);
      action.clampWhenFinished = false;
    }
  }
  /** Plays a one-shot gesture over the idle. Reports false when the clip is not registered. */
  triggerGesture(name: string, fadeSeconds = GESTURE_FADE, weight = 1) {
    const action = this.clips.get(name);
    if (
      !action ||
      this.disposed ||
      action === this.idle ||
      action === this.held?.action
    )
      return false;
    const fade = Math.max(0, fadeSeconds);
    if (this.gesture && this.gesture !== action) this.gesture.fadeOut(fade);
    action.reset().setEffectiveWeight(clamp(weight)).fadeIn(fade).play();
    this.gesture = action;
    this.gestureFading = false;
    return true;
  }
  /** Holds a looping clip (listening, thinking) until another is held or `null` releases it. */
  private hold(name: string | null, weight = 1) {
    if (this.held?.name === name) return;
    this.held?.action.fadeOut(HOLD_FADE);
    this.held = undefined;
    if (name === null) return;
    const action = this.clips.get(name);
    if (!action || action === this.idle) return;
    action.reset().setEffectiveWeight(clamp(weight)).fadeIn(HOLD_FADE).play();
    this.held = { name, action };
  }
  private request(name: MotionName) {
    if (!this.motions || this.requested.has(name)) return;
    this.requested.add(name);
    this.motions.request(name);
  }
  /** Adopts clips the library has finished loading since the last frame. */
  private adoptLoaded() {
    if (!this.motions) return;
    for (const name of this.requested) {
      if (this.clips.has(name)) continue;
      const animation = this.motions.take(name);
      if (animation) {
        this.registerClip(
          name,
          animation,
          name === "idle"
            ? "idle"
            : MOTION_CLIPS[name].loop
              ? "loop"
              : "gesture",
        );
        // A cue that fired while this clip was loading still plays if it is not stale yet.
        const due = this.pendingGestures.get(name);
        this.pendingGestures.delete(name);
        if (due !== undefined && this.time - due <= STALE_GESTURE_MS / 1000)
          this.playCueGesture(name);
        if (name === "idle" && !this.prefetched) {
          // The rest of the set is small (under 70 KB); fetch it now so first-turn cues are not missed.
          this.prefetched = true;
          for (const other of MOTION_NAMES) this.request(other);
        }
      } else if (animation === null) {
        this.requested.delete(name);
        this.pendingGestures.delete(name);
      }
    }
  }
  update(delta: number, input: AvatarPerformance, reducedMotion = false) {
    if (this.disposed) return;
    const dt = Number.isFinite(delta) ? Math.max(0, Math.min(delta, 0.05)) : 0;
    this.time += dt;
    const speaking = input.state === "speaking";
    // Decide what this frame needs, adopt the clips that arrived, then hold and play.
    const cueFrame = speaking ? this.cues(input.speech) : this.clearCues();
    this.requestFor(input);
    this.adoptLoaded();
    this.direct(input, dt, reducedMotion);
    // Undo last frame's procedural offsets, never the mixer's pose: three's PropertyMixer skips a
    // write when a track's value did not change, so a held pose would vanish under a full reset.
    this.undoOffsets();
    if (reducedMotion) {
      this.vrm.humanoid.setNormalizedPose(this.basePose);
    } else {
      for (const gesture of cueFrame.gestures) this.playCueGesture(gesture);
      this.mixer.update(dt);
      if (
        this.gesture &&
        !this.gestureFading &&
        this.gesture.time >= this.gesture.getClip().duration - GESTURE_FADE
      ) {
        this.gesture.fadeOut(GESTURE_FADE);
        this.gestureFading = true;
      }
    }
    const bone = (
      name: Parameters<VRM["humanoid"]["getNormalizedBoneNode"]>[0],
    ) => this.vrm.humanoid.getNormalizedBoneNode(name);
    const chest = bone("chest") ?? bone("spine");
    const head = bone("head");
    const speech = speaking
      ? this.speechMotion(input.speech, reducedMotion)
      : null;
    const headOffset = { x: 0, y: 0, z: 0 };
    if (!reducedMotion) {
      // Breathing stays procedural so it continues when the idle clip is missing or still loading.
      if (chest && !this.idle)
        this.offset(
          chest,
          Math.sin(this.time * 1.5) * 0.008,
          0,
          Math.sin(this.time * 0.6) * 0.006,
        );
      const nod = (this.time - this.proceduralNod) / PROCEDURAL_NOD_SECONDS;
      if (nod >= 0 && nod < 1)
        headOffset.x += Math.sin(Math.PI * nod) * PROCEDURAL_NOD_PITCH;
      if (input.state === "processing" && !this.held) headOffset.y += 0.12;
    }
    if (speech) {
      headOffset.x += speech.pitch;
      headOffset.z += speech.roll;
      headOffset.y += speech.yaw;
    }
    if (head) this.offset(head, headOffset.x, headOffset.y, headOffset.z);
    const manager = this.vrm.expressionManager;
    const sample = speaking ? input.mouth?.sample() : null;
    const mouthActive = sample !== null && sample !== undefined;
    const smoothing = reducedMotion ? 1 : 1 - Math.exp(-dt * 8);
    const preset = this.emotionTarget(input, cueFrame.emotion);
    for (const name of emotions) {
      const expression = manager?.getExpression(name);
      if (!expression) continue;
      // VRM overrideMouth suppresses vowels, not emotion. Clear that suppression while speaking,
      // and route emotion through upper-face-only binds so jaw/mouth cannot fight the source.
      expression.overrideMouth = speaking
        ? "none"
        : (this.overrides.get(name)?.mouth ?? "none");
      const target = clamp(preset.expressions[name] ?? 0);
      const previous = this.emotionWeights.get(name) ?? 0;
      const weight = previous + (target - previous) * smoothing;
      this.emotionWeights.set(name, weight);
      manager?.setValue(name, mouthActive ? 0 : weight);
      const upper = this.upperEmotions.get(name);
      if (upper) upper.weight = mouthActive ? weight : 0;
    }
    manager?.setValue("blink", reducedMotion ? 0 : this.blink.update(dt));
    for (const name of this.mouthNames)
      manager?.setValue(name, mouthActive ? clamp(sample[name] ?? 0) : 0);
    for (const name of this.accentNames) {
      const target = clamp(preset.accents[name] ?? 0);
      const previous = this.accentWeights.get(name) ?? 0;
      const accent = previous + (target - previous) * smoothing;
      this.accentWeights.set(name, accent);
      manager?.setValue(
        name,
        clamp(
          (mouthActive ? (sample[name] ?? 0) : 0) +
            (speech ? speech.brow * (speechBrow[name] ?? 0) : 0) +
            accent,
        ),
      );
    }
    if (this.vrm.lookAt) {
      this.gazeTime -= dt;
      if (this.gazeTime <= 0) {
        this.gazeTime = 2 + this.random() * 3;
        this.gazeX = (this.random() - 0.5) * 0.06;
        this.gazeY = (this.random() - 0.5) * 0.03;
      }
      const headPosition = bone("head");
      if (headPosition) headPosition.getWorldPosition(this.gaze.position);
      this.gaze.position.x += reducedMotion
        ? 0
        : input.state === "processing"
          ? 0.6
          : this.gazeX;
      this.gaze.position.y += reducedMotion ? 0 : this.gazeY;
      if (speech) {
        this.gaze.position.x += speech.gazeX;
        this.gaze.position.y += speech.gazeY;
      }
      this.gaze.position.z += 3;
      this.gaze.updateMatrixWorld();
    }
    this.vrm.update(dt);
  }
  /** Asks the library for the clips the state will need, ahead of the frame that plays them. */
  private requestFor(input: AvatarPerformance) {
    if (input.state === "listening" && input.candidateSpeaking) {
      this.request("listen");
      this.request("nod");
      this.request("lean_in");
    }
    if (input.state === "processing") this.request("thinking");
  }
  /** State-driven motion: the held loop and the listening reactions. */
  private direct(input: AvatarPerformance, dt: number, reducedMotion: boolean) {
    const listening =
      input.state === "listening" && input.candidateSpeaking === true;
    if (listening && !this.listening) {
      this.listenClock = 0;
      this.nextNod = this.between(LISTEN_FIRST_NOD);
      this.leanedIn = false;
    }
    this.listening = listening;
    if (reducedMotion) {
      this.hold(null);
      return;
    }
    if (input.state === "processing") this.hold("thinking");
    else if (listening) this.hold("listen", LISTEN_WEIGHT);
    else if (input.state === "speaking" && this.cueHold)
      this.hold(this.cueHold);
    else this.hold(null);
    if (!listening) return;
    this.listenClock += dt;
    if (this.listenClock >= this.nextNod) {
      this.nextNod = this.listenClock + this.between(LISTEN_NOD_GAP);
      if (!this.triggerGesture("nod", GESTURE_FADE, LISTEN_WEIGHT))
        this.proceduralNod = this.time;
    }
    if (!this.leanedIn && this.listenClock >= LISTEN_LEAN_IN_AFTER) {
      this.leanedIn = true;
      this.triggerGesture("lean_in", HOLD_FADE, LISTEN_WEIGHT);
    }
  }
  private between([low, high]: readonly [number, number]) {
    return low + clamp(this.random()) * (high - low);
  }
  /** Follows the turn's cues on the audio clock; a new turn (or a reconnect re-send) starts over. */
  private cues(speech: SpeechPerformance | undefined) {
    if (!speech) return this.clearCues();
    if (this.cueTrack?.speech !== speech) {
      const cursor = new CueCursor(speech.cues ?? []);
      this.cueTrack = { speech, cursor };
      this.cueEmotion = null;
      this.cueHold = null;
      this.pendingGestures.clear();
      for (const name of cursor.gestureNames()) this.request(name);
    }
    if (speech.audio.paused || speech.audio.ended)
      return { emotion: this.cueEmotion, gestures: [] };
    const frame = this.cueTrack.cursor.advance(speech.audio.currentTime * 1000);
    this.cueEmotion = frame.emotion;
    return frame;
  }
  private clearCues() {
    this.cueTrack = undefined;
    this.cueEmotion = null;
    this.cueHold = null;
    this.pendingGestures.clear();
    return { emotion: null, gestures: [] };
  }
  /**
   * A one-shot clip plays over the idle; a looping clip (listen) is held instead. A clip still loading
   * is played when it arrives, unless it is stale by then; an unavailable nod degrades to a procedural
   * nod and any other unavailable clip to nothing.
   */
  private playCueGesture(name: MotionName) {
    this.cueHold = MOTION_CLIPS[name].loop ? name : null;
    if (this.cueHold) {
      this.hold(this.cueHold);
      return;
    }
    if (this.triggerGesture(name)) return;
    if (this.motions && this.requested.has(name) && !this.clips.has(name)) {
      this.pendingGestures.set(name, this.time);
      return;
    }
    if (name === "nod") this.proceduralNod = this.time;
  }
  /** A cue wins; then the state (thinking while the answer is processed); then the baseline the room sets. */
  private emotionTarget(
    input: AvatarPerformance,
    cue: CueEmotion | null,
  ): {
    expressions: Partial<Record<Emotion, number>>;
    accents: Readonly<Record<string, number>>;
  } {
    if (cue) return EMOTION_PRESETS[cue];
    if (input.state === "processing") return EMOTION_PRESETS.thinking;
    if (input.emotion) return { expressions: input.emotion, accents: {} };
    return EMOTION_PRESETS.neutral;
  }
  private speechMotion(
    speech: SpeechPerformance | undefined,
    reducedMotion: boolean,
  ): SpeechMotionFrame | null {
    if (!speech || speech.audio.paused || speech.audio.ended) return null;
    if (
      this.speechPlan?.words !== speech.words ||
      this.speechPlan.text !== speech.text
    )
      this.speechPlan = {
        words: speech.words,
        text: speech.text,
        plan: planSpeechMotion(speech.words, speech.text),
      };
    return sampleSpeechMotion(
      this.speechPlan.plan,
      speech.audio.currentTime * 1000,
      reducedMotion ? reducedSpeechMotion : 1,
    );
  }
  /** Turns `node` by the Euler offset on top of the mixer's pose, remembering it for `undoOffsets`. */
  private offset(node: Object3D, x: number, y: number, z: number) {
    if (x === 0 && y === 0 && z === 0) return;
    const { euler, quaternion } = this.scratch;
    quaternion.setFromEuler(euler.set(x, y, z));
    node.quaternion.multiply(quaternion);
    const applied = this.offsets.get(node) ?? new Quaternion();
    applied.copy(quaternion);
    this.offsets.set(node, applied);
  }
  private undoOffsets() {
    for (const [node, applied] of this.offsets)
      node.quaternion.multiply(applied.invert());
    this.offsets.clear();
  }
  /**
   * An expression weight is 0..1, so an output scale above 1 only makes the eyes saturate early. Map
   * the full look shape to EYE_RANGE_DEG instead; `dispose` restores the model's values.
   */
  private normalizeEyeRange() {
    const applier = this.vrm.lookAt?.applier;
    if (!(applier instanceof VRMLookAtExpressionApplier)) return;
    for (const map of [
      applier.rangeMapHorizontalInner,
      applier.rangeMapHorizontalOuter,
      applier.rangeMapVerticalDown,
      applier.rangeMapVerticalUp,
    ]) {
      if (!(map.outputScale > 1)) continue;
      this.eyeRanges.push({
        map,
        inputMaxValue: map.inputMaxValue,
        outputScale: map.outputScale,
      });
      map.inputMaxValue = EYE_RANGE_DEG;
      map.outputScale = 1;
    }
  }
  /** Names of the clips registered so far (for tests and diagnostics). */
  clipNames() {
    return [...this.clips.keys()].filter(isMotionName);
  }
  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    this.mixer.stopAllAction();
    this.mixer.uncacheRoot(this.vrm.scene);
    this.clips.clear();
    this.idle = this.gesture = this.held = undefined;
    this.cueTrack = undefined;
    this.offsets.clear();
    this.vrm.humanoid.setNormalizedPose(this.basePose);
    if (this.vrm.lookAt) this.vrm.lookAt.target = this.previousLookAt;
    for (const { map, inputMaxValue, outputScale } of this.eyeRanges) {
      map.inputMaxValue = inputMaxValue;
      map.outputScale = outputScale;
    }
    this.eyeRanges.length = 0;
    this.vrm.expressionManager?.resetValues();
    for (const expression of this.upperEmotions.values())
      this.vrm.expressionManager?.unregisterExpression(expression);
    this.upperEmotions.clear();
    this.emotionWeights.clear();
    this.accentWeights.clear();
    this.speechPlan = undefined;
    for (const [name, override] of this.overrides) {
      const expression = this.vrm.expressionManager?.getExpression(name);
      if (expression) {
        expression.overrideMouth = override.mouth;
        expression.overrideBlink = override.blink;
      }
    }
  }
}
