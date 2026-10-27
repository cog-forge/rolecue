import { createAudioMonitor } from "./audio-monitor";
import type { VoiceActivity, VoiceActivityOptions } from "./voice-activity";

export type MicrophoneErrorCode =
  | "unsupported"
  | "permission-denied"
  | "no-device"
  | "device-unavailable"
  | "constraints"
  | "device-lost"
  | "unknown";
export interface MicrophoneError {
  code: MicrophoneErrorCode;
  message: string;
}
export interface MicrophoneSnapshot {
  status:
    "idle" | "requesting" | "active" | "interrupted" | "error" | "stopped";
  readiness: "not-ready" | "ready" | "muted" | "interrupted" | "error";
  activity: VoiceActivity;
  level: number;
  muted: boolean;
  requestPending: boolean;
  devices: readonly { deviceId: string; label: string }[];
  selectedDeviceId: string;
  actualDeviceId: string | null;
  actualDeviceLabel: string | null;
  /** Borrowed, controller-owned stream. Consumers must not stop/mutate/clone its tracks. */
  stream: MediaStream | null;
  /** Increments on successful acquisition, not on meter updates. */
  revision: number;
  error: MicrophoneError | null;
  analysisWarning: string | null;
  deviceWarning: string | null;
}
export const INITIAL_MICROPHONE_SNAPSHOT: MicrophoneSnapshot = {
  status: "idle",
  readiness: "not-ready",
  activity: "inactive",
  level: 0,
  muted: false,
  requestPending: false,
  devices: [],
  selectedDeviceId: "default",
  actualDeviceId: null,
  actualDeviceLabel: null,
  stream: null,
  revision: 0,
  error: null,
  analysisWarning: null,
  deviceWarning: null,
};

function captureError(error: unknown): MicrophoneError {
  const name =
    typeof error === "object" && error !== null && "name" in error
      ? error.name
      : "";
  switch (name) {
    case "NotAllowedError":
    case "SecurityError":
      return {
        code: "permission-denied",
        message:
          "Microphone access is not allowed. Check this site's browser permission, your system privacy settings or site policy, then retry.",
      };
    case "NotSupportedError":
      return {
        code: "unsupported",
        message:
          "This browser could not capture microphone audio. Try a supported browser and check your device settings.",
      };
    case "NotFoundError":
      return {
        code: "no-device",
        message:
          "No microphone was found. Connect a microphone or choose another input, then retry.",
      };
    case "NotReadableError":
    case "AbortError":
      return {
        code: "device-unavailable",
        message:
          "The microphone could not be opened. Check the connection and close other apps using it, then retry.",
      };
    case "OverconstrainedError":
      return {
        code: "constraints",
        message:
          "The selected microphone is unavailable. Choose another input and retry.",
      };
    default:
      return {
        code: "unknown",
        message:
          "The microphone check could not finish. Check your device and retry.",
      };
  }
}

