import { describe, expect, it, vi } from "vitest";
import {
  Bone,
  Euler,
  Group,
  NumberKeyframeTrack,
  Quaternion,
  QuaternionKeyframeTrack,
} from "three";
import {
  VRM,
  VRMExpression,
  VRMExpressionManager,
  VRMHumanoid,
  VRMLookAt,
  VRMLookAtExpressionApplier,
  VRMLookAtRangeMap,
} from "@pixiv/three-vrm";
import { VRMAnimation } from "@pixiv/three-vrm-animation";
import { AvatarDirector } from "@/components/avatar-viewer/avatar-director";
import type { MotionLibrary } from "@/components/avatar-viewer/motions/motion-library";
import {
  MOTION_CLIPS,
  MOTION_NAMES,
  type MotionName,
} from "@/components/avatar-viewer/motions/vocabulary";

// A rig with the bones the clips move and the expressions the presets touch. Like a loaded VRM, the
// normalized rig the mixer animates hangs under the scene, and every raw bone has a distinct name.
function model() {
  const scene = new Group();
  const hips = new Bone();
  hips.name = "hips";
  hips.position.y = 1;
  scene.add(hips);
  const bone = (name: string) => {
    const node = new Bone();
    node.name = name;
    hips.add(node);
    return { node };
  };
  const humanoid = new VRMHumanoid({
    hips: { node: hips },
    spine: bone("spine"),
    chest: bone("chest"),
    neck: bone("neck"),
    head: bone("head"),
    leftUpperArm: bone("leftUpperArm"),
    leftLowerArm: bone("leftLowerArm"),
    leftHand: bone("leftHand"),
    rightUpperArm: bone("rightUpperArm"),
    rightLowerArm: bone("rightLowerArm"),
    rightHand: bone("rightHand"),
    leftUpperLeg: bone("leftUpperLeg"),
    leftLowerLeg: bone("leftLowerLeg"),
    leftFoot: bone("leftFoot"),
    rightUpperLeg: bone("rightUpperLeg"),
    rightLowerLeg: bone("rightLowerLeg"),
    rightFoot: bone("rightFoot"),
  });
  scene.add(humanoid.normalizedHumanBonesRoot);
  const expressionManager = new VRMExpressionManager();
  for (const name of [
    "happy",
    "relaxed",
    "surprised",
    "aa",
    "jawOpen",
    "blink",
    "browInnerUp",
    "browDownLeft",
    "cheekSquintLeft",
  ])
    expressionManager.registerExpression(new VRMExpression(name));
  return new VRM({
    scene,
    humanoid,
    expressionManager,
    meta: { metaVersion: "1", name: "rig", authors: [], licenseUrl: "" },
  });
}

/** A delta clip: `bone` turns by `radians` about X over `duration` s (a one-shot returns to rest; a loop holds). */
function clip(
  bone: "head" | "rightUpperArm" | "spine",
  radians: number,
  duration: number,
  loop = false,
) {
  const animation = new VRMAnimation();
  animation.duration = duration;
  animation.restHipsPosition.set(0, 1, 0);
  const at = (angle: number) =>
    new Quaternion().setFromEuler(new Euler(angle, 0, 0)).toArray();
  const times = loop ? [0, duration] : [0, duration / 2, duration];
  const values = loop
    ? [...at(radians), ...at(radians)]
    : [...at(0), ...at(radians), ...at(0)];
  animation.humanoidTracks.rotation.set(
    bone,
    new QuaternionKeyframeTrack(`${bone}.quaternion`, times, values),
  );
  return animation;
}

/** A library whose clips arrive when the test says so. */
function library(available: Partial<Record<MotionName, VRMAnimation | null>>) {
  const requested: MotionName[] = [];
  const lib: MotionLibrary & { requested: MotionName[] } = {
    requested,
    request: (name) => {
      requested.push(name);
    },
    take: (name) => (name in available ? available[name] : undefined),
  };
  return lib;
}

function playing(
  cues: { offset_ms: number; kind: "emotion" | "gesture"; name: string }[],
) {
  const audio = document.createElement("audio");
  const state = { currentTime: 0, paused: false, ended: false };
  for (const key of ["currentTime", "paused", "ended"] as const)
    Object.defineProperty(audio, key, {
      configurable: true,
      get: () => state[key],
    });
  return {
    state,
    speech: {
      audio,
      words: [],
      text: "Hello there. What drew you here?",
      cues,
    },
  };
}

