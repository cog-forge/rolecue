import { describe, expect, it } from "vitest";
import fixture from "@tests/fixtures/interviewer-turn-timelines.json";
import {
  azureBlendShapeChannels,
  azureBlendShapeNames,
  azureVisemeToOculus,
  oculusVisemes,
} from "@/components/avatar-viewer/lipsync/azure";
import {
  createVisemeTrack,
  sampleBlendShapes,
} from "@/components/avatar-viewer/lipsync/timeline-drivers";
import {
  lastAtOrBefore,
  normalizeTimelines,
} from "@/components/avatar-viewer/lipsync/timelines";

const frame = (offset_ms: number, set: Record<string, number>) => ({
  offset_ms,
  values: azureBlendShapeNames.map((name) => set[name] ?? 0),
});
const top = (sample: Readonly<Record<string, number>>) =>
  Object.entries(sample).sort((a, b) => b[1] - a[1])[0]?.[0];

describe("timelines", () => {
  it("binary-searches the last entry at or before a time", () => {
    const entries = [0, 17, 33, 50].map((offset_ms) => ({ offset_ms }));
    expect(lastAtOrBefore(entries, -1)).toBe(-1);
    expect(lastAtOrBefore(entries, 0)).toBe(0);
    expect(lastAtOrBefore(entries, 32.9)).toBe(1);
    expect(lastAtOrBefore(entries, 33)).toBe(2);
    expect(lastAtOrBefore(entries, 9999)).toBe(3);
    expect(lastAtOrBefore([], 10)).toBe(-1);
  });
  it("sorts and drops malformed entries instead of trusting the wire", () => {
    const normalized = normalizeTimelines({
      visemes: [
        { offset_ms: 50, id: 21 },
        { offset_ms: 10, id: 1 },
        { offset_ms: Number.NaN, id: 2 },
        { offset_ms: 60, id: 99 },
      ],
      blend_shapes: [frame(17, {}), { offset_ms: 0, values: [1, 2] }],
      words: [
        { offset_ms: 5, duration_ms: Number.POSITIVE_INFINITY, text: "x" },
      ],
    });
    expect(normalized.visemes.map((v) => v.offset_ms)).toEqual([10, 50]);
    expect(normalized.blend_shapes).toHaveLength(1);
    expect(normalized.words).toEqual([]);
  });
});

describe("Azure mapping", () => {
  it("follows Azure's 55-channel order and never applies eye or roll channels", () => {
    expect(azureBlendShapeNames).toHaveLength(55);
    expect(azureBlendShapeNames[17]).toBe("jawOpen");
    expect(azureBlendShapeNames.slice(52)).toEqual([
      "headRoll",
      "leftEyeRoll",
      "rightEyeRoll",
    ]);
    const applied = new Map(
      azureBlendShapeChannels.map((channel) => [channel.name, channel.gain]),
    );
    expect([...applied.keys()].some((name) => name.startsWith("eye"))).toBe(
      false,
    );
    expect(applied.has("headRoll")).toBe(false);
    expect(applied.get("jawOpen")).toBe(1);
    expect(applied.get("tongueOut")).toBe(1);
    expect(applied.get("browInnerUp")).toBe(0.3);
    expect(applied.get("cheekSquintLeft")).toBe(0.3);
    expect(applied.size).toBe(38);
  });
  it("maps every Azure viseme ID to an Oculus viseme and uses all 15", () => {
    expect(azureVisemeToOculus).toHaveLength(22);
    expect(new Set(azureVisemeToOculus)).toEqual(new Set(oculusVisemes));
    expect(azureVisemeToOculus[0]).toBe("sil");
    expect(azureVisemeToOculus[21]).toBe("PP");
    expect(azureVisemeToOculus[18]).toBe("FF");
  });
});

describe("sampleBlendShapes", () => {
  it("interpolates between 60 FPS frames and scales the upper face", () => {
    const frames = [
      frame(0, { jawOpen: 0.2, browInnerUp: 1, eyeBlinkLeft: 1 }),
      frame(16, { jawOpen: 0.6, browInnerUp: 0 }),
    ];
    const sample = sampleBlendShapes(frames, 8);
    expect(sample.jawOpen).toBeCloseTo(0.4);
    expect(sample.browInnerUp).toBeCloseTo(0.15);
    expect(sample.eyeBlinkLeft).toBeUndefined();
    expect(sampleBlendShapes(frames, 16).jawOpen).toBeCloseTo(0.6);
  });
  it("rests outside the timeline", () => {
    const frames = [frame(100, { jawOpen: 0.5 }), frame(117, { jawOpen: 0.5 })];
    expect(sampleBlendShapes(frames, 99)).toEqual({});
    expect(sampleBlendShapes(frames, 130).jawOpen).toBeCloseTo(0.5);
    expect(sampleBlendShapes(frames, 1000)).toEqual({});
    expect(sampleBlendShapes([], 0)).toEqual({});
  });
  it("opens on a vowel and closes the lips on /m/ in the captured turn", () => {
    const timelines = normalizeTimelines(fixture);
    const vowel = sampleBlendShapes(timelines.blend_shapes, 1700);
    expect(vowel.jawOpen).toBeGreaterThan(0.3);
    expect(vowel.mouthLowerDownLeft).toBeGreaterThan(0.5);
    expect(vowel.mouthClose ?? 0).toBeLessThan(0.1);
    // ARKit closes the lips with mouthClose ≈ jawOpen.
    const bilabial = sampleBlendShapes(timelines.blend_shapes, 3400);
    expect(bilabial.mouthClose).toBeGreaterThan(0.4);
    expect(bilabial.mouthClose! / bilabial.jawOpen!).toBeGreaterThan(0.85);
  });
});

describe("createVisemeTrack", () => {
  it("peaks each viseme just after its offset and blends into the next", () => {
    const track = createVisemeTrack([
      { offset_ms: 0, id: 0 },
      { offset_ms: 100, id: 21 },
      { offset_ms: 200, id: 2 },
      { offset_ms: 400, id: 0 },
    ]);
    expect(track(0)).toEqual({});
    expect(track(125).viseme_PP).toBeCloseTo(1);
    const overlap = track(195);
    expect(overlap.viseme_PP).toBeGreaterThan(0);
    expect(overlap.viseme_aa).toBeGreaterThan(0);
    expect(track(225).viseme_aa).toBeCloseTo(0.75);
    expect(track(400 + 120)).toEqual({});
  });
  it("never sums the same viseme above its peak", () => {
    const track = createVisemeTrack([
      { offset_ms: 0, id: 21 },
      { offset_ms: 50, id: 21 },
      { offset_ms: 100, id: 0 },
    ]);
    for (let t = 0; t <= 200; t += 5)
      expect(track(t).viseme_PP ?? 0).toBeLessThanOrEqual(1);
  });
  it("tracks the captured turn's vowels and bilabials", () => {
    const track = createVisemeTrack(normalizeTimelines(fixture).visemes);
    expect(top(track(3425))).toBe("viseme_PP");
    expect(track(3425).viseme_PP).toBeCloseTo(1);
    expect(top(track(2247))).toBe("viseme_E");
    expect(track(20)).toEqual({});
    expect(track(5600)).toEqual({});
  });
});
