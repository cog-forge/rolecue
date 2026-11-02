"use client";

import {
  createContext,
  createElement,
  useContext,
  useEffect,
  useMemo,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import type { InterviewerTurn } from "../transport/protocol";

export type PlaybackStatus =
  | "idle"
  | "loading"
  | "playing"
  /** play() was refused (autoplay policy); a user tap must call resume(). */
  | "blocked"
  | "ended";
export interface PlaybackSnapshot {
  audio: HTMLAudioElement | null;
  turn: InterviewerTurn | null;
  playing: boolean;
  status: PlaybackStatus;
}
const IDLE: PlaybackSnapshot = {
  audio: null,
  turn: null,
  playing: false,
  status: "idle",
};

function decodeAudio(turn: InterviewerTurn) {
  if (turn.audio_status !== "ready" || !turn.audio) return null;
  try {
    const binary = atob(turn.audio.data);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index++)
      bytes[index] = binary.charCodeAt(index);
    return new Blob([bytes], { type: turn.audio.mime_type });
  } catch {
    return null;
  }
}

/** One owned audio element per room. A new turn, stop() or dispose() ends the previous one, so
 * interviewer audio never overlaps. Object URLs are revoked as soon as their turn is replaced.
 */
export function createInterviewerPlayback(
  createAudio: () => HTMLAudioElement = () => new Audio(),
) {
  let snapshot = IDLE;
  let audio: HTMLAudioElement | null = null;
  let url: string | null = null;
  let done: (() => void) | null = null;
  let generation = 0;
  const listeners = new Set<() => void>();

  function publish(patch: Partial<PlaybackSnapshot>) {
    snapshot = { ...snapshot, ...patch };
    listeners.forEach((listener) => listener());
  }
  function finish(current: number) {
    if (current !== generation) return;
    const callback = done;
    done = null;
    publish({ playing: false, status: "ended" });
    callback?.();
  }
  function release() {
    generation++;
    done = null;
    if (audio) {
      audio.onplaying = audio.onended = audio.onerror = audio.onpause = null;
      audio.pause();
      audio.removeAttribute("src");
      audio.load();
    }
    if (url) URL.revokeObjectURL(url);
    url = null;
  }
  function start(current: number) {
    if (!audio || current !== generation) return;
    audio.play().catch((error: unknown) => {
      if (current !== generation) return;
      const name =
        typeof error === "object" && error !== null && "name" in error
          ? error.name
          : "";
      if (name === "NotAllowedError")
        publish({ playing: false, status: "blocked" });
      else if (name !== "AbortError") finish(current);
    });
  }
  return {
    getSnapshot: () => snapshot,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    /** Plays one turn and calls onDone once when it ends, or right away for a text-only turn. */
    play(turn: InterviewerTurn, onDone: () => void) {
      release();
      const current = generation;
      const blob = decodeAudio(turn);
      if (!blob) {
        publish({ turn, playing: false, status: "ended" });
        onDone();
        return;
      }
      audio ??= createAudio();
      const element = audio;
      done = onDone;
      url = URL.createObjectURL(blob);
      element.onplaying = () => {
        if (current === generation)
          publish({ playing: true, status: "playing" });
      };
      element.onpause = () => {
        if (current === generation && !element.ended)
          publish({ playing: false });
      };
      element.onended = () => finish(current);
      // A turn whose audio cannot be decoded still continues: the caption carries the question.
      element.onerror = () => finish(current);
      element.src = url;
      publish({ audio: element, turn, playing: false, status: "loading" });
      start(current);
    },
    /** Retries play() from a user gesture after the autoplay policy refused it. */
    resume() {
      if (snapshot.status === "blocked") {
        publish({ status: "loading" });
        start(generation);
      }
    },
    /** Interruption or reconnect: silence now, keep the turn as the caption, never call onDone. */
    stop() {
      release();
      publish({
        playing: false,
        status: snapshot.status === "idle" ? "idle" : "ended",
      });
    },
    /** Unmount: releases the element and its URL. The store can be reused if the room remounts. */
    dispose() {
      release();
      audio = null;
      publish(IDLE);
    },
  };
}
export type InterviewerPlayback = ReturnType<typeof createInterviewerPlayback>;

const PlaybackContext = createContext<InterviewerPlayback | null>(null);

/** Owns the room's playback for the lifetime of the provider and releases it on unmount. */
export function InterviewerPlaybackProvider({
  children,
}: {
  children: ReactNode;
}) {
  // Creating the store touches no browser API; the element is created on the first playable turn.
  const [playback] = useState(() => createInterviewerPlayback());
  useEffect(() => () => playback.dispose(), [playback]);
  return createElement(PlaybackContext.Provider, { value: playback }, children);
}

const subscribeNothing = () => () => {};
const getIdle = () => IDLE;
function usePlaybackSnapshot() {
  const playback = useContext(PlaybackContext);
  return useSyncExternalStore(
    playback?.subscribe ?? subscribeNothing,
    playback?.getSnapshot ?? getIdle,
    getIdle,
  );
}

/** The room's playback controller, for the session runtime. */
export function useInterviewerPlaybackController() {
  const playback = useContext(PlaybackContext);
  if (!playback)
    throw new Error("useInterviewerPlaybackController needs its provider.");
  return playback;
}

/** Read-only view for the avatar layer (KAN-59 lip-sync): the owned element, the current turn
 * (visemes, blend_shapes, words) and whether audio is audible now.
 */
export function useInterviewerPlayback(): {
  audio: HTMLAudioElement | null;
  turn: InterviewerTurn | null;
  playing: boolean;
} {
  const { audio, turn, playing } = usePlaybackSnapshot();
  return useMemo(() => ({ audio, turn, playing }), [audio, turn, playing]);
}

export function useInterviewerPlaybackStatus() {
  return usePlaybackSnapshot().status;
}