describe("AvatarDirector performance cues", () => {
  it("requests the idle loop and the gestures of a turn, then plays each gesture on the audio clock over the idle", () => {
    const vrm = model();
    const lib = library({
      idle: clip("spine", 0.1, 2, true),
      greet: clip("rightUpperArm", 1, 1),
      nod: clip("head", 0.2, 1),
    });
    const director = new AvatarDirector(vrm, () => 0.5, lib);
    expect(lib.requested).toEqual(["idle"]);
    const { state, speech } = playing([
      { offset_ms: 0, kind: "emotion", name: "friendly" },
      { offset_ms: 0, kind: "gesture", name: "greet" },
      { offset_ms: 800, kind: "gesture", name: "nod" },
      { offset_ms: 800, kind: "emotion", name: "curious" },
    ]);
    director.update(0.016, { state: "speaking", speech });
    // The turn's gestures are requested first; once the idle arrives the rest of the set is prefetched.
    expect(lib.requested.slice(0, 3)).toEqual(["idle", "greet", "nod"]);
    expect(new Set(lib.requested)).toEqual(new Set(MOTION_NAMES));
    expect(director.clipNames()).toEqual(["idle", "greet", "nod"]);
    const arm = vrm.humanoid.getNormalizedBoneNode("rightUpperArm")!;
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    const spine = vrm.humanoid.getNormalizedBoneNode("spine")!;
    // Half way through the greet the arm is up; the idle keeps the spine (additive layering).
    for (let i = 0; i < 30; i++) {
      state.currentTime += 0.016;
      director.update(0.016, { state: "speaking", speech });
    }
    expect(arm.rotation.x).toBeGreaterThan(0.5);
    expect(head.rotation.x).toBeCloseTo(0, 2);
    expect(spine.rotation.x).toBeGreaterThan(0.05);
    // The nod fires once its offset is crossed, not before.
    state.currentTime = 0.79;
    director.update(0.016, { state: "speaking", speech });
    expect(head.rotation.x).toBeCloseTo(0, 2);
    state.currentTime = 0.81;
    for (let i = 0; i < 25; i++) {
      state.currentTime += 0.016;
      director.update(0.016, { state: "speaking", speech });
    }
    expect(head.rotation.x).toBeGreaterThan(0.1);
    // Emotion follows the cue in force: curious lifts the brows, friendly's smile has faded.
    expect(vrm.expressionManager!.getValue("browInnerUp")).toBeGreaterThan(0.2);
    expect(vrm.expressionManager!.getValue("happy")).toBeLessThan(0.1);
    director.dispose();
  });

  it("never lets a gesture or a preset touch the mouth, which stays with the lip-sync sample", () => {
    const vrm = model();
    const animation = clip("head", 0.3, 1);
    animation.expressionTracks.preset.set(
      "aa",
      new NumberKeyframeTrack("aa.weight", [0, 1], [1, 1]),
    );
    const lib = library({ nod: animation, idle: null });
    const director = new AvatarDirector(vrm, () => 0.5, lib);
    const { state, speech } = playing([
      { offset_ms: 0, kind: "emotion", name: "encouraging" },
      { offset_ms: 0, kind: "gesture", name: "nod" },
    ]);
    const mouth = { sample: () => ({ aa: 0.6, jawOpen: 0.4 }) };
    for (let i = 0; i < 20; i++) {
      state.currentTime += 0.016;
      director.update(0.016, { state: "speaking", speech, mouth });
    }
    const manager = vrm.expressionManager!;
    expect(manager.getValue("aa")).toBe(0.6);
    expect(manager.getValue("jawOpen")).toBe(0.4);
    expect(
      vrm.humanoid.getNormalizedBoneNode("head")!.rotation.x,
    ).toBeGreaterThan(0.05);
    // Encouraging goes through the upper-face route while the mouth is active.
    expect(manager.getValue("happy")).toBe(0);
    expect(manager.getValue("cheekSquintLeft")).toBeGreaterThan(0.05);
    director.dispose();
  });

  it("degrades to a procedural nod when the clip is missing and ignores unknown names", () => {
    const vrm = model();
    const lib = library({ idle: null, nod: null, greet: null });
    const director = new AvatarDirector(vrm, () => 0.5, lib);
    const { state, speech } = playing([
      { offset_ms: 0, kind: "gesture", name: "greet" },
      { offset_ms: 0, kind: "gesture", name: "nod" },
      { offset_ms: 0, kind: "gesture", name: "moonwalk" },
      { offset_ms: 0, kind: "emotion", name: "ecstatic" },
    ]);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    let max = 0;
    for (let i = 0; i < 25; i++) {
      director.update(0.016, { state: "speaking", speech });
      state.currentTime += 0.016;
      max = Math.max(max, head.rotation.x);
    }
    expect(max).toBeGreaterThan(0.02);
    head.rotation.x = max;
    expect(head.rotation.x).toBeLessThanOrEqual(0.046);
    expect(director.clipNames()).toEqual([]);
    expect(lib.requested).toEqual(["idle", "greet", "nod"]);
    director.dispose();
  });

  it("plays a cue whose clip was still loading once it arrives, unless it is stale by then", () => {
    const vrm = model();
    const available: Partial<Record<MotionName, VRMAnimation | null>> = {
      idle: null,
    };
    const lib = library(available);
    const director = new AvatarDirector(vrm, () => 0.5, lib);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    const { state, speech } = playing([
      { offset_ms: 0, kind: "gesture", name: "nod" },
    ]);
    // The nod fires while nod.vrma is in flight: nothing plays yet, and no procedural nod either.
    for (let i = 0; i < 10; i++) {
      director.update(0.016, { state: "speaking", speech });
      state.currentTime += 0.016;
    }
    expect(head.rotation.x).toBeCloseTo(0, 3);
    // The clip lands 160 ms later: the pending nod plays.
    available.nod = clip("head", 0.3, 0.6);
    let max = 0;
    for (let i = 0; i < 30; i++) {
      director.update(0.016, { state: "speaking", speech });
      state.currentTime += 0.016;
      max = Math.max(max, head.rotation.x);
    }
    expect(max).toBeGreaterThan(0.1);
    // A second turn whose greet arrives only after the stale window: dropped, not played late.
    const late = playing([{ offset_ms: 0, kind: "gesture", name: "greet" }]);
    director.update(0.016, { state: "speaking", speech: late.speech });
    for (let i = 0; i < 120; i++)
      director.update(0.016, { state: "speaking", speech: late.speech });
    available.greet = clip("rightUpperArm", 1, 0.6);
    for (let i = 0; i < 30; i++)
      director.update(0.016, { state: "speaking", speech: late.speech });
    expect(
      vrm.humanoid.getNormalizedBoneNode("rightUpperArm")!.rotation.x,
    ).toBeCloseTo(0, 3);
    director.dispose();
  });

  it("starts the cues over for a re-sent turn and drops them when the turn is interrupted", () => {
    const vrm = model();
    const lib = library({ nod: clip("head", 0.3, 0.5), idle: null });
    const director = new AvatarDirector(vrm, () => 0.5, lib);
    const first = playing([{ offset_ms: 0, kind: "gesture", name: "nod" }]);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    const peak = () => {
      let max = 0;
      for (let i = 0; i < 20; i++) {
        director.update(0.016, { state: "speaking", speech: current.speech });
        current.state.currentTime += 0.016;
        max = Math.max(max, head.rotation.x);
      }
      return max;
    };
    let current = first;
    expect(peak()).toBeGreaterThan(0.1);
    // Interrupted: the socket dropped mid-turn. Nothing lingers from the old turn.
    for (let i = 0; i < 60; i++)
      director.update(0.016, { state: "processing" });
    expect(head.rotation.x).toBeCloseTo(0, 2);
    // The reconnect re-sends the same turn as a new object with the audio at 0: the cues replay.
    current = playing(first.speech.cues);
    expect(peak()).toBeGreaterThan(0.1);
    director.dispose();
  });

  it("holds the thinking loop and the thinking face while the answer is processed", () => {
    const vrm = model();
    const lib = library({ thinking: clip("head", 0.25, 2, true), idle: null });
    const director = new AvatarDirector(vrm, () => 0.5, lib);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    for (let i = 0; i < 60; i++)
      director.update(0.016, { state: "processing", emotion: { happy: 1 } });
    expect(lib.requested).toContain("thinking");
    expect(head.rotation.x).toBeGreaterThan(0.2);
    expect(head.rotation.y).toBe(0);
    expect(vrm.expressionManager!.getValue("browDownLeft")).toBeGreaterThan(
      0.2,
    );
    expect(vrm.expressionManager!.getValue("happy")).toBeLessThan(0.05);
    // Back to listening: the hold fades out and the baseline emotion returns.
    for (let i = 0; i < 90; i++)
      director.update(0.016, { state: "listening", emotion: { happy: 1 } });
    expect(head.rotation.x).toBeCloseTo(0, 2);
    expect(vrm.expressionManager!.getValue("happy")).toBeGreaterThan(0.9);
    director.dispose();
  });
});

