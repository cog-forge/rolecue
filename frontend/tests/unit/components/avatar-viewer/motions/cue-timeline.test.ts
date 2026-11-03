import { describe, expect, it } from "vitest";
import {
  CueCursor,
  normalizeCues,
  STALE_GESTURE_MS,
} from "@/components/avatar-viewer/motions/cue-timeline";

describe("normalizeCues", () => {
  it("keeps only the bounded vocabulary, sorted by offset", () => {
    expect(
      normalizeCues([
        { offset_ms: 900, kind: "gesture", name: "nod" },
        { offset_ms: 0, kind: "emotion", name: "friendly" },
        { offset_ms: 10, kind: "emotion", name: "smug" },
        { offset_ms: 20, kind: "gesture", name: "backflip" },
        { offset_ms: 30, kind: "gesture", name: "friendly" },
        { offset_ms: Number.NaN, kind: "gesture", name: "nod" },
        { offset_ms: -5, kind: "gesture", name: "nod" },
        { offset_ms: 400, kind: "gesture", name: "greet" },
      ]),
    ).toEqual([
      { offset_ms: 0, kind: "emotion", name: "friendly" },
      { offset_ms: 400, kind: "gesture", name: "greet" },
      { offset_ms: 900, kind: "gesture", name: "nod" },
    ]);
  });
});

describe("CueCursor", () => {
  const cues = [
    { offset_ms: 0, kind: "emotion", name: "friendly" },
    { offset_ms: 0, kind: "gesture", name: "greet" },
    { offset_ms: 1500, kind: "emotion", name: "curious" },
    { offset_ms: 2600, kind: "gesture", name: "explain_hand" },
    { offset_ms: 5000, kind: "gesture", name: "nod" },
  ] as const;

  it("fires each gesture once as the audio clock crosses it and reports the emotion in force", () => {
    const cursor = new CueCursor(cues);
    expect(cursor.gestureNames()).toEqual(["greet", "explain_hand", "nod"]);
    expect(cursor.advance(0)).toEqual({
      emotion: "friendly",
      gestures: ["greet"],
    });
    expect(cursor.advance(16)).toEqual({ emotion: "friendly", gestures: [] });
    expect(cursor.advance(1499)).toEqual({ emotion: "friendly", gestures: [] });
    expect(cursor.advance(1500)).toEqual({ emotion: "curious", gestures: [] });
    expect(cursor.advance(2650)).toEqual({
      emotion: "curious",
      gestures: ["explain_hand"],
    });
    expect(cursor.advance(2700)).toEqual({ emotion: "curious", gestures: [] });
  });

  it("replays after a backward jump (replay or seek) and skips stale gestures after a forward jump", () => {
    const cursor = new CueCursor(cues);
    cursor.advance(3000);
    // The audio restarted (reconnect re-send, or the Candidate replayed the turn).
    expect(cursor.advance(0)).toEqual({
      emotion: "friendly",
      gestures: ["greet"],
    });
    // The tab was hidden for a while: explain_hand (2600) is stale, the nod at 5000 is not.
    expect(cursor.advance(5000 + STALE_GESTURE_MS - 1)).toEqual({
      emotion: "curious",
      gestures: ["nod"],
    });
    const late = new CueCursor(cues);
    expect(late.advance(9000)).toEqual({ emotion: "curious", gestures: [] });
  });

  it("is harmless without cues or with a bad clock", () => {
    const cursor = new CueCursor([]);
    expect(cursor.size).toBe(0);
    expect(cursor.advance(Number.NaN)).toEqual({ emotion: null, gestures: [] });
    expect(cursor.gestureNames()).toEqual([]);
  });
});
