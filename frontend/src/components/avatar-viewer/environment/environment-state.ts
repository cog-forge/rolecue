/**
 * Lifecycle of the 3D room behind the interviewer. `off` means a 2D image was chosen; every 3D
 * failure ends in `fallback`, where the scene's still image stays on screen instead.
 */
export type EnvironmentStatus =
  | { state: "off" }
  | { state: "loading"; progress: number }
  | { state: "ready" }
  | { state: "fallback"; reason: EnvironmentFallbackReason };

/** `unsupported`: no or software WebGL, or no interviewer canvas to draw into. */
export type EnvironmentFallbackReason = "unsupported" | "error" | "timeout";

export type EnvironmentEvent =
  | { type: "progress"; loaded: number; total: number }
  | { type: "loaded" }
  | { type: "failed" }
  | { type: "timed-out" };

export const environmentLoading: EnvironmentStatus = {
  state: "loading",
  progress: 0,
};

// Only a loading scene moves: late events from a download that already finished, failed or timed out
// are ignored, and progress never goes backwards or reaches 100% before the scene is built.
export function environmentReducer(
  status: EnvironmentStatus,
  event: EnvironmentEvent,
): EnvironmentStatus {
  if (status.state !== "loading") return status;
  switch (event.type) {
    case "progress": {
      const progress =
        event.total > 0 ? Math.min(0.99, event.loaded / event.total) : 0;
      return progress > status.progress
        ? { state: "loading", progress }
        : status;
    }
    case "loaded":
      return { state: "ready" };
    case "failed":
      return { state: "fallback", reason: "error" };
    case "timed-out":
      return { state: "fallback", reason: "timeout" };
  }
}
