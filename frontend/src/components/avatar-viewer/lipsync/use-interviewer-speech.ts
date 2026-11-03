"use client";

import { useEffect, useMemo, useRef } from "react";
import type {
  MouthSample,
  MouthSource,
  SpeechPerformance,
} from "../avatar-director";
import type { PerformanceCue } from "../motions/vocabulary";
import { createInterviewerMouth } from "./interviewer-mouth";
import type { InterviewerTimelines } from "./timelines";

export type InterviewerTurnPerformance = InterviewerTimelines & {
  text: string;
  /** Performance cues of the turn (KAN-250); absent on turns sent before them. */
  cues?: readonly PerformanceCue[];
};

/**
 * Mouth and speech motion for the turn playing in `audio`. One mouth per turn: a new turn (including the
 * same turn re-sent after a reconnect), a new element or unmount disposes the previous one. The returned
 * `mouth` is stable, so it can sit in a memoized AvatarPerformance.
 */
export function useInterviewerSpeech(
  audio: HTMLMediaElement | null,
  turn: InterviewerTurnPerformance | null,
): { mouth: MouthSource; speech: SpeechPerformance | undefined } {
  const current = useRef<(MouthSource & { dispose(): void }) | null>(null);
  useEffect(() => {
    if (!audio || !turn) return;
    const mouth = createInterviewerMouth(audio, turn);
    current.current = mouth;
    return () => {
      if (current.current === mouth) current.current = null;
      mouth.dispose();
    };
  }, [audio, turn]);
  const mouth = useMemo<MouthSource>(
    () => ({
      sample: (): MouthSample | null => current.current?.sample() ?? null,
    }),
    [],
  );
  const speech = useMemo(
    () =>
      audio && turn
        ? { audio, words: turn.words, text: turn.text, cues: turn.cues ?? [] }
        : undefined,
    [audio, turn],
  );
  return { mouth, speech };
}
