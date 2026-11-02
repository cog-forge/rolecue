// @vitest-environment node
import { describe, expect, it } from "vitest";
import { mergeWebm } from "@/features/interview/recording/webm-merge";
import { readWebm, webmFile } from "./webm-builder";

const blob = (bytes: Uint8Array<ArrayBuffer>) =>
  new Blob([bytes], { type: "video/webm" });
async function read(merged: Blob | undefined) {
  if (!merged) throw new Error("not merged");
  return readWebm(new Uint8Array(await merged.arrayBuffer()));
}
// Two MediaRecorder runs of one interview: audio every 60 ms and video every 40 ms.
const first = webmFile({
  clusters: [
    {
      time: 0,
      blocks: [
        { track: 1, time: 0, tag: 1 },
        { track: 2, time: 2, tag: 2 },
        { track: 2, time: 42, tag: 3 },
      ],
    },
    {
      time: 1000,
      blocks: [
        { track: 1, time: 1000, tag: 4 },
        { track: 1, time: 1060, tag: 5 },
        { track: 2, time: 1010, tag: 6 },
        { track: 2, time: 1050, tag: 7 },
      ],
    },
  ],
});
const second = webmFile({
  clusters: [
    {
      time: 0,
      blocks: [
        { track: 1, time: 0, tag: 8 },
        { track: 2, time: 5, tag: 9 },
      ],
    },
  ],
});

describe("mergeWebm", () => {
  it("puts a reloaded recording's clusters after the first on one timeline", async () => {
    const merged = await mergeWebm([blob(first), blob(second)]);
    const file = await read(merged?.blob);
    expect(file.segments).toBe(1);
    expect(file.topLevel).toEqual([0x1a45dfa3, 0x18538067]);
    // The first run ends one audio packet after its last audio block (1060 + 60); the second run
    // has one block per track, so it ends at its last block (5).
    expect(merged?.duration).toBe(1125);
    expect(file.duration).toBe(1125);
    expect(file.blocks.map((block) => block.tag)).toEqual([
      1, 2, 3, 4, 5, 6, 7, 8, 9,
    ]);
    expect(file.blocks.at(-2)).toEqual({ track: 1, time: 1120, tag: 8 });
    expect(file.blocks.at(-1)).toEqual({ track: 2, time: 1125, tag: 9 });
  });

  it("renumbers the tracks of a later recording that numbered them the other way round", async () => {
    // After the reload the video frame arrived first, so video is track 1 and audio track 2.
    const swapped = webmFile({
      tracks: [
        { number: 1, type: 1, codec: "V_VP8" },
        { number: 2, type: 2, codec: "A_OPUS" },
      ],
      clusters: [
        {
          time: 0,
          blocks: [
            { track: 1, time: 0, tag: 8 },
            { track: 2, time: 5, tag: 9 },
          ],
        },
      ],
    });
    const file = await read(
      (await mergeWebm([blob(first), blob(swapped)]))?.blob,
    );
    expect(file.blocks.slice(-2)).toEqual([
      { track: 2, time: 1120, tag: 8 },
      { track: 1, time: 1125, tag: 9 },
    ]);
    // The first file's own blocks are untouched.
    expect(file.blocks[0]).toEqual({ track: 1, time: 0, tag: 1 });
  });

  it("gives a single recording a duration and keeps every block", async () => {
    const merged = await mergeWebm([blob(first)]);
    const file = await read(merged?.blob);
    expect(file.duration).toBe(1120);
    expect(file.blocks).toHaveLength(7);
  });

  it("reads known-size clusters too and drops the old seek and cue data", async () => {
    const known = webmFile({
      unknownSizes: false,
      clusters: [{ time: 0, blocks: [{ track: 1, time: 0, tag: 1 }] }],
      // A Void element and a SeekHead stub point into the old layout.
      extra: [0xec, 0x82, 0x00, 0x00, 0x11, 0x4d, 0x9b, 0x74, 0x80],
    });
    const file = await read(
      (await mergeWebm([blob(known), blob(second)]))?.blob,
    );
    expect(file.blocks.map((block) => block.tag)).toEqual([1, 8, 9]);
  });

  it("keeps the whole elements of a file cut short by a crash", async () => {
    const cut = first.subarray(0, first.length - 3);
    const file = await read((await mergeWebm([blob(cut)]))?.blob);
    expect(file.blocks.map((block) => block.tag)).toEqual([1, 2, 3, 4, 5, 6]);
  });

  it("refuses files that are not WebM or do not share their tracks", async () => {
    const other = webmFile({
      tracks: [{ number: 1, type: 1, codec: "V_VP9" }],
      clusters: [{ time: 0, blocks: [{ track: 1, time: 0, tag: 1 }] }],
    });
    expect(await mergeWebm([blob(first), blob(other)])).toBeNull();
    expect(
      await mergeWebm([new Blob([new Uint8Array([0, 0, 0, 0x20, 0x66])])]),
    ).toBeNull();
    expect(await mergeWebm([])).toBeNull();
  });
});
