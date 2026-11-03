import { useEffect, useRef } from "react";
import { useThree } from "@react-three/fiber";
import {
  environmentLoading,
  environmentReducer,
  type EnvironmentEvent,
  type EnvironmentStatus,
} from "./environment-state";
import { loadEnvironment } from "./load-environment";
import type { EnvironmentScene } from "./scenes";

/** A scene still missing after this long falls back to its still image. */
export const environmentTimeoutMs = 20_000;

/**
 * Loads one scene into the interviewer's canvas and reports its status. Remount it (key by scene) to
 * switch scenes: the previous one is removed and its GPU resources freed first.
 */
export function EnvironmentLayer({
  scene,
  onStatus,
}: {
  scene: EnvironmentScene;
  onStatus: (status: EnvironmentStatus) => void;
}) {
  const target = useThree((state) => state.scene);
  const invalidate = useThree((state) => state.invalidate);
  // Owners often pass a fresh callback each render; that must not restart the download.
  const report = useRef(onStatus);
  useEffect(() => {
    report.current = onStatus;
  }, [onStatus]);

  useEffect(() => {
    const abort = new AbortController();
    let status = environmentLoading;
    const send = (event: EnvironmentEvent) => {
      const next = environmentReducer(status, event);
      if (next === status || abort.signal.aborted) return;
      status = next;
      report.current(next);
    };
    report.current(status);
    const timer = window.setTimeout(
      () => send({ type: "timed-out" }),
      environmentTimeoutMs,
    );
    let dispose: (() => void) | undefined;
    loadEnvironment(scene, abort.signal, (loaded, total) =>
      send({ type: "progress", loaded, total }),
    )
      .then((built) => {
        if (!built) return;
        // Arrived after the timeout: the still is already showing, so drop the late scene.
        if (status.state !== "loading") return built.dispose();
        dispose = built.dispose;
        target.add(built.root);
        target.environment = built.lighting;
        target.environmentRotation.y = scene.rotation;
        window.clearTimeout(timer);
        send({ type: "loaded" });
        invalidate();
      })
      .catch(() => send({ type: "failed" }));
    return () => {
      abort.abort();
      window.clearTimeout(timer);
      if (dispose) {
        target.environment = null;
        dispose();
        invalidate();
      }
    };
  }, [scene, target, invalidate]);

  return null;
}
