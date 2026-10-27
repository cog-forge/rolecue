import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createMicrophoneCapture } from "@/features/interview/audio/microphone-capture";
import { deferred, device, installMediaFakes } from "./media-fakes";

let browser: ReturnType<typeof installMediaFakes>;
let capture: ReturnType<typeof createMicrophoneCapture>;
beforeEach(() => {
  browser = installMediaFakes();
  capture = createMicrophoneCapture();
});
afterEach(() => {
  capture.dispose();
  vi.unstubAllGlobals();
});

describe("microphone lifecycle", () => {
  it("is inert until start and exposes a stable borrowed audio stream", async () => {
    expect(browser.media.getUserMedia).not.toHaveBeenCalled();
    expect(capture.getSnapshot()).toBe(capture.getSnapshot());
    await capture.start();
    expect(browser.media.getUserMedia).toHaveBeenCalledWith({
      audio: true,
      video: false,
    });
    expect(capture.getSnapshot()).toMatchObject({
      status: "active",
      readiness: "ready",
      activity: "unavailable",
      revision: 1,
    });
    const stream = capture.getSnapshot().stream;
    await capture.start();
    expect(capture.getSnapshot().stream).toBe(stream);
    expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
  });

  it.each([
    ["NotAllowedError", "permission-denied"],
    ["NotFoundError", "no-device"],
    ["NotSupportedError", "unsupported"],
    ["NotReadableError", "device-unavailable"],
    ["AbortError", "device-unavailable"],
    ["OverconstrainedError", "constraints"],
    ["UnknownError", "unknown"],
  ])("maps %s and permits explicit recovery", async (name, code) => {
    browser.media.getUserMedia.mockRejectedValueOnce(
      new DOMException("private details", name),
    );
    await capture.start();
    expect(capture.getSnapshot().error?.code).toBe(code);
    expect(capture.getSnapshot().error?.message).not.toContain(
      "private details",
    );
    expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
    await capture.retry();
    expect(capture.getSnapshot().readiness).toBe("ready");
  });

  it("does not request permission in insecure or unsupported environments", async () => {
    vi.stubGlobal("isSecureContext", false);
    await capture.start();
    expect(capture.getSnapshot().error?.code).toBe("unsupported");
    expect(browser.media.getUserMedia).not.toHaveBeenCalled();
  });

  it("serializes requests and stops permission results arriving after cancellation", async () => {
    const request = deferred<MediaStream>();
    browser.media.getUserMedia.mockReturnValueOnce(request.promise);
    const start = capture.start();
    void capture.start();
    capture.stop();
    void capture.retry();
    expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
    const late = browser.stream();
    request.resolve(late);
    await start;
    expect(late.getTracks()[0].readyState).toBe("ended");
    expect(capture.getSnapshot()).toMatchObject({
      status: "stopped",
      stream: null,
      requestPending: false,
    });
    await capture.retry();
    expect(capture.getSnapshot().readiness).toBe("ready");
  });

  it("disposes late streams and never notifies disposed subscribers", async () => {
    const request = deferred<MediaStream>();
    browser.media.getUserMedia.mockReturnValueOnce(request.promise);
    const listener = vi.fn();
    capture.subscribe(listener);
    const start = capture.start();
    capture.dispose();
    listener.mockClear();
    request.resolve(browser.stream());
    await start;
    expect(browser.tracks[0].readyState).toBe("ended");
    expect(listener).not.toHaveBeenCalled();
    await capture.start();
    expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
  });

  it("cleans up listeners and tracks idempotently and supports unsubscribe", async () => {
    const listener = vi.fn();
    const unsubscribe = capture.subscribe(listener);
    await capture.start();
    unsubscribe();
    listener.mockClear();
    capture.stop();
    capture.stop();
    browser.media.dispatchEvent(new Event("devicechange"));
    browser.tracks[0].end();
    expect(browser.tracks[0].stop).toHaveBeenCalledTimes(1);
    expect(capture.getSnapshot().status).toBe("stopped");
    expect(listener).not.toHaveBeenCalled();
  });
});

