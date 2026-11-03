import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { interviewFraming } from "@/components/avatar-viewer/interview-framing";
import { environmentScenes } from "@/components/avatar-viewer/environment/scenes";

type Vec3 = [number, number, number];
type Gltf = {
  scene?: number;
  scenes: { nodes: number[] }[];
  nodes: {
    mesh?: number;
    children?: number[];
    translation?: Vec3;
    scale?: Vec3;
    rotation?: number[];
  }[];
  meshes: { primitives: { attributes: { POSITION: number } }[] }[];
  accessors: {
    min: Vec3;
    max: Vec3;
    componentType: number;
    normalized?: boolean;
  }[];
};

// Normalized integer components divide by their type's maximum (glTF 2.0 §3.11).
const normalizers: Record<number, number> = {
  5120: 127,
  5121: 255,
  5122: 32767,
  5123: 65535,
};

// World bounds of a GLB's meshes from the accessor min/max in its JSON chunk. Meshopt quantization
// stores positions as integers under a node translation and scale; the props have no rotations.
async function propBounds(path: string) {
  const file = await readFile(join(process.cwd(), "public", path));
  const jsonLength = file.readUInt32LE(12);
  const gltf: Gltf = JSON.parse(file.subarray(20, 20 + jsonLength).toString());
  const min: Vec3 = [Infinity, Infinity, Infinity];
  const max: Vec3 = [-Infinity, -Infinity, -Infinity];
  const visit = (index: number, offset: Vec3, scale: Vec3) => {
    const node = gltf.nodes[index];
    expect(node.rotation ?? [0, 0, 0, 1]).toEqual([0, 0, 0, 1]);
    const t = node.translation ?? [0, 0, 0];
    const s = node.scale ?? [1, 1, 1];
    const nextOffset = offset.map(
      (o, axis) => o + scale[axis] * t[axis],
    ) as Vec3;
    const nextScale = scale.map((value, axis) => value * s[axis]) as Vec3;
    if (node.mesh !== undefined)
      for (const primitive of gltf.meshes[node.mesh].primitives) {
        const accessor = gltf.accessors[primitive.attributes.POSITION];
        const unit = accessor.normalized
          ? normalizers[accessor.componentType]
          : 1;
        for (const axis of [0, 1, 2]) {
          const a =
            nextOffset[axis] + (nextScale[axis] * accessor.min[axis]) / unit;
          const b =
            nextOffset[axis] + (nextScale[axis] * accessor.max[axis]) / unit;
          min[axis] = Math.min(min[axis], a, b);
          max[axis] = Math.max(max[axis], a, b);
        }
      }
    node.children?.forEach((child) => visit(child, nextOffset, nextScale));
  };
  gltf.scenes[gltf.scene ?? 0].nodes.forEach((node) =>
    visit(node, [0, 0, 0], [1, 1, 1]),
  );
  return { min, max };
}

// A typical full-body VRM standing on the origin, as the canvas measures it.
const avatarHeight = 1.72;
const viewports = [
  { name: "desktop", width: 1600, height: 900 },
  { name: "mobile", width: 390, height: 844 },
];

describe("scene furniture and the interviewer framing", () => {
  it.each(Object.entries(environmentScenes))(
    "%s furniture stands between the camera and the interviewer, below eye level, in every camera view",
    async (_, scene) => {
      const { min, max } = await propBounds(scene.props);
      const [, scaleY, scaleZ] = scene.prop.scale;
      const top = max[1] * scaleY;
      const near = scene.prop.position[2] + max[2] * scaleZ;
      const far = scene.prop.position[2] + min[2] * scaleZ;
      // Counter height: tall enough to hide the hips, low enough to keep the hands and chest in view.
      expect(top).toBeGreaterThan(0.95);
      expect(top).toBeLessThan(1.1);
      // The far edge stays in front of the body (about 0.15 m deep), so nothing cuts through it.
      expect(far).toBeGreaterThan(0.15);
      for (const viewport of viewports)
        for (const close of [false, true]) {
          const framing = interviewFraming(
            avatarHeight,
            0,
            viewport.width,
            viewport.height,
            close,
          );
          expect(framing.eyeY).toBeGreaterThan(top);
          expect(near).toBeLessThan(framing.distance);
        }
    },
  );
});
