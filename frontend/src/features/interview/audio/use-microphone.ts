"use client";

import { useEffect, useRef, useState } from "react";
import {
  createMicrophoneCapture,
  INITIAL_MICROPHONE_SNAPSHOT,
  type MicrophoneCapture,
} from "./microphone-capture";

/** Each mounted consumer owns its capture. Route changes release it, never transfer it. */
export function useMicrophone() {
  const controller = useRef<MicrophoneCapture | null>(null);
  const [snapshot, setSnapshot] = useState(INITIAL_MICROPHONE_SNAPSHOT);
  useEffect(() => {
    const capture = createMicrophoneCapture();
    controller.current = capture;
    const unsubscribe = capture.subscribe(() =>
      setSnapshot(capture.getSnapshot()),
    );
    return () => {
      unsubscribe();
      capture.dispose();
      controller.current = null;
    };
  }, []);
  return {
    snapshot,
    start: () => controller.current?.start(),
    retry: () => controller.current?.retry(),
    refreshDevices: () => controller.current?.refreshDevices(),
    selectDevice: (id: string) => controller.current?.selectDevice(id),
    setMuted: (muted: boolean) => controller.current?.setMuted(muted),
    retryAnalysis: () => controller.current?.retryAnalysis(),
    stop: () => controller.current?.stop(),
  };
}
