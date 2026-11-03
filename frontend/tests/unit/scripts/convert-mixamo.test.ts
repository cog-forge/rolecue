import { describe, expect, it } from "vitest";
import { Euler, Quaternion } from "three";
import {
  buildMixamoClip,
  firstFrame,
  type Sampled,
} from "../../../scripts/motions/convert-mixamo";

const rot = (x: number) => new Quaternion().setFromEuler(new Euler(x, 0, 0));
const angleOf = (values: number[], key: number) =>
  new Quaternion().fromArray(values, key * 4).angleTo(new Quaternion());

/** Two seconds at 10 fps: the head nods to 0.3 rad and back; the spine holds a constant 0.2 rad. */
function sampled(): Sampled {
  const head = Array.from({ length: 21 }, (_, i) =>
    rot(0.3 * Math.sin((Math.PI * i) / 20)),
  );
  const spine = Array.from({ length: 21 }, () => rot(0.2));
  return {
    fps: 10,
    duration: 2,
    bones: new Map([
      ["head", head],
      ["spine", spine],
    ]),
  };
}

describe("buildMixamoClip", () => {
  it("keeps absolute rotations for the idle and drops keys a slerp reproduces", () => {
    const clip = buildMixamoClip("idle", sampled());
    const spine = clip.rotations.spine!;
    // A constant track keeps only its two ends.
    expect(spine.times).toEqual([0, 2]);
    expect(angleOf(spine.values, 0)).toBeCloseTo(0.2, 3);
    const head = clip.rotations.head!;
    expect(head.times[0]).toBe(0);
    expect(head.times.at(-1)).toBe(2);
    expect(head.times.length).toBeLessThan(21);
  });

  it("stores deltas from a reference pose and leaves out bones that never leave it", () => {
    const source = sampled();
    const clip = buildMixamoClip("nod", source, {
      reference: firstFrame(source),
    });
    // The spine only holds its first pose: nothing to layer over the idle.
    expect(clip.rotations.spine).toBeUndefined();
    const head = clip.rotations.head!;
    const peak = head.times.findIndex((t) => Math.abs(t - 1) < 1e-6);
    expect(angleOf(head.values, peak)).toBeCloseTo(0.3, 2);
  });

  it("eases a one-shot from and back to the reference and scales it down", () => {
    const source = sampled();
    source.bones.set(
      "head",
      source.bones.get("head")!.map(() => rot(0.4)),
    );
    const clip = buildMixamoClip("lean_in", source, {
      reference: new Map([["head", rot(0)]]),
      ease: 0.5,
      amount: 0.5,
    });
    const head = clip.rotations.head!;
    expect(angleOf(head.values, 0)).toBeCloseTo(0, 4);
    expect(angleOf(head.values, head.times.length - 1)).toBeCloseTo(0, 4);
    // The held part is the delta at half strength; the reduction keeps only its corners.
    const angles = head.times.map((_, key) => angleOf(head.values, key));
    expect(Math.max(...angles)).toBeCloseTo(0.2, 2);
  });

  it("blends a loop's end into its start so the seam does not pop", () => {
    const source = sampled();
    source.bones.set(
      "head",
      Array.from({ length: 21 }, (_, i) => rot(0.02 * i)),
    );
    const head = buildMixamoClip("thinking", source, { seam: 0.5 }).rotations
      .head!;
    expect(angleOf(head.values, 0)).toBeCloseTo(0, 4);
    expect(angleOf(head.values, head.times.length - 1)).toBeCloseTo(0, 3);
  });
});
