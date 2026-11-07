import { readApiError } from "@/lib/i18n/api-errors";
import {
  completeRecordingUpload,
  createRecordingUpload,
  getRecordingStatus,
  putRecording,
  type RecordStatus,
  type RecordingUploadGrant,
} from "./recording-api";
import type { BufferedSegment, RecordingBuffer } from "./recording-buffer";
import { RECORDING_MAX_BYTES, uploadContentType } from "./recording-format";
import { mergeWebm } from "./webm-merge";

export interface AssembledRecording {
  blob: Blob;
  contentType: "video/webm" | "video/mp4";
  /** false: the segments could not be joined, so only the longest one is uploaded. */
  complete: boolean;
}

/**
 * The one file of a recording from its buffered segments. WebM segments are joined into one timeline;
 * a single segment is uploaded as recorded. Segments that cannot be joined (MP4 from Safari after a
 * reload) leave only the longest one, flagged as partial.
 */
export async function assembleRecording(
  segments: readonly BufferedSegment[],
): Promise<AssembledRecording | null> {
  const recorded = segments.filter((segment) => segment.bytes > 0);
  if (recorded.length === 0) return null;
  const files = recorded.map((segment) => ({
    blob: new Blob(segment.chunks, {
      type: uploadContentType(segment.mimeType),
    }),
    contentType: uploadContentType(segment.mimeType),
  }));
  if (files.every((file) => file.contentType === "video/webm")) {
    const merged = await mergeWebm(files.map((file) => file.blob));
    if (merged)
      return { blob: merged.blob, contentType: "video/webm", complete: true };
  }
  if (files.length === 1) return { ...files[0], complete: true };
  const longest = files.reduce((best, file) =>
    file.blob.size > best.blob.size ? file : best,
  );
  return { ...longest, complete: false };
}

export interface UploadSnapshot {
  status:
    | "idle"
    | "checking"
    | "preparing"
    | "uploading"
    | "retrying"
    | "processing"
    | "ready"
    | "failed"
    | "missing";
  /** 0 to 1 while uploading. */
  progress: number;
  /** 1-based attempt of this run. */
  attempt: number;
  /** failed: the Candidate gave the upload up and the copy in this browser is deleted. */
  abandoned: boolean;
  /** The uploaded file holds only part of the session (see assembleRecording). */
  partial: boolean;
  /** failed: why. "rejected" and "exhausted" cannot be fixed by retrying (too large, refused, or every upload grant of this interview used). */
  reason: "network" | "rejected" | "exhausted" | null;
}
export const INITIAL_UPLOAD_SNAPSHOT: UploadSnapshot = {
  status: "idle",
  progress: 0,
  attempt: 0,
  abandoned: false,
  partial: false,
  reason: null,
};
export const UPLOAD_ATTEMPTS = 3;
export const UPLOAD_RETRY_DELAYS_MS = [2_000, 5_000] as const;

export interface UploadApi {
  status(interviewId: string): Promise<RecordStatus>;
  grant(
    interviewId: string,
    upload: { contentType: string; size: number; consentVersion?: string },
  ): Promise<RecordingUploadGrant>;
  put(
    grant: RecordingUploadGrant,
    blob: Blob,
    onProgress: (fraction: number) => void,
    signal: AbortSignal,
  ): Promise<void>;
  complete(interviewId: string): Promise<RecordStatus>;
}
const defaultApi: UploadApi = {
  status: (interviewId) => getRecordingStatus(interviewId),
  grant: createRecordingUpload,
  put: putRecording,
  complete: completeRecordingUpload,
};

function wait(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    const timer = setTimeout(resolve, ms);
    signal.addEventListener(
      "abort",
      () => {
        clearTimeout(timer);
        resolve();
      },
      { once: true },
    );
  });
}

type Attempt = "ready" | "retry" | "rejected" | "exhausted";

/**
 * Uploads the buffered recording of one interview and finalizes it: grant, PUT to storage, complete.
 * Each attempt asks completion even after a failed PUT, so the server records what really arrived
 * (ready, or failed when nothing did). Interrupted attempts are retried; when they keep failing the
 * Candidate can retry later or give up, which leaves the recording failed. It never reports ready
 * unless the server said ready, and it only ever calls the API for its own interview.
 */
