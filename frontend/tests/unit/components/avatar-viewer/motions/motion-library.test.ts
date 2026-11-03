// @vitest-environment node
// GLTFLoader checks `instanceof ArrayBuffer`, which fails across jsdom's realm; the parser is browser code.
import { readFile } from "node:fs/promises";
import { describe, expect, it, vi } from "vitest";
import { VRMAnimation } from "@pixiv/three-vrm-animation";
import {
  createMotionLibrary,
  parseVrma,
} from "@/components/avatar-viewer/motions/motion-library";
import {
  MOTION_CLIPS,
  MOTION_NAMES,
} from "@/components/avatar-viewer/motions/vocabulary";

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

function clip(duration = 1) {
  const animation = new VRMAnimation();
  animation.duration = duration;
  return animation;
}

describe("motion library", () => {
  it("fetches a clip once, within its budget, and shares the parsed animation", async () => {
    const fetchClip = vi.fn(
      async () =>
        new Response(new Uint8Array(100), {
          headers: { "content-length": "100" },
        }),
    );
    const parsed = clip();
    const library = createMotionLibrary({
      fetch: fetchClip,
      parse: async () => parsed,
    });
    expect(library.take("nod")).toBeUndefined();
    library.request("nod");
    library.request("nod");
    expect(library.take("nod")).toBeUndefined();
    await flush();
    expect(library.take("nod")).toBe(parsed);
    expect(fetchClip).toHaveBeenCalledTimes(1);
    expect(fetchClip).toHaveBeenCalledWith(MOTION_CLIPS.nod.file);
  });

  it("marks a clip unavailable on an oversized download, an HTTP error, a parse error or an empty clip", async () => {
    const reasons: string[] = [];
    const cases = {
      nod: async () =>
        new Response(new Uint8Array(MOTION_CLIPS.nod.budgetBytes + 1)),
      greet: async () =>
        new Response(new Uint8Array(10), {
          headers: {
            "content-length": String(MOTION_CLIPS.greet.budgetBytes + 1),
          },
        }),
      lean_in: async () => new Response(null, { status: 404 }),
      idle: async () => new Response(new Uint8Array(10)),
      thinking: async () => new Response(new Uint8Array(10)),
    } as const;
    const library = createMotionLibrary({
      fetch: (url) => {
        const name = (Object.keys(cases) as (keyof typeof cases)[]).find(
          (key) => MOTION_CLIPS[key].file === url,
        )!;
        return cases[name]();
      },
      parse: async (bytes) => {
        if (bytes.byteLength === 10 && reasons.length < 3)
          throw new Error("not a glb");
        return clip(0);
      },
      onUnavailable: (name, reason) => reasons.push(`${name}: ${reason}`),
    });
    for (const name of Object.keys(cases) as (keyof typeof cases)[])
      library.request(name);
    await flush();
    for (const name of Object.keys(cases) as (keyof typeof cases)[])
      expect(library.take(name)).toBeNull();
    expect(reasons).toHaveLength(5);
    expect(reasons.join("\n")).toMatch(/over the .* budget/);
    expect(reasons.join("\n")).toMatch(/HTTP 404/);
    expect(reasons.join("\n")).toMatch(/empty/);
  });

  it("loads every shipped clip through the real parser within its budget", async () => {
    for (const name of MOTION_NAMES) {
      const bytes = await readFile(`public${MOTION_CLIPS[name].file}`);
      expect(bytes.byteLength, name).toBeLessThanOrEqual(
        MOTION_CLIPS[name].budgetBytes,
      );
      const animation = await parseVrma(
        bytes.buffer.slice(
          bytes.byteOffset,
          bytes.byteOffset + bytes.byteLength,
        ),
      );
      expect(animation.duration, name).toBeGreaterThan(0.5);
      expect(animation.humanoidTracks.rotation.size, name).toBeGreaterThan(0);
      // No clip moves the hips or the legs, so none can slide the feet.
      expect(animation.humanoidTracks.translation.size, name).toBe(0);
      expect(
        animation.expressionTracks.preset.size +
          animation.expressionTracks.custom.size,
        name,
      ).toBe(0);
      for (const bone of animation.humanoidTracks.rotation.keys())
        expect(bone, name).not.toMatch(/Leg|Foot|Toes|hips/);
    }
  });
});
