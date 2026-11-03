/**
 * Converts Mixamo FBX clips into compact .vrma files for `public/motions/` (KAN-251).
 *
 * The retargeting follows the three-vrm `humanoidAnimation` example (`loadMixamoAnimation.js`,
 * `mixamoVRMRigMap.js`, MIT, © pixiv Inc.): each Mixamo bone's rotation is taken into world space with
 * its rest pose and back into the VRM parent's space, which gives VRM normalized-space rotations. The
 * runtime never sees an FBX; raw FBX files are not committed (see `public/motions/CREDITS.md`).
 *
 * On top of the example:
 * - The hips are dropped (rotation and translation): the interviewer stands still, so the feet can
 *   never slide and the legs never move. Legs are not converted either. The hips rotation is folded
 *   into the spine, so the torso keeps the orientation the clip balanced it to.
 * - Every clip except the idle loop is stored as a delta from a reference pose (the idle's first
 *   frame), because the director plays them additively over the idle: `idle · delta ≈ clip`.
 * - Tracks are resampled to a fixed rate and keys that a straight interpolation reproduces are removed,
 *   to stay inside the per-clip budgets.
 * - One-shot clips ease in from and out to the reference pose; loops blend their last frames into the
 *   first so the seam does not pop.
 */
import { Quaternion, QuaternionLinearInterpolant, type Object3D } from "three";
import { FBXLoader } from "three/examples/jsm/loaders/FBXLoader.js";
import type { HumanoidBone, MotionClip, Track } from "./vrma-writer";

const fingerMap: Record<string, string> = {
  Thumb1: "ThumbMetacarpal",
  Thumb2: "ThumbProximal",
  Thumb3: "ThumbDistal",
  Index1: "IndexProximal",
  Index2: "IndexIntermediate",
  Index3: "IndexDistal",
  Middle1: "MiddleProximal",
  Middle2: "MiddleIntermediate",
  Middle3: "MiddleDistal",
  Ring1: "RingProximal",
  Ring2: "RingIntermediate",
  Ring3: "RingDistal",
  Pinky1: "LittleProximal",
  Pinky2: "LittleIntermediate",
  Pinky3: "LittleDistal",
};

/** Mixamo rig names to VRM humanoid bones (upper body and fingers only; no hips, no legs). */
export const mixamoVRMRigMap: Record<string, HumanoidBone> = {
  mixamorigSpine: "spine",
  mixamorigSpine1: "chest",
  mixamorigSpine2: "upperChest",
  mixamorigNeck: "neck",
  mixamorigHead: "head",
  mixamorigLeftShoulder: "leftShoulder",
  mixamorigLeftArm: "leftUpperArm",
  mixamorigLeftForeArm: "leftLowerArm",
  mixamorigLeftHand: "leftHand",
  mixamorigRightShoulder: "rightShoulder",
  mixamorigRightArm: "rightUpperArm",
  mixamorigRightForeArm: "rightLowerArm",
  mixamorigRightHand: "rightHand",
  ...Object.fromEntries(
    (["Left", "Right"] as const).flatMap((side) =>
      Object.entries(fingerMap).map(([mixamo, vrm]) => [
        `mixamorig${side}Hand${mixamo}`,
        `${side.toLowerCase()}${vrm}` as HumanoidBone,
      ]),
    ),
  ),
};

/** Rotation samples per bone at a fixed rate, in VRM normalized space. */
export type Sampled = {
  fps: number;
  duration: number;
  bones: Map<HumanoidBone, Quaternion[]>;
};

/** Reads an FBX and samples every mapped bone at `fps` over [start, end]. */
export function sampleMixamo(
  fbx: ArrayBuffer,
  {
    fps = 20,
    start = 0,
    end,
  }: { fps?: number; start?: number; end?: number } = {},
): Sampled {
  const asset = new FBXLoader().parse(fbx, "");
  const clip = asset.animations[0];
  if (!clip) throw new Error("the FBX carries no animation");
  asset.updateMatrixWorld(true);
  const stop = Math.min(end ?? clip.duration, clip.duration);
  if (!(stop > start)) throw new Error("the trim window is empty");
  const count = Math.round((stop - start) * fps) + 1;
  const bones = new Map<HumanoidBone, Quaternion[]>();
  let hips: Quaternion[] | undefined;
  for (const track of clip.tracks) {
    const [mixamoName, property] = track.name.split(".");
    const bone =
      mixamoName === "mixamorigHips" ? "hips" : mixamoVRMRigMap[mixamoName];
    const node = asset.getObjectByName(mixamoName) as Object3D | undefined;
    if (!bone || property !== "quaternion" || !node?.parent) continue;
    const restInverse = node.getWorldQuaternion(new Quaternion()).invert();
    const parentRest = node.parent.getWorldQuaternion(new Quaternion());
    const interpolant = new QuaternionLinearInterpolant(
      track.times,
      track.values,
      4,
      new Float32Array(4),
    );
    const samples: Quaternion[] = [];
    for (let i = 0; i < count; i++) {
      const t = Math.min(stop, start + i / fps);
      const value = interpolant.evaluate(t);
      samples.push(
        new Quaternion()
          .fromArray(value)
          .normalize()
          .premultiply(parentRest)
          .multiply(restInverse),
      );
    }
    if (mixamoName === "mixamorigHips") hips = samples;
    else bones.set(bone, samples);
  }
  // The hips are not animated (the feet must not slide), but Mixamo balances the torso against them:
  // fold the hips rotation into the spine so the upper body keeps its world orientation.
  const spine = bones.get("spine");
  if (hips && spine) spine.forEach((q, i) => q.premultiply(hips![i]));
  return { fps, duration: (count - 1) / fps, bones };
}