export function createRecordingUploader({
  interviewId,
  buffer,
  api = defaultApi,
  delay = wait,
  consentVersion,
}: {
  interviewId: string;
  buffer: RecordingBuffer;
  /** The consent version the Candidate accepted, sent with every upload grant (KAN-297). */
  consentVersion?: () => string | undefined;
  api?: UploadApi;
  delay?: (ms: number, signal: AbortSignal) => Promise<void>;
}) {
  let snapshot = INITIAL_UPLOAD_SNAPSHOT;
  const listeners = new Set<() => void>();
  let running: Promise<void> | null = null;
  let controller = new AbortController();
  // A grant was taken and no completion answered yet: the server may still say uploading.
  let unsettled = false;
  let lastSize = 0;
  let lastType: AssembledRecording["contentType"] = "video/webm";
  let disposed = false;

  function publish(patch: Partial<UploadSnapshot>) {
    if (disposed) return;
    snapshot = { ...snapshot, ...patch };
    listeners.forEach((listener) => listener());
  }

  async function attempt(
    recording: AssembledRecording,
    signal: AbortSignal,
  ): Promise<Attempt> {
    let grant: RecordingUploadGrant;
    try {
      grant = await api.grant(interviewId, {
        contentType: recording.contentType,
        size: recording.blob.size,
        consentVersion: consentVersion?.(),
      });
    } catch (error) {
      const { code, status } = readApiError(error);
      // The server issues at most a few upload grants per interview and never another: retrying
      // cannot succeed.
      if (code === "RECORDING_UPLOAD_LIMIT") return "exhausted";
      if (code === "RECORDING_NOT_ALLOWED") {
        // Already ready (an earlier page finished it) or no longer accepted.
        const current = await api.status(interviewId).catch(() => null);
        return current === "ready" ? "ready" : "rejected";
      }
      if (
        code === "VALIDATION_ERROR" ||
        code === "RECORDING_CONSENT_REQUIRED" ||
        code === "INTERVIEW_NOT_FOUND" ||
        status === 401 ||
        status === 403
      )
        return "rejected";
      return "retry";
    }
    unsettled = true;
    try {
      await api.put(
        grant,
        recording.blob,
        (progress) => publish({ progress }),
        signal,
      );
    } catch {
      // Completion below tells whether the object arrived anyway.
    }
    if (signal.aborted) return "retry";
    try {
      const status = await api.complete(interviewId);
      unsettled = false;
      return status === "ready" ? "ready" : "retry";
    } catch (error) {
      const { code, network } = readApiError(error);
      if (!network) unsettled = false;
      return code === "VALIDATION_ERROR" ? "rejected" : "retry";
    }
  }

  async function upload(signal: AbortSignal) {
    publish({ ...INITIAL_UPLOAD_SNAPSHOT, status: "checking" });
    const server = await api.status(interviewId).catch(() => null);
    if (signal.aborted) return;
    if (server === "ready") {
      await buffer.clear().catch(() => undefined);
      publish({ status: "ready", progress: 1 });
      return;
    }
    if (server === "processing") {
      publish({ status: "processing" });
      return;
    }
    publish({ status: "preparing" });
    const recording = await assembleRecording(
      await buffer.load().catch(() => []),
    );
    if (signal.aborted) return;
    if (!recording) {
      publish({ status: "missing" });
      return;
    }
    lastSize = recording.blob.size;
    lastType = recording.contentType;
    publish({ partial: !recording.complete });
    if (recording.blob.size > RECORDING_MAX_BYTES) {
      publish({ status: "failed", reason: "rejected" });
      return;
    }
    for (let index = 1; index <= UPLOAD_ATTEMPTS; index++) {
      publish({ status: "uploading", attempt: index, progress: 0 });
      const result = await attempt(recording, signal);
      if (signal.aborted) return;
      if (result === "ready") {
        await buffer.clear().catch(() => undefined);
        publish({ status: "ready", progress: 1 });
        return;
      }
      if (result === "rejected") {
        publish({ status: "failed", reason: "rejected" });
        return;
      }
      if (result === "exhausted") {
        publish({ status: "failed", reason: "exhausted" });
        return;
      }
      if (index < UPLOAD_ATTEMPTS) {
        publish({ status: "retrying" });
        await delay(UPLOAD_RETRY_DELAYS_MS[index - 1], signal);
        if (signal.aborted) return;
      }
    }
    publish({ status: "failed", reason: "network" });
  }

  return {
    getSnapshot: () => snapshot,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    /** Uploads what the buffer holds. A second call while one runs joins it. */
    run(): Promise<void> {
      if (disposed) return Promise.resolve();
      if (snapshot.status === "ready" || snapshot.abandoned)
        return Promise.resolve();
      const signal = controller.signal;
      running ??= upload(signal).finally(() => {
        running = null;
      });
      return running;
    },
    /**
     * The Candidate stops trying: the recording ends failed on the server (completion without an
     * object) and the copy in this browser is deleted.
     */
    async giveUp() {
      controller.abort();
      controller = new AbortController();
      await running;
      let status = await api.status(interviewId).catch(() => null);
      if (status === null) {
        // Offline: nothing can be recorded on the server, so the copy here is kept.
        publish({ status: "failed", reason: "network" });
        return;
      }
      try {
        if (status === "none" && lastSize > 0) {
          await api.grant(interviewId, {
            contentType: lastType,
            size: lastSize,
            consentVersion: consentVersion?.(),
          });
          status = "uploading";
        }
        if (status === "uploading") status = await api.complete(interviewId);
      } catch (error) {
        if (readApiError(error).code === "RECORDING_NOT_FOUND")
          status = "failed";
      }
      unsettled = false;
      await buffer.clear().catch(() => undefined);
      if (status === "ready") publish({ status: "ready", progress: 1 });
      else publish({ status: "failed", abandoned: true, reason: null });
    },
    /** The page is going away: an upload that may not have finished must not stay uploading. */
    unsettled: () => unsettled,
    dispose() {
      controller.abort();
      disposed = true;
      listeners.clear();
    },
  };
}
export type RecordingUploader = ReturnType<typeof createRecordingUploader>;
