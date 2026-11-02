import {
  AnimationMixer,
  LoopOnce,
  Object3D,
  type AnimationAction,
} from "three";
import {
  VRMExpression,
  VRMExpressionMorphTargetBind,
  type VRM,
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

export type PerformanceState = "idle" | "speaking" | "listening" | "processing";
export type Emotion = "happy" | "relaxed" | "sad" | "angry" | "surprised";
export type MouthSample = Readonly<Record<string, number>>;
/**
 * Sampled once per frame while speaking (lipsync/createInterviewerMouth reads audio.currentTime; no clock
 * here). Mouth, jaw and tongue channels win over emotion; brow, cheek and nose channels add lightly on top.
 * Any other name (eyes, blink) is ignored: those belong to the blink and gaze layers.
 */
export interface MouthSource {
  sample: () => MouthSample | null;
}
/** The turn being spoken; drives subtle head, gaze and brow motion from its word timings. */
export interface SpeechPerformance {
  audio: HTMLMediaElement;
  words: InterviewerTimelines["words"];
  text: string;
}
export interface AvatarPerformance {
  state: PerformanceState;
  candidateSpeaking?: boolean;
  emotion?: Partial<Record<Emotion, number>>;
  mouth?: MouthSource;
  speech?: SpeechPerformance;
}
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

/** One instance per loaded VRM. Owns animation state, never the model's GPU resources. */
export class AvatarDirector {
  private readonly mixer: AnimationMixer;
  private readonly clips = new Map<string, AnimationAction>();
  private idle: AnimationAction | undefined;
  private gesture: AnimationAction | undefined;
  private readonly blink: BlinkScheduler;
  private readonly gaze = new Object3D();
  private readonly previousLookAt: Object3D | null | undefined;
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
  private readonly upperEmotions = new Map<Emotion, VRMExpression>();
  private readonly emotionWeights = new Map<Emotion, number>();
  private readonly overrides = new Map<
    string,
    { mouth: "none" | "block" | "blend"; blink: "none" | "block" | "blend" }
  >();
  private readonly basePose: ReturnType<VRM["humanoid"]["getNormalizedPose"]>;
  constructor(
    private readonly vrm: VRM,
    random: () => number = Math.random,
  ) {
    this.mixer = new AnimationMixer(vrm.scene);
    this.blink = new BlinkScheduler(random);
    this.basePose = vrm.humanoid.getNormalizedPose();
    this.previousLookAt = vrm.lookAt?.target;
    if (vrm.lookAt) vrm.lookAt.target = this.gaze;
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
  }
  registerClip(
    name: string,
    animation: VRMAnimation,
    slot: "idle" | "gesture" = "gesture",
  ) {
    if (this.disposed) return;
    const old = this.clips.get(name);
    if (old) {
      old.stop();
      this.mixer.uncacheAction(old.getClip());
    }
    const action = this.mixer.clipAction(
      createVRMAnimationClip(animation, this.vrm),
    );
    this.clips.set(name, action);
    if (slot === "idle") {
      this.idle?.fadeOut(0.3);
      this.idle = action;
      action.reset().fadeIn(0.3).play();
    } else {
      action.setLoop(LoopOnce, 1);
      action.clampWhenFinished = false;
    }
  }
  triggerGesture(name: string, fadeSeconds = 0.3) {
    const action = this.clips.get(name);
    if (!action || this.disposed || action === this.idle) return false;
    this.gesture?.fadeOut(Math.max(0, fadeSeconds));
    action
      .reset()
      .setEffectiveWeight(1)
      .fadeIn(Math.max(0, fadeSeconds))
      .play();
    this.gesture = action;
    this.gestureFading = false;
    return true;
  }
  update(delta: number, input: AvatarPerformance, reducedMotion = false) {
    if (this.disposed) return;
    const dt = Number.isFinite(delta) ? Math.max(0, Math.min(delta, 0.05)) : 0;
    this.time += dt;
    // Reset the base before the mixer; procedural offsets never accumulate across frames.
    this.vrm.humanoid.setNormalizedPose(this.basePose);
    if (!reducedMotion) {
      this.mixer.update(dt);
      if (
        this.gesture &&
        !this.gestureFading &&
        this.gesture.time >= this.gesture.getClip().duration - 0.3
      ) {
        this.gesture.fadeOut(0.3);
        this.gestureFading = true;
      }
    }
    const bone = (
      name: Parameters<VRM["humanoid"]["getNormalizedBoneNode"]>[0],
    ) => this.vrm.humanoid.getNormalizedBoneNode(name);
    const chest = bone("chest") ?? bone("spine");
    const head = bone("head");
    const speech =
      input.state === "speaking"
        ? this.speechMotion(input.speech, reducedMotion)
        : null;
    if (!reducedMotion) {
      if (chest) {
        chest.rotation.x += Math.sin(this.time * 1.5) * 0.008;
        chest.rotation.z += Math.sin(this.time * 0.6) * 0.006;
      }
      if (head && input.state === "listening" && input.candidateSpeaking)
        head.rotation.x += Math.sin(this.time * 2.4) * 0.045;
      if (head && input.state === "processing") head.rotation.y += 0.12;
    }
    if (head && speech) {
      head.rotation.x += speech.pitch;
      head.rotation.z += speech.roll;
      head.rotation.y += speech.yaw;
    }
    const manager = this.vrm.expressionManager;
    const sample = input.state === "speaking" ? input.mouth?.sample() : null;
    const mouthActive = sample !== null && sample !== undefined;
    const smoothing = reducedMotion ? 1 : 1 - Math.exp(-dt * 8);
    for (const name of emotions) {
      const expression = manager?.getExpression(name);
      if (!expression) continue;
      // VRM overrideMouth suppresses vowels, not emotion. Clear that suppression while speaking,
      // and route emotion through upper-face-only binds so jaw/mouth cannot fight the source.
      expression.overrideMouth =
        input.state === "speaking"
          ? "none"
          : (this.overrides.get(name)?.mouth ?? "none");
      const target = clamp(input.emotion?.[name] ?? 0);
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
    for (const name of this.accentNames)
      manager?.setValue(
        name,
        clamp(
          (mouthActive ? (sample[name] ?? 0) : 0) +
            (speech ? speech.brow * (speechBrow[name] ?? 0) : 0),
        ),
      );
    if (this.vrm.lookAt) {
      this.gazeTime -= dt;
      if (this.gazeTime <= 0) {
        this.gazeTime = 2 + Math.random() * 3;
        this.gazeX = (Math.random() - 0.5) * 0.06;
        this.gazeY = (Math.random() - 0.5) * 0.03;
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
  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    this.mixer.stopAllAction();
    this.mixer.uncacheRoot(this.vrm.scene);
    this.clips.clear();
    this.vrm.humanoid.setNormalizedPose(this.basePose);
    if (this.vrm.lookAt) this.vrm.lookAt.target = this.previousLookAt;
    this.vrm.expressionManager?.resetValues();
    for (const expression of this.upperEmotions.values())
      this.vrm.expressionManager?.unregisterExpression(expression);
    this.upperEmotions.clear();
    this.emotionWeights.clear();
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
