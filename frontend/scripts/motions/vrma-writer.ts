/**
 * Minimal VRM Animation (.vrma) writer: a glTF 2.0 binary with the `VRMC_vrm_animation` 1.0 extension,
 * as `@pixiv/three-vrm-animation` loads it. One animation per file; rotation tracks per humanoid bone in
 * the VRM normalized (T-pose, identity rest rotation) space, plus an optional hips translation track.
 *
 * The node hierarchy is a plain rest skeleton with identity rotations, so the loader's parent-rotation
 * conversion is the identity and the track values are played as authored.
 */

const fingers = ["Thumb", "Index", "Middle", "Ring", "Little"] as const;
type Finger = (typeof fingers)[number];
type FingerSegment<F extends Finger> = F extends "Thumb"
  ? "Metacarpal" | "Proximal" | "Distal"
  : "Proximal" | "Intermediate" | "Distal";
type FingerBone = {
  [F in Finger]: `${"left" | "right"}${F}${FingerSegment<F>}`;
}[Finger];

export type HumanoidBone =
  | "hips"
  | "spine"
  | "chest"
  | "upperChest"
  | "neck"
  | "head"
  | "leftShoulder"
  | "leftUpperArm"
  | "leftLowerArm"
  | "leftHand"
  | "rightShoulder"
  | "rightUpperArm"
  | "rightLowerArm"
  | "rightHand"
  | FingerBone;

type Joint = { parent: HumanoidBone | null; offset: [number, number, number] };

/** Rest skeleton (metres) for an average 1.7 m humanoid; only the proportions matter for retargeting. */
const restSkeleton = {
  hips: { parent: null, offset: [0, 0.95, 0] },
  spine: { parent: "hips", offset: [0, 0.1, 0] },
  chest: { parent: "spine", offset: [0, 0.12, 0] },
  upperChest: { parent: "chest", offset: [0, 0.12, 0] },
  neck: { parent: "upperChest", offset: [0, 0.14, 0] },
  head: { parent: "neck", offset: [0, 0.08, 0] },
  leftShoulder: { parent: "upperChest", offset: [0.07, 0.1, 0] },
  leftUpperArm: { parent: "leftShoulder", offset: [0.09, 0, 0] },
  leftLowerArm: { parent: "leftUpperArm", offset: [0.27, 0, 0] },
  leftHand: { parent: "leftLowerArm", offset: [0.25, 0, 0] },
  rightShoulder: { parent: "upperChest", offset: [-0.07, 0.1, 0] },
  rightUpperArm: { parent: "rightShoulder", offset: [-0.09, 0, 0] },
  rightLowerArm: { parent: "rightUpperArm", offset: [-0.27, 0, 0] },
  rightHand: { parent: "rightLowerArm", offset: [-0.25, 0, 0] },
  ...fingerJoints("left"),
  ...fingerJoints("right"),
} as Record<HumanoidBone, Joint>;

/** Finger chains along the hand's axis, 3 cm segments, spread across the palm. */
function fingerJoints(side: "left" | "right") {
  const x = side === "left" ? 1 : -1;
  const joints: Record<string, Joint> = {};
  fingers.forEach((finger, index) => {
    const segments =
      finger === "Thumb"
        ? (["Metacarpal", "Proximal", "Distal"] as const)
        : (["Proximal", "Intermediate", "Distal"] as const);
    let parent: string = `${side}Hand`;
    segments.forEach((segment, depth) => {
      const name = `${side}${finger}${segment}`;
      joints[name] = {
        parent: parent as HumanoidBone,
        offset:
          depth === 0
            ? [x * 0.03, 0, finger === "Thumb" ? 0.025 : 0.02 - index * 0.012]
            : [x * 0.03, 0, 0],
      };
      parent = name;
    });
  });
  return joints;
}
const boneOrder = Object.keys(restSkeleton) as HumanoidBone[];

export type Track = { times: number[]; values: number[] };
export type MotionClip = {
  name: string;
  /** Quaternion tracks (x, y, z, w per key) per bone. */
  rotations: Partial<Record<HumanoidBone, Track>>;
  /** Optional hips translation in metres (x, y, z per key). */
  hipsTranslation?: Track;
};

const FLOAT = 5126;
const ARRAY_BUFFER = 34962;

type Accessor = {
  bufferView: number;
  componentType: number;
  count: number;
  type: "SCALAR" | "VEC3" | "VEC4";
  min?: number[];
  max?: number[];
};

