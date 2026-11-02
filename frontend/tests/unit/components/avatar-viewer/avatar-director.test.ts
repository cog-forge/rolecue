import { describe, expect, it, vi } from "vitest";
import { Bone, Group, Mesh, VectorKeyframeTrack } from "three";
import {
  VRM,
  VRMExpression,
  VRMExpressionManager,
  VRMHumanoid,
  VRMExpressionMorphTargetBind,
} from "@pixiv/three-vrm";
import { VRMAnimation } from "@pixiv/three-vrm-animation";
import {
  AvatarDirector,
  BlinkScheduler,
} from "@/components/avatar-viewer/avatar-director";
import fixture from "@tests/fixtures/interviewer-turn-timelines.json";

function model() {
  const scene = new Group();
  const hips = new Bone();
  hips.position.y = 1;
  scene.add(hips);
  const bone = () => {
    const node = new Bone();
    hips.add(node);
    return { node };
  };
  const humanoid = new VRMHumanoid({
    hips: { node: hips },
    spine: bone(),
    head: bone(),
    leftUpperArm: bone(),
    leftLowerArm: bone(),
    leftHand: bone(),
    rightUpperArm: bone(),
    rightLowerArm: bone(),
    rightHand: bone(),
    leftUpperLeg: bone(),
    leftLowerLeg: bone(),
    leftFoot: bone(),
    rightUpperLeg: bone(),
    rightLowerLeg: bone(),
    rightFoot: bone(),
  });
  const expressionManager = new VRMExpressionManager();
  for (const name of [
    "happy",
    "surprised",
    "relaxed",
    "aa",
    "viseme_PP",
    "jawOpen",
    "blink",
  ])
    expressionManager.registerExpression(new VRMExpression(name));
  return new VRM({
    scene,
    humanoid,
    expressionManager,
    meta: { metaVersion: "0" },
  });
}

describe("BlinkScheduler", () => {
  it("waits, closes and opens continuously, then schedules the next blink", () => {
    const random = vi.fn(() => 0);
    const blink = new BlinkScheduler(random);
    expect(blink.update(2.49)).toBe(0);
    expect(blink.update(0.04)).toBeCloseTo(0.5);
    expect(blink.update(0.03)).toBeCloseTo(1);
    expect(blink.update(0.06)).toBeCloseTo(0.5);
    expect(blink.update(0.061)).toBe(0);
    expect(random).toHaveBeenCalledTimes(2);
    expect(blink.update(1)).toBe(0);
  });
});
describe("AvatarDirector", () => {
  it("smooths emotion and gives mouth channels precedence only with a speaking sample", () => {
    const vrm = model();
    const director = new AvatarDirector(vrm);
    const manager = vrm.expressionManager!;
    manager.getExpression("happy")!.overrideMouth = "block";
    director.update(0.05, { state: "listening", emotion: { happy: 1 } });
    expect(manager.getValue("happy")).toBeGreaterThan(0);
    expect(manager.getValue("happy")).toBeLessThan(1);
    director.update(0.05, {
      state: "speaking",
      emotion: { happy: 1 },
      mouth: {
        sample: () => ({ aa: 0.7, jawOpen: 0.4, viseme_PP: 10, blink: 1 }),
      },
    });
    expect(manager.getValue("happy")).toBe(0);
    expect(manager.getValue("aa")).toBe(0.7);
    expect(manager.getValue("viseme_PP")).toBe(1);
    expect(manager.getValue("blink")).toBe(0);
    director.update(0.05, { state: "processing" });
    expect(manager.getValue("aa")).toBe(0);
    expect(manager.getValue("jawOpen")).toBe(0);
    director.dispose();
  });
  it("preserves upper-face emotion while mouth samples own the jaw, then removes temporary expressions", () => {
    const vrm = model();
    const mesh = new Mesh();
    mesh.morphTargetDictionary = { browInnerUp: 0, mouthSmileLeft: 1 };
    mesh.morphTargetInfluences = [0, 0];
    const manager = vrm.expressionManager!;
    const happy = manager.getExpression("happy")!;
    happy.addBind(
      new VRMExpressionMorphTargetBind({
        primitives: [mesh],
        index: 0,
        weight: 1,
      }),
    );
    happy.addBind(
      new VRMExpressionMorphTargetBind({
        primitives: [mesh],
        index: 1,
        weight: 1,
      }),
    );
    happy.overrideMouth = "block";
    const initial = manager.expressions.length;
    const director = new AvatarDirector(vrm);
    director.update(0.05, {
      state: "speaking",
      emotion: { happy: 1 },
      mouth: { sample: () => ({ aa: 0.8 }) },
    });
    expect(mesh.morphTargetInfluences[0]).toBeGreaterThan(0);
    expect(mesh.morphTargetInfluences[1]).toBe(0);
    expect(manager.getValue("aa")).toBe(0.8);
    expect(happy.overrideMouth).toBe("none");
    director.dispose();
    expect(happy.overrideMouth).toBe("block");
    expect(manager.expressions).toHaveLength(initial);
  });
  it("never calls the mouth provider outside speaking; clears stale and non-finite values", () => {
    const vrm = model();
    const director = new AvatarDirector(vrm);
    const sample = vi.fn(() => ({ aa: Number.NaN, jawOpen: -1 }));
    director.update(0.05, { state: "listening", mouth: { sample } });
    expect(sample).not.toHaveBeenCalled();
    director.update(0.05, { state: "speaking", mouth: { sample } });
    expect(vrm.expressionManager?.getValue("aa")).toBe(0);
    expect(vrm.expressionManager?.getValue("jawOpen")).toBe(0);
    director.dispose();
  });
  it("keeps pose bounded across a simulated 30 minute interview and restores it on disposal", () => {
    const vrm = model();
    const director = new AvatarDirector(vrm, () => 0);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    for (let i = 0; i < 30 * 60 * 30; i++)
      director.update(1 / 30, { state: "listening", candidateSpeaking: true });
    expect(Math.abs(head.rotation.x)).toBeLessThan(0.046);
    director.update(
      0.05,
      { state: "processing", emotion: { relaxed: 1 } },
      true,
    );
    expect(vrm.expressionManager?.getValue("relaxed")).toBe(1);
    expect(head.rotation.y).toBe(0);
    expect(vrm.expressionManager?.getValue("blink")).toBe(0);
    director.dispose();
    director.dispose();
    const update = vi.spyOn(vrm, "update");
    director.update(1, { state: "speaking" });
    expect(update).not.toHaveBeenCalled();
    expect(head.rotation.x).toBeCloseTo(0);
  });
  it("supports the empty gesture slot, retargets registered VRMA and releases mixer bindings", () => {
    const vrm = model();
    const director = new AvatarDirector(vrm);
    expect(director.triggerGesture("nod")).toBe(false);
    const animation = new VRMAnimation();
    animation.duration = 1;
    animation.restHipsPosition.set(0, 1, 0);
    animation.humanoidTracks.translation.set(
      "hips",
      new VectorKeyframeTrack("hips.position", [0, 1], [0, 1, 0, 0, 1.01, 0]),
    );
    director.registerClip("idle", animation, "idle");
    director.registerClip("nod", animation);
    expect(director.triggerGesture("nod")).toBe(true);
    director.update(0.05, { state: "idle" });
    director.dispose();
    expect(director.triggerGesture("nod")).toBe(false);
    const remounted = new AvatarDirector(vrm);
    remounted.update(0.05, { state: "idle" });
    remounted.dispose();
  });
});

