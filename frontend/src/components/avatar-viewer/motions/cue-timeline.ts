import {
  isCueEmotion,
  isCueGesture,
  type CueEmotion,
  type CueGesture,
  type PerformanceCue,
} from "./vocabulary";

export type TimedCue =
  | { offset_ms: number; kind: "emotion"; name: CueEmotion }
  | { offset_ms: number; kind: "gesture"; name: CueGesture };

/** Keeps the cues the client can perform, sorted by offset; anything else is dropped silently. */
export function normalizeCues(cues: readonly PerformanceCue[]): TimedCue[] {
  const out: TimedCue[] = [];
  for (const cue of cues) {
    if (!Number.isFinite(cue.offset_ms) || cue.offset_ms < 0) continue;
    if (cue.kind === "emotion" && isCueEmotion(cue.name))
      out.push({ offset_ms: cue.offset_ms, kind: "emotion", name: cue.name });
    else if (cue.kind === "gesture" && isCueGesture(cue.name))
      out.push({ offset_ms: cue.offset_ms, kind: "gesture", name: cue.name });
  }
  return out.toSorted((a, b) => a.offset_ms - b.offset_ms);
}

/** A gesture whose offset is further behind the clock than this is skipped, not played late. */
export const STALE_GESTURE_MS = 1500;

/**
 * Walks a turn's cues with the audio clock. `advance(time)` returns the emotion in force at `time` and
 * the gestures whose offset was crossed since the previous call. A backward jump (replay, seek) rewinds
 * the cursor so the cues play again; a forward jump (a tab that was hidden) skips gestures that are
 * already stale instead of firing them in a burst. Pure: no timers, no clock of its own.
 */
export class CueCursor {
  private readonly cues: TimedCue[];
  private index = 0;
  private last = Number.NEGATIVE_INFINITY;
  constructor(cues: readonly PerformanceCue[]) {
    this.cues = normalizeCues(cues);
  }
  get size() {
    return this.cues.length;
  }
  advance(timeMs: number): {
    emotion: CueEmotion | null;
    gestures: CueGesture[];
  } {
    const time = Number.isFinite(timeMs) ? timeMs : 0;
    if (time < this.last) {
      this.index = 0;
    }
    const gestures: CueGesture[] = [];
    let emotion: CueEmotion | null = null;
    for (
      let i = 0;
      i < this.cues.length && this.cues[i].offset_ms <= time;
      i++
    ) {
      const cue = this.cues[i];
      if (cue.kind === "emotion") emotion = cue.name;
      else if (i >= this.index && time - cue.offset_ms <= STALE_GESTURE_MS)
        gestures.push(cue.name);
      this.index = Math.max(this.index, i + 1);
    }
    this.last = time;
    return { emotion, gestures };
  }
  /** The gestures this turn will need, so their clips can be requested before they are due. */
  gestureNames(): CueGesture[] {
    return [
      ...new Set(
        this.cues.flatMap((cue) => (cue.kind === "gesture" ? [cue.name] : [])),
      ),
    ];
  }
}
