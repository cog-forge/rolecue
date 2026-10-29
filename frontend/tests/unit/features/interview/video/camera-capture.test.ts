import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { createCameraCapture } from "@/features/interview/video/camera-capture";
import { deferred } from "../audio/media-fakes";

class Track extends EventTarget {
  readyState = "live";
  stop = vi.fn(() => {
    this.readyState = "ended";
  });
}
class Stream {
  track = new Track();
  getTracks() {
    return [this.track];
  }
  getVideoTracks() {
    return [this.track];
  }
  getAudioTracks() {
    return [];
  }
}
const getUserMedia = vi.fn<MediaDevices["getUserMedia"]>();
beforeEach(() => {
  getUserMedia.mockReset();
  vi.stubGlobal("MediaStream", Stream);
  vi.stubGlobal("isSecureContext", true);
  Object.defineProperty(navigator, "mediaDevices", {
    configurable: true,
    value: { getUserMedia },
  });
});
afterEach(() => vi.unstubAllGlobals());
it("is opt-in, captures only video, and stops the actual track", async () => {
  getUserMedia.mockResolvedValue(new MediaStream());
  const capture = createCameraCapture();
  expect(getUserMedia).not.toHaveBeenCalled();
  await capture.start();
  expect(getUserMedia).toHaveBeenCalledExactlyOnceWith({
    audio: false,
    video: { facingMode: "user" },
  });
  const live = capture.getSnapshot().stream;
  expect(live).not.toBeNull();
  capture.stop();
  expect(live?.getTracks()[0].readyState).toBe("ended");
  expect(capture.getSnapshot().stream).toBeNull();
  capture.dispose();
});
it("serializes permission prompts and releases a late stream after cancellation", async () => {
  const permission = deferred<MediaStream>();
  getUserMedia.mockReturnValue(permission.promise);
  const capture = createCameraCapture();
  const first = capture.start();
  expect(capture.start()).toBe(first);
  capture.stop();
  const stream = new MediaStream();
  permission.resolve(stream);
  await first;
  expect(stream.getTracks()[0].readyState).toBe("ended");
  expect(capture.getSnapshot()).toMatchObject({
    status: "off",
    stream: null,
    requestPending: false,
  });
  expect(getUserMedia).toHaveBeenCalledTimes(1);
  capture.dispose();
});
it("handles permission denial and device loss without keeping tracks alive", async () => {
  getUserMedia.mockRejectedValueOnce(
    new DOMException("Denied", "NotAllowedError"),
  );
  const capture = createCameraCapture();
  await capture.start();
  expect(capture.getSnapshot().error).toMatch(/permission/i);
  const stream = new MediaStream();
  getUserMedia.mockResolvedValue(stream);
  await capture.start();
  stream.getTracks()[0].dispatchEvent(new Event("ended"));
  expect(capture.getSnapshot()).toMatchObject({
    status: "error",
    stream: null,
  });
  expect(stream.getTracks()[0].readyState).toBe("ended");
  capture.dispose();
});

it("recovers from a synchronous browser permission failure", async () => {
  getUserMedia.mockImplementationOnce(() => {
    throw new DOMException("Blocked", "SecurityError");
  });
  const capture = createCameraCapture();
  await capture.start();
  getUserMedia.mockResolvedValue(new MediaStream());
  await capture.start();
  expect(capture.getSnapshot().status).toBe("active");
  capture.dispose();
});
