"use client";

import {
  useCallback,
  useEffect,
  useEffectEvent,
  useRef,
  useState,
} from "react";
import { completeRecordingUploadOnExit } from "./recording-api";
import { acceptedConsentVersion } from "./recording-consent";
import {
  createIndexedDbRecordingStore,
  type RecordingBufferStore,
} from "./recording-buffer";
import {
  createRecordingUploader,
  INITIAL_UPLOAD_SNAPSHOT,
  type RecordingUploader,
} from "./recording-upload";
import {
  createSessionRecorder,
  INITIAL_RECORDER_SNAPSHOT,
  type SessionRecorder,
} from "./session-recorder";

/**
 * off: practice, or the type is not known yet; nothing is recorded and no recording call is made.
 * live: a recruitment interview in progress. finished: a completed one, opened again to upload.
 * The mode only decides how the room begins; a later change of live to finished is the completion,
 * which the room reports with finish().
 */
export type RecordingMode = "off" | "live" | "finished";

let sharedStore: RecordingBufferStore | null = null;
function browserStore() {
  sharedStore ??= createIndexedDbRecordingStore();
  return sharedStore;
}

const busyUploads = new Set(["checking", "preparing", "uploading", "retrying"]);

/** The recruitment recording of one room: capture while live, then upload once completed. */
export function useInterviewRecording({
  interviewId,
  ownerId,
  mode,
  micMuted,
  store,
}: {
  interviewId: string;
  ownerId: string;
  mode: RecordingMode;
  micMuted: boolean;
  store?: RecordingBufferStore;
}) {
  const controllers = useRef<{
    recorder: SessionRecorder;
    uploader: RecordingUploader;
  } | null>(null);
  const [recorder, setRecorder] = useState(INITIAL_RECORDER_SNAPSHOT);
  const [upload, setUpload] = useState(INITIAL_UPLOAD_SNAPSHOT);
  const enabled = mode !== "off";
  const begin = useEffectEvent(
    (current: NonNullable<typeof controllers.current>) => {
      current.recorder.setAudioEnabled(!micMuted);
      if (mode === "live") void current.recorder.start();
      else void current.uploader.run();
    },
  );
  useEffect(() => {
    if (!enabled) return;
    const buffer = (store ?? browserStore()).open(ownerId, interviewId);
    const current = {
      recorder: createSessionRecorder({ buffer }),
      uploader: createRecordingUploader({
        interviewId,
        buffer,
        consentVersion: () => acceptedConsentVersion(ownerId),
      }),
    };
    controllers.current = current;
    const unsubscribe = [
      current.recorder.subscribe(() =>
        setRecorder(current.recorder.getSnapshot()),
      ),
      current.uploader.subscribe(() =>
        setUpload(current.uploader.getSnapshot()),
      ),
    ];
    // Leaving while an upload is busy asks first; if the page goes anyway, the upload that may not
    // have finished is completed, so it ends failed (and can be retried) instead of uploading forever.
    const warn = (event: BeforeUnloadEvent) => {
      if (busyUploads.has(current.uploader.getSnapshot().status))
        event.preventDefault();
    };
    const leave = () => {
      if (current.uploader.unsettled())
        completeRecordingUploadOnExit(interviewId);
    };
    window.addEventListener("beforeunload", warn);
    window.addEventListener("pagehide", leave);
    begin(current);
    return () => {
      window.removeEventListener("beforeunload", warn);
      window.removeEventListener("pagehide", leave);
      unsubscribe.forEach((stop) => stop());
      leave();
      current.recorder.dispose();
      current.uploader.dispose();
      controllers.current = null;
    };
  }, [enabled, interviewId, ownerId, store]);
  useEffect(() => {
    controllers.current?.recorder.setAudioEnabled(!micMuted);
  }, [micMuted]);

  /** The interview completed: close the last segment, then upload the whole recording. */
  const finish = useCallback(async () => {
    const current = controllers.current;
    if (!current) return;
    await current.recorder.stop();
    await current.uploader.run();
  }, []);
  /** The room stopped (another window took over, the window closed): end the segment, keep it. */
  const pause = useCallback(() => {
    void controllers.current?.recorder.stop();
  }, []);
  const retryCamera = useCallback(() => {
    void controllers.current?.recorder.start();
  }, []);
  const retryUpload = useCallback(() => {
    void controllers.current?.uploader.run();
  }, []);
  const giveUp = useCallback(() => {
    void controllers.current?.uploader.giveUp();
  }, []);

  return {
    enabled,
    recorder,
    upload,
    finish,
    pause,
    retryCamera,
    retryUpload,
    giveUp,
  };
}
export type InterviewRecording = ReturnType<typeof useInterviewRecording>;