describe("device selection and recovery", () => {
  it("selects while idle without capture; switches active streams preserving user mute", async () => {
    await capture.selectDevice("mic-2");
    expect(browser.media.getUserMedia).not.toHaveBeenCalled();
    browser.media.getUserMedia.mockResolvedValueOnce(browser.stream("mic-2"));
    await capture.start();
    capture.setMuted(true);
    const previous = capture.getSnapshot().stream;
    browser.media.getUserMedia.mockResolvedValueOnce(browser.stream("mic-1"));
    await capture.selectDevice("mic-1");
    expect(previous?.getTracks()[0].readyState).toBe("ended");
    expect(capture.getSnapshot()).toMatchObject({
      selectedDeviceId: "mic-1",
      actualDeviceId: "mic-1",
      muted: true,
      readiness: "muted",
      revision: 2,
      level: 0,
      activity: "inactive",
    });
    expect(capture.getSnapshot().stream?.getTracks()[0].enabled).toBe(false);
    capture.setMuted(false);
    expect(capture.getSnapshot().readiness).toBe("ready");
    expect(browser.media.getUserMedia).toHaveBeenLastCalledWith({
      audio: { deviceId: { exact: "mic-1" } },
      video: false,
    });
  });

  it("distinguishes source interruption from intentional mute", async () => {
    await capture.start();
    browser.tracks[0].interrupt(true);
    expect(capture.getSnapshot().readiness).toBe("interrupted");
    capture.setMuted(true);
    browser.tracks[0].interrupt(false);
    expect(capture.getSnapshot().readiness).toBe("muted");
    capture.setMuted(false);
    expect(capture.getSnapshot().readiness).toBe("ready");
    expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
  });

  it("invalidates an ended stream and retries only when requested", async () => {
    await capture.start();
    browser.tracks[0].end();
    expect(capture.getSnapshot()).toMatchObject({
      status: "error",
      stream: null,
      level: 0,
      error: { code: "device-lost" },
    });
    browser.media.dispatchEvent(new Event("devicechange"));
    await vi.waitFor(() =>
      expect(browser.media.enumerateDevices).toHaveBeenCalledTimes(2),
    );
    expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
    await capture.retry();
    expect(capture.getSnapshot().readiness).toBe("ready");
  });

  it("refreshes unrelated devices without replacing capture, then detects selected device removal", async () => {
    await capture.selectDevice("mic-1");
    await capture.start();
    const stream = capture.getSnapshot().stream;
    browser.media.enumerateDevices.mockResolvedValueOnce([
      device("mic-1"),
      device("mic-3", ""),
    ]);
    browser.media.dispatchEvent(new Event("devicechange"));
    await vi.waitFor(() =>
      expect(
        capture.getSnapshot().devices.some((d) => d.deviceId === "mic-3"),
      ).toBe(true),
    );
    expect(capture.getSnapshot().stream).toBe(stream);
    browser.media.enumerateDevices.mockResolvedValueOnce([device("mic-3")]);
    browser.media.dispatchEvent(new Event("devicechange"));
    await vi.waitFor(() =>
      expect(capture.getSnapshot().error?.code).toBe("device-lost"),
    );
    expect(stream?.getTracks()[0].readyState).toBe("ended");
  });

  it("does not let stale enumeration overwrite a newer list or invalidate a good stream", async () => {
    await capture.start();
    const old = deferred<MediaDeviceInfo[]>();
    browser.media.enumerateDevices.mockReturnValueOnce(old.promise);
    browser.media.dispatchEvent(new Event("devicechange"));
    browser.media.enumerateDevices.mockResolvedValueOnce([
      device("mic-1"),
      device("new"),
    ]);
    browser.media.dispatchEvent(new Event("devicechange"));
    await vi.waitFor(() =>
      expect(
        capture.getSnapshot().devices.some((d) => d.deviceId === "new"),
      ).toBe(true),
    );
    old.resolve([]);
    await old.promise;
    expect(capture.getSnapshot().readiness).toBe("ready");
    expect(capture.getSnapshot().devices).toHaveLength(2);
  });

  it("keeps a live stream if enumeration fails", async () => {
    browser.media.enumerateDevices.mockRejectedValueOnce(
      new Error("device list failed"),
    );
    await capture.start();
    expect(capture.getSnapshot().readiness).toBe("ready");
    expect(capture.getSnapshot().deviceWarning).toBeTruthy();
  });
});

it("retains unavailable analysis after mute/unmute when Web Audio is unsupported", async () => {
  await capture.start();
  capture.setMuted(true);
  capture.setMuted(false);
  expect(capture.getSnapshot()).toMatchObject({
    readiness: "ready",
    activity: "unavailable",
  });
});

it("recovers after a synchronous browser exception instead of leaving retry pending", async () => {
  browser.media.getUserMedia.mockImplementationOnce(() => {
    throw new DOMException("blocked", "NotAllowedError");
  });
  await capture.start();
  expect(capture.getSnapshot().requestPending).toBe(false);
  await capture.retry();
  expect(capture.getSnapshot().readiness).toBe("ready");
});

it("publishes readiness and stream invalidation together", async () => {
  await capture.start();
  const invalidStates: boolean[] = [];
  capture.subscribe(() => {
    const state = capture.getSnapshot();
    invalidStates.push(state.readiness === "ready" && state.stream === null);
  });
  capture.stop();
  expect(invalidStates).not.toContain(true);
});

it("refreshes a failed device list explicitly without replacing the live stream", async () => {
  browser.media.enumerateDevices.mockRejectedValueOnce(
    new Error("enumeration failed"),
  );
  await capture.start();
  const stream = capture.getSnapshot().stream;
  expect(capture.getSnapshot().deviceWarning).toBeTruthy();
  await capture.refreshDevices();
  expect(capture.getSnapshot().deviceWarning).toBeNull();
  expect(capture.getSnapshot().devices).toHaveLength(2);
  expect(capture.getSnapshot().stream).toBe(stream);
  expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
});

it("publishes mute/readiness only after applying the track change", async () => {
  await capture.start();
  const inconsistent: boolean[] = [];
  capture.subscribe(() => {
    const state = capture.getSnapshot();
    inconsistent.push(
      state.readiness === "ready" &&
        (state.muted || !state.stream?.getAudioTracks()[0].enabled),
    );
  });
  capture.setMuted(true);
  capture.setMuted(false);
  expect(inconsistent).not.toContain(true);
});
