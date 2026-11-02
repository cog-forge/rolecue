import { MAX_ANSWER_DURATION_MS } from "../transport/protocol";
import { ANSWER_BITS_PER_SECOND, ANSWER_MIME_TYPES } from "./answer-audio";

export interface AnswerRecorderSnapshot {
  status: "idle" | "recording";
  elapsedMs: number;
  error: "unsupported" | "failed" | null;
}
export interface FinishedRecording {
  blob: Blob;
  mimeType: string;
  /** "time-limit": the 3-minute countdown stopped the recorder. */
  reason: "done" | "time-limit";
}
const IDLE: AnswerRecorderSnapshot = {
  status: "idle",
  elapsedMs: 0,
  error: null,
};
const TICK_MS = 250;

export function pickAnswerMimeType(isTypeSupported: (type: string) => boolean) {
  return ANSWER_MIME_TYPES.find((type) => isTypeSupported(type)) ?? "";
}

/** One answer at a time over a borrowed microphone stream. It never stops the stream's tracks.
 * The countdown stops the recorder at 3:00 and hands over what was recorded.
 */
export function createAnswerRecorder({
  now = () => performance.now(),
}: { now?: () => number } = {}) {
  let snapshot = IDLE;
  let recorder: MediaRecorder | null = null;
  let parts: Blob[] = [];
  let startedAt = 0;
  let ticker: ReturnType<typeof setInterval> | null = null;
  let limit: ReturnType<typeof setTimeout> | null = null;
  let reason: FinishedRecording["reason"] = "done";
  let keep = true;
  let stopping = false;
  const listeners = new Set<() => void>();
  const recordedListeners = new Set<(recording: FinishedRecording) => void>();

  function publish(next: AnswerRecorderSnapshot) {
    snapshot = next;
    listeners.forEach((listener) => listener());
  }
  function clearTimers() {
    if (ticker) clearInterval(ticker);
    if (limit) clearTimeout(limit);
    ticker = limit = null;
  }
  function stopWith(nextReason: FinishedRecording["reason"], save: boolean) {
    if (!recorder) return;
    reason = nextReason;
    keep = save;
    stopping = true;
    clearTimers();
    if (recorder.state !== "inactive") recorder.stop();
    else finalize(recorder);
  }
  function finalize(current: MediaRecorder) {
    if (recorder !== current) return;
    recorder = null;
    current.ondataavailable = current.onstop = current.onerror = null;
    const blob = new Blob(parts, {
      type: current.mimeType || parts[0]?.type || "audio/webm",
    });
    parts = [];
    publish(IDLE);
    if (keep && blob.size > 0)
      recordedListeners.forEach((listener) =>
        listener({ blob, mimeType: blob.type, reason }),
      );
  }
  return {
    getSnapshot: () => snapshot,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    /** Called once per kept recording: "Done answering" or the time limit. */
    subscribeRecorded(listener: (recording: FinishedRecording) => void) {
      recordedListeners.add(listener);
      return () => {
        recordedListeners.delete(listener);
      };
    },
    start(stream: MediaStream) {
      if (recorder) return true;
      if (typeof MediaRecorder === "undefined") {
        publish({ ...IDLE, error: "unsupported" });
        return false;
      }
      const mimeType = pickAnswerMimeType((type) =>
        MediaRecorder.isTypeSupported(type),
      );
      let current: MediaRecorder;
      try {
        current = new MediaRecorder(stream, {
          ...(mimeType ? { mimeType } : {}),
          audioBitsPerSecond: ANSWER_BITS_PER_SECOND,
        });
        parts = [];
        reason = "done";
        keep = true;
        stopping = false;
        current.ondataavailable = (event) => {
          if (event.data.size > 0) parts.push(event.data);
        };
        // A stop nobody asked for (the microphone was stopped or lost) is never sent as an answer.
        const interrupted = () => {
          keep = false;
          clearTimers();
          finalize(current);
          publish({ ...IDLE, error: "failed" });
        };
        current.onstop = () => {
          if (stopping) finalize(current);
          else interrupted();
        };
        current.onerror = interrupted;
        current.start();
      } catch {
        publish({ ...IDLE, error: "failed" });
        return false;
      }
      recorder = current;
      startedAt = now();
      publish({ status: "recording", elapsedMs: 0, error: null });
      ticker = setInterval(() => {
        publish({
          status: "recording",
          elapsedMs: Math.min(MAX_ANSWER_DURATION_MS, now() - startedAt),
          error: null,
        });
      }, TICK_MS);
      limit = setTimeout(
        () => stopWith("time-limit", true),
        MAX_ANSWER_DURATION_MS,
      );
      return true;
    },
    /** "Done answering": stop and hand the recording over. */
    finish() {
      stopWith("done", true);
    },
    /** Discard the recording (typed answer chosen, turn moved on, unmount). */
    cancel() {
      stopWith("done", false);
    },
  };
}
export type AnswerRecorder = ReturnType<typeof createAnswerRecorder>;