/** The first frame of a sampled clip: the reference pose deltas are taken from. */
export function firstFrame(sampled: Sampled) {
  return new Map([...sampled.bones].map(([bone, q]) => [bone, q[0].clone()]));
}

const smooth = (u: number) => u * u * (3 - 2 * u);

/**
 * Builds the clip. With `reference`, rotations become deltas (`reference⁻¹ · q`); `ease` seconds at
 * each end slerp the delta to identity (one-shots), `seam` seconds blend the end into the start (loops).
 * `amount` scales a delta (0..1) to tone a gesture down.
 */
export function buildMixamoClip(
  name: string,
  sampled: Sampled,
  {
    reference,
    ease = 0,
    seam = 0,
    amount = 1,
    boneAmount,
    tolerance = 0.005,
  }: {
    reference?: Map<HumanoidBone, Quaternion>;
    ease?: number;
    seam?: number;
    amount?: number;
    /** Per-bone multipliers on `amount` (for example a quieter head). */
    boneAmount?: Partial<Record<HumanoidBone, number>>;
    tolerance?: number;
  } = {},
): MotionClip {
  const identity = new Quaternion();
  const rotations: MotionClip["rotations"] = {};
  for (const [bone, samples] of sampled.bones) {
    const ref = reference?.get(bone);
    const scale = amount * (boneAmount?.[bone] ?? 1);
    const inverse = ref?.clone().invert();
    const keys = samples.map((q, i) => {
      const t = i / sampled.fps;
      const out = inverse ? inverse.clone().multiply(q) : q.clone();
      if (scale !== 1) out.slerp(identity, 1 - scale);
      if (seam > 0 && t > sampled.duration - seam) {
        const first = inverse
          ? inverse.clone().multiply(samples[0])
          : samples[0].clone();
        out.slerp(first, smooth((t - (sampled.duration - seam)) / seam));
      }
      if (ease > 0) {
        const edge = Math.min(t, sampled.duration - t);
        if (edge < ease)
          out.slerp(identity, 1 - smooth(Math.max(0, edge) / ease));
      }
      return out;
    });
    // A delta that never leaves the reference pose adds nothing over the idle: leave the bone out.
    if (inverse && keys.every((q) => q.angleTo(identity) <= tolerance))
      continue;
    rotations[bone] = reduce(keys, sampled.fps, tolerance);
  }
  return { name, rotations };
}

/** Drops keys that the slerp between their kept neighbours reproduces within `tolerance` radians. */
function reduce(keys: Quaternion[], fps: number, tolerance: number): Track {
  const keep = [0];
  for (let i = 1; i < keys.length - 1; i++) {
    const from = keep[keep.length - 1];
    // Keep i when any key between `from` and i + 1 drifts from the straight slerp.
    let fits = true;
    for (let j = from + 1; j <= i; j++) {
      const u = (j - from) / (i + 1 - from);
      const guess = keys[from].clone().slerp(keys[i + 1], u);
      if (guess.angleTo(keys[j]) > tolerance) {
        fits = false;
        break;
      }
    }
    if (!fits) keep.push(i);
  }
  if (keys.length > 1) keep.push(keys.length - 1);
  const times: number[] = [];
  const values: number[] = [];
  let previous: Quaternion | null = null;
  for (const index of keep) {
    const q = keys[index].clone();
    // Stay on one hemisphere so the interpolation never takes the long way round.
    if (previous && previous.dot(q) < 0) q.set(-q.x, -q.y, -q.z, -q.w);
    previous = q;
    times.push(Number((index / fps).toFixed(4)));
    values.push(...q.toArray().map((v) => Number(v.toFixed(5))));
  }
  return { times, values };
}
