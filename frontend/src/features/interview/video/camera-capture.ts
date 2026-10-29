export type CameraSnapshot = {
  status: "off" | "requesting" | "active" | "error";
  stream: MediaStream | null;
  requestPending: boolean;
  error: string | null;
};
export const INITIAL_CAMERA_SNAPSHOT: CameraSnapshot = {
  status: "off",
  stream: null,
  requestPending: false,
  error: null,
};

function cameraError(error: unknown) {
  const name =
    typeof error === "object" && error !== null && "name" in error
      ? error.name
      : "";
  if (name === "NotAllowedError" || name === "SecurityError")
    return "Camera permission is blocked. Allow camera access in your browser and try again.";
  if (name === "NotFoundError")
    return "No camera was found. Connect a camera and try again.";
  if (name === "NotReadableError")
    return "Your camera is unavailable. Close other apps using it and try again.";
  return "We could not open your camera. Check your device and try again.";
}

/** This controller owns only local video. It never opens audio or uploads a stream. */
export function createCameraCapture() {
  let snapshot = INITIAL_CAMERA_SNAPSHOT;
  const listeners = new Set<() => void>();
  let disposed = false;
  let generation = 0;
  let pending: Promise<void> | null = null;
  let detach: (() => void) | null = null;
  function publish(patch: Partial<CameraSnapshot>) {
    if (disposed) return;
    snapshot = { ...snapshot, ...patch };
    listeners.forEach((listener) => listener());
  }
  function release() {
    detach?.();
    detach = null;
    snapshot.stream?.getTracks().forEach((track) => track.stop());
  }
  function stop() {
    generation++;
    release();
    publish({ status: "off", stream: null, error: null });
  }
  function start(): Promise<void> {
    if (disposed) return Promise.resolve();
    if (pending) return pending;
    if (snapshot.status === "active") return Promise.resolve();
    if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
      publish({
        status: "error",
        error:
          "Camera access requires HTTPS or localhost in a supported browser.",
      });
      return Promise.resolve();
    }
    const current = ++generation;
    publish({ status: "requesting", requestPending: true, error: null });
    let request: Promise<MediaStream>;
    try {
      request = navigator.mediaDevices.getUserMedia({
        audio: false,
        video: { facingMode: "user" },
      });
    } catch (error) {
      request = Promise.reject(error);
    }
    pending = (async () => {
      try {
        const stream = await request;
        if (disposed || current !== generation) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }
        const video = stream.getVideoTracks()[0];
        if (!video || video.readyState !== "live") {
          stream.getTracks().forEach((track) => track.stop());
          throw new DOMException("No video track", "NotFoundError");
        }
        const ended = () => {
          release();
          publish({
            status: "error",
            stream: null,
            error: "Your camera disconnected. Reconnect it and try again.",
          });
        };
        video.addEventListener("ended", ended);
        detach = () => video.removeEventListener("ended", ended);
        publish({ status: "active", stream, error: null });
      } catch (error) {
        if (!disposed && current === generation)
          publish({ status: "error", stream: null, error: cameraError(error) });
      } finally {
        pending = null;
        publish({ requestPending: false });
      }
    })();
    return pending;
  }
  return {
    start,
    stop,
    getSnapshot: () => snapshot,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    dispose() {
      if (disposed) return;
      stop();
      disposed = true;
      listeners.clear();
    },
  };
}
