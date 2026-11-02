/**
 * Structural copy of the `interviewer.turn` timelines (docs/interview-ws-protocol.md §4). Offsets are
 * milliseconds from the start of the turn audio. Kept local so the avatar layer never depends on the
 * transport's types.
 */
export interface InterviewerTimelines {
  visemes: readonly { offset_ms: number; id: number }[];
  /** 60 FPS frames of 55 weights in Azure's ARKit order (`jawOpen` is index 17). */
  blend_shapes: readonly { offset_ms: number; values: readonly number[] }[];
  words: readonly { offset_ms: number; duration_ms: number; text: string }[];
}

export type TimedEntry = { readonly offset_ms: number };

/** Index of the last entry at or before `time`, or -1. `entries` must be sorted by offset. */
export function lastAtOrBefore(entries: readonly TimedEntry[], time: number) {
  let low = 0;
  let high = entries.length - 1;
  let found = -1;
  while (low <= high) {
    const middle = (low + high) >> 1;
    if (entries[middle].offset_ms <= time) {
      found = middle;
      low = middle + 1;
    } else high = middle - 1;
  }
  return found;
}

const finite = (n: unknown): n is number =>
  typeof n === "number" && Number.isFinite(n);
const byOffset = (a: TimedEntry, b: TimedEntry) => a.offset_ms - b.offset_ms;

/** Drops malformed entries and sorts by offset, so sampling can binary-search safely. */
export function normalizeTimelines(
  timelines: InterviewerTimelines,
): InterviewerTimelines {
  return {
    visemes: (timelines.visemes ?? [])
      .filter(
        (v) =>
          finite(v.offset_ms) &&
          Number.isInteger(v.id) &&
          v.id >= 0 &&
          v.id <= 21,
      )
      .toSorted(byOffset),
    blend_shapes: (timelines.blend_shapes ?? [])
      .filter(
        (frame) =>
          finite(frame.offset_ms) &&
          Array.isArray(frame.values) &&
          frame.values.length === 55,
      )
      .toSorted(byOffset),
    words: (timelines.words ?? [])
      .filter(
        (word) =>
          finite(word.offset_ms) &&
          finite(word.duration_ms) &&
          typeof word.text === "string",
      )
      .toSorted(byOffset),
  };
}