describe("AvatarDirector listening", () => {
  it("holds the listen pose and nods now and then while the Candidate speaks, never distracting", () => {
    const vrm = model();
    const lib = library({
      listen: clip("spine", 0.1, 3, true),
      nod: clip("head", 0.3, 1),
      lean_in: clip("spine", 0.2, 2),
      idle: null,
    });
    const random = vi.fn(() => 0);
    const director = new AvatarDirector(vrm, random, lib);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    const spine = vrm.humanoid.getNormalizedBoneNode("spine")!;
    const run = (seconds: number, candidateSpeaking: boolean) => {
      let nods = 0;
      let above = false;
      let max = 0;
      for (let i = 0; i < seconds * 60; i++) {
        director.update(1 / 60, { state: "listening", candidateSpeaking });
        max = Math.max(max, head.rotation.x);
        const now = head.rotation.x > 0.15;
        if (now && !above) nods++;
        above = now;
      }
      return { nods, max };
    };
    expect(run(2, true).nods).toBe(0);
    expect(spine.rotation.x).toBeGreaterThan(0.03);
    // random 0: nods at 2.5 s, 6.5 s and 10.5 s of speech; the listen weight keeps them subtle.
    const { nods, max } = run(10, true);
    expect(nods).toBe(3);
    expect(max).toBeLessThan(0.3);
    expect(lib.requested.slice(0, 4)).toEqual([
      "idle",
      "listen",
      "nod",
      "lean_in",
    ]);
    // Silence: the pose releases and nothing fires.
    expect(run(3, false).nods).toBe(0);
    expect(spine.rotation.x).toBeCloseTo(0, 2);
  });

  it("nods procedurally when the clip is missing, within the bound of the speech nod", () => {
    const vrm = model();
    const director = new AvatarDirector(
      vrm,
      () => 0,
      library({ idle: null, nod: null, listen: null, lean_in: null }),
    );
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    let max = 0;
    for (let i = 0; i < 4 * 60; i++) {
      director.update(1 / 60, { state: "listening", candidateSpeaking: true });
      max = Math.max(max, head.rotation.x);
    }
    expect(max).toBeGreaterThan(0.03);
    expect(max).toBeLessThanOrEqual(0.046);
    director.dispose();
  });

  it("plays no clips with reduced motion", () => {
    const vrm = model();
    const lib = library({
      listen: clip("spine", 0.3, 3, true),
      nod: clip("head", 0.3, 1),
      idle: clip("spine", 0.3, 2, true),
    });
    const director = new AvatarDirector(vrm, () => 0, lib);
    for (let i = 0; i < 5 * 60; i++)
      director.update(
        1 / 60,
        { state: "listening", candidateSpeaking: true },
        true,
      );
    expect(vrm.humanoid.getNormalizedBoneNode("spine")!.rotation.x).toBeCloseTo(
      0,
      6,
    );
    expect(vrm.humanoid.getNormalizedBoneNode("head")!.rotation.x).toBeCloseTo(
      0,
      6,
    );
    expect(MOTION_CLIPS.listen.loop).toBe(true);
    director.dispose();
  });
});