/** No browser work at creation/import. Call start from an explicit user action. */
export function createMicrophoneCapture(
  options?: Partial<VoiceActivityOptions>,
) {
  let snapshot = INITIAL_MICROPHONE_SNAPSHOT;
  const listeners = new Set<() => void>();
  let disposed = false;
  let generation = 0;
  let enumeration = 0;
  let pending: Promise<void> | null = null;
  let media: MediaDevices | null = null;
  let stream: MediaStream | null = null;
  let track: MediaStreamTrack | null = null;
  let monitor: ReturnType<typeof createAudioMonitor> | null = null;
  let detachTrack: (() => void) | null = null;

  function publish(patch: Partial<MicrophoneSnapshot>) {
    if (
      disposed ||
      Object.entries(patch).every(
        ([key, value]) => snapshot[key as keyof MicrophoneSnapshot] === value,
      )
    )
      return;
    snapshot = { ...snapshot, ...patch };
    listeners.forEach((listener) => listener());
  }
  function releaseCapture() {
    detachTrack?.();
    detachTrack = null;
    monitor?.dispose();
    monitor = null;
    stream?.getTracks().forEach((item) => item.stop());
    stream = null;
    track = null;
  }
  function invalidate(patch: Partial<MicrophoneSnapshot>) {
    releaseCapture();
    publish({
      stream: null,
      actualDeviceId: null,
      actualDeviceLabel: null,
      level: 0,
      activity: "inactive",
      analysisWarning: null,
      ...patch,
    });
  }
  function lostDevice() {
    invalidate({
      status: "error",
      readiness: "error",
      error: {
        code: "device-lost",
        message:
          "The microphone disconnected. Reconnect it or choose another input, then retry.",
      },
    });
  }
  function updateReadiness(muted = snapshot.muted) {
    if (!track) {
      publish({ muted });
      return;
    }
    const interrupted = track.muted;
    const enabled = !muted && !interrupted;
    publish({
      muted,
      status: interrupted ? "interrupted" : "active",
      readiness: interrupted ? "interrupted" : muted ? "muted" : "ready",
      ...(!enabled ? { activity: "inactive" as const, level: 0 } : {}),
    });
    monitor?.setEnabled(enabled);
  }
  function startMonitor() {
    if (!stream) return;
    monitor?.dispose();
    monitor = null;
    const currentGeneration = generation;
    const analysis = createAudioMonitor(
      stream,
      (sample) => {
        if (disposed || generation !== currentGeneration || !track) return;
        publish({
          analysisWarning: sample.warning,
          activity:
            snapshot.readiness === "ready" ? sample.activity : "inactive",
          level: snapshot.readiness === "ready" ? sample.level : 0,
        });
      },
      options,
    );
    if (disposed || generation !== currentGeneration || !stream)
      analysis.dispose();
    else monitor = analysis;
  }
  async function refreshDevices() {
    if (!media || disposed) return;
    const request = ++enumeration;
    const currentGeneration = generation;
    try {
      const devices = await media.enumerateDevices();
      if (
        disposed ||
        request !== enumeration ||
        currentGeneration !== generation
      )
        return;
      const inputs = devices.filter(
        (item) => item.kind === "audioinput" && item.deviceId,
      );
      const unique = [
        ...new Map(inputs.map((item) => [item.deviceId, item])).values(),
      ];
      publish({
        devices: unique.map((item, index) => ({
          deviceId: item.deviceId,
          label: item.label || `Microphone ${index + 1}`,
        })),
        deviceWarning: null,
      });
      // Only a deliberately selected device can be proven missing from this list.
      // The default alias may follow an OS device without ending its track.
      if (
        track &&
        snapshot.selectedDeviceId !== "default" &&
        !unique.some((item) => item.deviceId === snapshot.selectedDeviceId)
      )
        lostDevice();
    } catch {
      if (
        !disposed &&
        request === enumeration &&
        currentGeneration === generation
      )
        publish({
          deviceWarning:
            "The microphone list could not be refreshed. Your current input may still work; refresh inputs to try again.",
        });
    }
  }
  const deviceChanged = () => {
    void refreshDevices();
  };
  function detachDevices() {
    media?.removeEventListener("devicechange", deviceChanged);
    media = null;
    enumeration++;
  }
  function start(): Promise<void> {
    if (disposed) return Promise.resolve();
    // getUserMedia has no abort API. Keep an outstanding prompt serialized even after Stop.
    if (pending) return pending;
    if (track?.readyState === "live") return Promise.resolve();
    if (
      typeof window === "undefined" ||
      !window.isSecureContext ||
      !navigator.mediaDevices?.getUserMedia
    ) {
      publish({
        status: "error",
        readiness: "error",
        error: {
          code: "unsupported",
          message:
            "Microphone access requires a supported browser on HTTPS or localhost.",
        },
      });
      return Promise.resolve();
    }
    const currentGeneration = ++generation;
    detachDevices();
    media = navigator.mediaDevices;
    media.addEventListener("devicechange", deviceChanged);
    publish({
      status: "requesting",
      readiness: "not-ready",
      requestPending: true,
      error: null,
      deviceWarning: null,
    });
    const selected = snapshot.selectedDeviceId;
    const audio =
      selected === "default" ? true : { deviceId: { exact: selected } };
    let request: Promise<MediaStream>;
    try {
      request = navigator.mediaDevices.getUserMedia({ audio, video: false });
    } catch (error) {
      request = Promise.reject(error);
    }
    pending = (async () => {
      try {
        const acquired = await request;
        if (disposed || generation !== currentGeneration) {
          acquired.getTracks().forEach((item) => item.stop());
          return;
        }
        stream = acquired;
        track = acquired.getAudioTracks()[0] ?? null;
        if (!track || track.readyState !== "live") {
          lostDevice();
          return;
        }
        track.enabled = !snapshot.muted;
        const currentTrack = track;
        const ended = () => {
          if (generation === currentGeneration && !disposed) lostDevice();
        };
        const sourceChanged = () => {
          if (generation === currentGeneration && !disposed) updateReadiness();
        };
        track.addEventListener("ended", ended);
        track.addEventListener("mute", sourceChanged);
        track.addEventListener("unmute", sourceChanged);
        detachTrack = () => {
          currentTrack.removeEventListener("ended", ended);
          currentTrack.removeEventListener("mute", sourceChanged);
          currentTrack.removeEventListener("unmute", sourceChanged);
        };
        publish({
          stream: acquired,
          revision: snapshot.revision + 1,
          actualDeviceId: track.getSettings().deviceId ?? null,
          actualDeviceLabel: track.label || null,
        });
        updateReadiness();
        if (disposed || generation !== currentGeneration || !stream) return;
        startMonitor();
        updateReadiness();
        void refreshDevices();
      } catch (error) {
        if (disposed || generation !== currentGeneration) return;
        invalidate({
          status: "error",
          readiness: "error",
          error: captureError(error),
        });
      } finally {
        pending = null;
        publish({ requestPending: false });
      }
    })();
    return pending;
  }
  function stop() {
    generation++;
    detachDevices();
    invalidate({
      status: "stopped",
      readiness: "not-ready",
      error: null,
      deviceWarning: null,
    });
  }
  return {
    getSnapshot: () => snapshot,
    subscribe(listener: () => void) {
      if (!disposed) listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    start,
    refreshDevices,
    retry() {
      if (disposed || pending) return pending ?? Promise.resolve();
      stop();
      return start();
    },
    async selectDevice(deviceId: string) {
      if (
        disposed ||
        pending ||
        !deviceId ||
        deviceId === snapshot.selectedDeviceId
      )
        return;
      const wasActive = stream !== null;
      if (wasActive) stop();
      publish({ selectedDeviceId: deviceId });
      if (wasActive) await start();
    },
    setMuted(muted: boolean) {
      if (disposed) return;
      if (track) track.enabled = !muted;
      updateReadiness(muted);
    },
    retryAnalysis() {
      if (disposed || !stream) return;
      startMonitor();
      updateReadiness();
    },
    stop,
    dispose() {
      if (disposed) return;
      listeners.clear();
      stop();
      disposed = true;
    },
  };
}
export type MicrophoneCapture = ReturnType<typeof createMicrophoneCapture>;
