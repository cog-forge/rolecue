"use client";

import {
  useEffect,
  useEffectEvent,
  useState,
  useSyncExternalStore,
} from "react";
import {
  createAnswerRecorder,
  type FinishedRecording,
} from "./answer-recorder";

/** The room's answer recorder. Unmount discards an open recording. */
export function useAnswerRecorder(
  onRecorded: (recording: FinishedRecording) => void,
) {
  const [recorder] = useState(() => createAnswerRecorder());
  const recorded = useEffectEvent(onRecorded);
  useEffect(() => {
    const unsubscribe = recorder.subscribeRecorded((recording) =>
      recorded(recording),
    );
    return () => {
      unsubscribe();
      recorder.cancel();
    };
  }, [recorder]);
  const snapshot = useSyncExternalStore(
    recorder.subscribe,
    recorder.getSnapshot,
    recorder.getSnapshot,
  );
  return { snapshot, recorder };
}