describe("AvatarDirector eyes", () => {
  it("keeps an over-scaled expression lookAt from saturating on small head motion, and restores it", () => {
    const base = model();
    const manager = base.expressionManager!;
    for (const name of ["lookLeft", "lookRight", "lookUp", "lookDown"])
      manager.registerExpression(new VRMExpression(name));
    // The Avaturn export: outputScale 10 over 90°, so 9° already gives the full look shape.
    const range = () => new VRMLookAtRangeMap(90, 10);
    const applier = new VRMLookAtExpressionApplier(
      manager,
      range(),
      range(),
      range(),
      range(),
    );
    const vrm = new VRM({
      scene: base.scene,
      humanoid: base.humanoid,
      expressionManager: manager,
      lookAt: new VRMLookAt(base.humanoid, applier),
      meta: { metaVersion: "1", name: "rig", authors: [], licenseUrl: "" },
    });
    const director = new AvatarDirector(vrm, () => 0.5, null);
    applier.applyYawPitch(9, 0);
    const look = Math.max(
      manager.getValue("lookLeft") ?? 0,
      manager.getValue("lookRight") ?? 0,
    );
    expect(look).toBeCloseTo(0.3, 2);
    applier.applyYawPitch(45, 0);
    expect(
      Math.max(
        manager.getValue("lookLeft") ?? 0,
        manager.getValue("lookRight") ?? 0,
      ),
    ).toBe(1);
    director.dispose();
    expect(applier.rangeMapHorizontalInner.inputMaxValue).toBe(90);
    expect(applier.rangeMapVerticalUp.outputScale).toBe(10);
  });
});
