"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { createCameraCapture, INITIAL_CAMERA_SNAPSHOT } from "./camera-capture";

export function useCamera() {
  const controller = useRef<ReturnType<typeof createCameraCapture> | null>(
    null,
  );
  const [snapshot, setSnapshot] = useState(INITIAL_CAMERA_SNAPSHOT);
  useEffect(() => {
    const capture = createCameraCapture();
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
  const start = useCallback(() => controller.current?.start(), []);
  const stop = useCallback(() => controller.current?.stop(), []);
  return { snapshot, start, stop };
}
