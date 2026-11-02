import type { BufferedSegment, RecordingBuffer } from "./recording-buffer";
import {
  RECORDING_AUDIO_BITS_PER_SECOND,
  RECORDING_CAPTURE_LIMIT_BYTES,
  RECORDING_TIMESLICE_MS,
  RECORDING_VIDEO_BITS_PER_SECOND,
  RECORDING_VIDEO_CONSTRAINTS,
  pickRecordingMimeType,
} from "./recording-format";

export type RecorderErrorCode =
  | "unsupported"
  | "insecure"
  | "permission-denied"
  | "no-device"
  | "device-unavailable"
  | "device-lost"
  | "failed";
export interface SessionRecorderSnapshot {
  status: "idle" | "requesting" | "recording" | "stopped" | "error";
  /** The recorded camera and microphone, for the self-view. Owned by the recorder. */
  stream: MediaStream | null;
  error: RecorderErrorCode | null;
  /** Bytes saved for this interview, earlier page loads included. */
  bytes: number;
  /** An earlier page load of this interview left a recording that this one continues. */
  resumed: boolean;
  /** Capture stopped near the upload limit; what was recorded is kept. */
  limitReached: boolean;
  /** Recording has started at least once on this page; it stays true after an interruption. */
  started: boolean;
}
export const INITIAL_RECORDER_SNAPSHOT: SessionRecorderSnapshot = {
  status: "idle",
  stream: null,
  error: null,
  bytes: 0,
  resumed: false,
  limitReached: false,
  started: false,
};

type RecorderPort = Pick<
  MediaRecorder,
  | "mimeType"
  | "state"
  | "ondataavailable"
  | "onstop"
  | "onerror"
  | "start"
  | "stop"
>;
export interface SessionRecorderDeps {
  buffer: RecordingBuffer;
  getUserMedia?: (constraints: MediaStreamConstraints) => Promise<MediaStream>;
  createRecorder?: (
    stream: MediaStream,
    options: MediaRecorderOptions,
  ) => RecorderPort;
  isTypeSupported?: (type: string) => boolean;
  secure?: () => boolean;
  now?: () => number;
}

function captureError(error: unknown): RecorderErrorCode {
  const name =
    typeof error === "object" && error !== null && "name" in error
      ? error.name
      : "";
  switch (name) {
    case "NotAllowedError":
    case "SecurityError":
      return "permission-denied";
    case "NotFoundError":
    case "OverconstrainedError":
      return "no-device";
    case "NotReadableError":
    case "AbortError":
      return "device-unavailable";
    default:
      return "failed";
  }
}

/**
 * One continuous camera and microphone recording of a recruitment interview. It opens its own
 * capture, so the answer recorder keeps its own microphone path and nothing is recorded twice by one
 * MediaRecorder. Every slice is saved to the buffer as it arrives; a reload starts the next segment
 * of the same recording.
 */