describe("AvatarDirector speech performance", () => {
  function speakingModel() {
    const vrm = model();
    for (const name of [
      "browInnerUp",
      "browOuterUpLeft",
      "cheekSquintLeft",
      "eyeBlinkLeft",
    ])
      vrm.expressionManager!.registerExpression(new VRMExpression(name));
    return vrm;
  }
  function playing(currentTime: number) {
    const audio = document.createElement("audio");
    const state = { currentTime, paused: false, ended: false };
    for (const key of ["currentTime", "paused", "ended"] as const)
      Object.defineProperty(audio, key, {
        configurable: true,
        get: () => state[key],
      });
    return {
      state,
      speech: { audio, words: fixture.words, text: fixture.text },
    };
  }
  // "CSS-in-JS" is a stressed word; its nod peaks 190 ms after it starts 60 ms early.
  const nodPeak = (1413 - 60 + 190) / 1000;

  it("layers word-timed head and brow motion over a mouth that keeps precedence", () => {
    const vrm = speakingModel();
    const manager = vrm.expressionManager!;
    const happy = manager.getExpression("happy")!;
    happy.overrideMouth = "block";
    const director = new AvatarDirector(vrm);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    const { state, speech } = playing(5.1);
    const mouth = {
      sample: () => ({
        jawOpen: 0.5,
        browInnerUp: 0.1,
        cheekSquintLeft: 0.05,
        eyeBlinkLeft: 1,
      }),
    };
    director.update(0.016, {
      state: "speaking",
      emotion: { happy: 1 },
      mouth,
      speech,
    });
    expect(happy.overrideMouth).toBe("none");
    expect(manager.getValue("happy")).toBe(0);
    expect(manager.getValue("jawOpen")).toBe(0.5);
    // Azure's light brow plus the end-of-question lift.
    expect(manager.getValue("browInnerUp")).toBeCloseTo(0.5);
    expect(manager.getValue("browOuterUpLeft")).toBeCloseTo(0.24);
    expect(manager.getValue("cheekSquintLeft")).toBeCloseTo(0.05);
    // Eye channels belong to the blink layer, whatever the source sends.
    expect(manager.getValue("eyeBlinkLeft")).toBe(0);

    state.currentTime = nodPeak;
    director.update(0.016, { state: "speaking", mouth, speech });
    expect(head.rotation.x).toBeCloseTo(0.045, 3);
    director.update(0.016, { state: "speaking", mouth, speech }, true);
    expect(head.rotation.x).toBeCloseTo(0.045 * 0.3, 3);

    state.paused = true;
    director.update(0.016, { state: "speaking", speech });
    expect(head.rotation.x).toBeCloseTo(0);
    expect(manager.getValue("browInnerUp")).toBe(0);
    expect(manager.getValue("jawOpen")).toBe(0);
    director.dispose();
    expect(happy.overrideMouth).toBe("block");
  });
  it("performs speech only in the speaking state", () => {
    const vrm = speakingModel();
    const director = new AvatarDirector(vrm);
    const head = vrm.humanoid.getNormalizedBoneNode("head")!;
    const { speech } = playing(5.1);
    director.update(0.016, { state: "speaking", speech });
    expect(vrm.expressionManager!.getValue("browInnerUp")).toBeCloseTo(0.4);
    director.update(0.016, { state: "listening", speech });
    expect(vrm.expressionManager!.getValue("browInnerUp")).toBe(0);
    expect(head.rotation.x).toBeCloseTo(0);
    director.dispose();
  });
});