class BinaryBuilder {
  private readonly chunks: Uint8Array[] = [];
  readonly views: { buffer: 0; byteOffset: number; byteLength: number }[] = [];
  readonly accessors: Accessor[] = [];
  private length = 0;
  push(values: number[], type: Accessor["type"]): number {
    const data = new Float32Array(values);
    const bytes = new Uint8Array(data.buffer);
    const view = this.views.length;
    this.views.push({
      buffer: 0,
      byteOffset: this.length,
      byteLength: bytes.byteLength,
    });
    this.chunks.push(bytes);
    this.length += bytes.byteLength;
    const pad = (4 - (this.length % 4)) % 4;
    if (pad) {
      this.chunks.push(new Uint8Array(pad));
      this.length += pad;
    }
    const size = type === "SCALAR" ? 1 : type === "VEC3" ? 3 : 4;
    const accessor: Accessor = {
      bufferView: view,
      componentType: FLOAT,
      count: values.length / size,
      type,
    };
    if (type === "SCALAR") {
      accessor.min = [values[0]];
      accessor.max = [values[values.length - 1]];
    }
    this.accessors.push(accessor);
    return this.accessors.length - 1;
  }
  bytes(): Uint8Array {
    const out = new Uint8Array(this.length);
    let offset = 0;
    for (const chunk of this.chunks) {
      out.set(chunk, offset);
      offset += chunk.byteLength;
    }
    return out;
  }
}

function validateTrack(track: Track, size: number, label: string) {
  if (track.times.length * size !== track.values.length)
    throw new Error(
      `${label}: ${track.times.length} keys need ${size} values each`,
    );
  for (let i = 1; i < track.times.length; i++)
    if (!(track.times[i] > track.times[i - 1]))
      throw new Error(`${label}: times must increase`);
}

/** Encodes one clip as a .vrma (GLB) file. */
export function encodeVrma(clip: MotionClip): Uint8Array {
  const binary = new BinaryBuilder();
  const nodes = boneOrder.map((bone) => ({
    name: bone,
    translation: restSkeleton[bone].offset,
    children: boneOrder
      .map((child, index) => ({ child, index }))
      .filter(({ child }) => restSkeleton[child].parent === bone)
      .map(({ index }) => index),
  }));
  const humanBones = Object.fromEntries(
    boneOrder.map((bone, index) => [bone, { node: index }]),
  );
  const channels: {
    sampler: number;
    target: { node: number; path: "rotation" | "translation" };
  }[] = [];
  const samplers: {
    input: number;
    output: number;
    interpolation: "LINEAR";
  }[] = [];
  const addTrack = (
    bone: HumanoidBone,
    path: "rotation" | "translation",
    track: Track,
  ) => {
    const size = path === "rotation" ? 4 : 3;
    validateTrack(track, size, `${clip.name}.${bone}.${path}`);
    const input = binary.push(track.times, "SCALAR");
    const output = binary.push(
      track.values,
      path === "rotation" ? "VEC4" : "VEC3",
    );
    samplers.push({ input, output, interpolation: "LINEAR" });
    channels.push({
      sampler: samplers.length - 1,
      target: { node: boneOrder.indexOf(bone), path },
    });
  };
  for (const bone of boneOrder) {
    const track = clip.rotations[bone];
    if (track) addTrack(bone, "rotation", track);
  }
  if (clip.hipsTranslation)
    addTrack("hips", "translation", clip.hipsTranslation);
  if (channels.length === 0)
    throw new Error(`${clip.name}: a clip needs at least one track`);

  const bin = binary.bytes();
  const json = {
    asset: { version: "2.0", generator: "rolecue-motions" },
    extensionsUsed: ["VRMC_vrm_animation"],
    extensions: {
      VRMC_vrm_animation: { specVersion: "1.0", humanoid: { humanBones } },
    },
    scene: 0,
    scenes: [{ nodes: [0] }],
    nodes: nodes.map((node) =>
      node.children.length
        ? node
        : { name: node.name, translation: node.translation },
    ),
    animations: [{ name: clip.name, channels, samplers }],
    accessors: binary.accessors,
    bufferViews: binary.views.map((view) => ({ ...view, target: undefined })),
    buffers: [{ byteLength: bin.byteLength }],
  };
  // Keep the JSON free of the undefined `target` keys and 4-byte aligned, as the GLB container requires.
  const jsonBytes = new TextEncoder().encode(JSON.stringify(json));
  const jsonPadded = pad(jsonBytes, 0x20);
  const binPadded = pad(bin, 0x00);
  const total = 12 + 8 + jsonPadded.byteLength + 8 + binPadded.byteLength;
  const out = new Uint8Array(total);
  const view = new DataView(out.buffer);
  view.setUint32(0, 0x46546c67, true); // glTF
  view.setUint32(4, 2, true);
  view.setUint32(8, total, true);
  view.setUint32(12, jsonPadded.byteLength, true);
  view.setUint32(16, 0x4e4f534a, true); // JSON
  out.set(jsonPadded, 20);
  const binStart = 20 + jsonPadded.byteLength;
  view.setUint32(binStart, binPadded.byteLength, true);
  view.setUint32(binStart + 4, 0x004e4942, true); // BIN
  out.set(binPadded, binStart + 8);
  return out;
}

function pad(bytes: Uint8Array, fill: number) {
  const extra = (4 - (bytes.byteLength % 4)) % 4;
  if (!extra) return bytes;
  const out = new Uint8Array(bytes.byteLength + extra);
  out.set(bytes);
  out.fill(fill, bytes.byteLength);
  return out;
}

export {
  ARRAY_BUFFER as GLTF_ARRAY_BUFFER_TARGET,
  boneOrder as humanoidBones,
  restSkeleton,
};