export function createSessionRecorder({
  buffer,
  getUserMedia = (constraints) =>
    navigator.mediaDevices.getUserMedia(constraints),
  createRecorder = (stream, options) => new MediaRecorder(stream, options),
  isTypeSupported = (type) =>
    typeof MediaRecorder !== "undefined" && MediaRecorder.isTypeSupported(type),
  secure = () =>
    window.isSecureContext && Boolean(navigator.mediaDevices?.getUserMedia),
  now = Date.now,
}: SessionRecorderDeps) {
  let snapshot = INITIAL_RECORDER_SNAPSHOT;
  const listeners = new Set<() => void>();
  let recorder: RecorderPort | null = null;
  let stream: MediaStream | null = null;
  let detach: (() => void) | null = null;
  let pending: Promise<void> | null = null;
  let writes: Promise<void> = Promise.resolve();
  let lastSegment = 0;
  let audioEnabled = true;
  let generation = 0;
  let disposed = false;

  function publish(patch: Partial<SessionRecorderSnapshot>) {
    if (disposed) return;
    snapshot = { ...snapshot, ...patch };
    listeners.forEach((listener) => listener());
  }
  function release() {
    detach?.();
    detach = null;
    stream?.getTracks().forEach((track) => track.stop());
    stream = null;
  }
  /** Stops the MediaRecorder; resolves once its last slice is saved. */
  function stopRecorder() {
    const current = recorder;
    recorder = null;
    if (!current) return writes;
    // The last slice arrives before "stop"; `writes` is read after it, so it includes that slice.
    return new Promise<void>((resolve) => {
      current.onstop = () => resolve();
      current.onerror = () => resolve();
      if (current.state === "inactive") resolve();
      else current.stop();
    }).then(() => writes);
  }
  function interrupt(error: RecorderErrorCode) {
    if (!recorder) return;
    void stopRecorder();
    release();
    publish({ status: "error", stream: null, error });
  }

  async function begin() {
    const current = ++generation;
    publish({ status: "requesting", error: null });
    if (!secure()) {
      publish({ status: "error", error: "insecure" });
      return;
    }
    const mimeType = pickRecordingMimeType(isTypeSupported);
    if (!mimeType) {
      publish({ status: "error", error: "unsupported" });
      return;
    }
    let earlier: BufferedSegment[] = [];
    try {
      earlier = await buffer.load();
    } catch {
      earlier = [];
    }
    let acquired: MediaStream;
    try {
      acquired = await getUserMedia({
        video: RECORDING_VIDEO_CONSTRAINTS,
        audio: { echoCancellation: true, noiseSuppression: true },
      });
    } catch (error) {
      if (current === generation && !disposed)
        publish({ status: "error", error: captureError(error) });
      return;
    }
    if (current !== generation || disposed) {
      acquired.getTracks().forEach((track) => track.stop());
      return;
    }
    const video = acquired.getVideoTracks()[0];
    const audio = acquired.getAudioTracks()[0];
    if (video?.readyState !== "live" || audio?.readyState !== "live") {
      acquired.getTracks().forEach((track) => track.stop());
      publish({ status: "error", error: "no-device" });
      return;
    }
    let media: RecorderPort;
    try {
      media = createRecorder(acquired, {
        mimeType,
        videoBitsPerSecond: RECORDING_VIDEO_BITS_PER_SECOND,
        audioBitsPerSecond: RECORDING_AUDIO_BITS_PER_SECOND,
      });
    } catch {
      acquired.getTracks().forEach((track) => track.stop());
      publish({ status: "error", error: "unsupported" });
      return;
    }
    stream = acquired;
    acquired
      .getAudioTracks()
      .forEach((track) => (track.enabled = audioEnabled));
    const lost = () => interrupt("device-lost");
    acquired
      .getTracks()
      .forEach((track) => track.addEventListener("ended", lost));
    detach = () =>
      acquired
        .getTracks()
        .forEach((track) => track.removeEventListener("ended", lost));

    const before = earlier.reduce((total, item) => total + item.bytes, 0);
    const latest = earlier.reduce(
      (max, item) => Math.max(max, item.segment),
      lastSegment,
    );
    // Start time keeps segments in order and apart, even when two tabs open the same interview.
    const segment = Math.max(now(), latest + 1);
    lastSegment = segment;
    let seq = 0;
    let bytes = before;
    media.ondataavailable = (event) => {
      if (event.data.size === 0) return;
      const chunk = event.data;
      const index = seq++;
      bytes += chunk.size;
      writes = writes
        .then(() =>
          buffer.append(segment, index, media.mimeType || mimeType, chunk),
        )
        .catch(() => undefined);
      publish({ bytes });
      if (bytes >= RECORDING_CAPTURE_LIMIT_BYTES && recorder === media) {
        void stopRecorder();
        release();
        publish({ status: "stopped", stream: null, limitReached: true });
      }
    };
    media.onerror = () => {
      if (recorder === media) interrupt("failed");
    };
    media.onstop = () => {
      if (recorder === media) interrupt("device-lost");
    };
    try {
      media.start(RECORDING_TIMESLICE_MS);
    } catch {
      release();
      publish({ status: "error", stream: null, error: "unsupported" });
      return;
    }
    recorder = media;
    publish({
      status: "recording",
      started: true,
      stream: acquired,
      error: null,
      bytes: before,
      resumed: earlier.some((item) => item.bytes > 0),
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
    /** Opens the camera and microphone and starts a segment. Errors are published, never thrown. */
    start(): Promise<void> {
      if (disposed || recorder || snapshot.limitReached)
        return Promise.resolve();
      pending ??= begin().finally(() => {
        pending = null;
      });
      return pending;
    },
    /** Ends the current segment and releases the devices. Resolves once every slice is saved. */
    async stop() {
      generation++;
      await pending;
      const done = stopRecorder();
      release();
      publish({ status: "stopped", stream: null });
      await done;
    },
    /** Follows the room's microphone mute: a muted Candidate is recorded as silence. */
    setAudioEnabled(enabled: boolean) {
      audioEnabled = enabled;
      stream?.getAudioTracks().forEach((track) => (track.enabled = enabled));
    },
    /** Unmount: stop without waiting. Saved slices stay in the buffer for the next page load. */
    dispose() {
      if (disposed) return;
      generation++;
      void stopRecorder();
      release();
      disposed = true;
      listeners.clear();
    },
  };
}
export type SessionRecorder = ReturnType<typeof createSessionRecorder>;
