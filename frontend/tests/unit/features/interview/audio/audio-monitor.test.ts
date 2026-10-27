import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { createMicrophoneCapture } from "@/features/interview/audio/microphone-capture";
import { installMediaFakes } from "./media-fakes";

let browser: ReturnType<typeof installMediaFakes>;
let capture: ReturnType<typeof createMicrophoneCapture>;
let amplitude: number;
let contexts: TestContext[];
let failSource: boolean;
class TestContext extends EventTarget {
  state = "running";
  source = { connect: vi.fn(), disconnect: vi.fn() };
  analyser = {
    fftSize: 1024,
    disconnect: vi.fn(),
    getFloatTimeDomainData: (samples: Float32Array) => samples.fill(amplitude),
  };
  constructor() {
    super();
    contexts.push(this);
  }
  createMediaStreamSource() {
    if (failSource) throw new Error("init failed");
    return this.source;
  }
  createAnalyser() {
    return this.analyser;
  }
  resume = vi.fn(async () => {});
  close = vi.fn(async () => {
    this.state = "closed";
  });
  suspend() {
    this.state = "suspended";
    this.dispatchEvent(new Event("statechange"));
  }
}
beforeEach(() => {
  vi.useFakeTimers();
  browser = installMediaFakes();
  contexts = [];
  amplitude = 0;
  failSource = false;
  vi.stubGlobal("AudioContext", TestContext);
  capture = createMicrophoneCapture();
});
afterEach(() => {
  capture.dispose();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it("silence is ready, sustained input drives VAD, mute resets and unmute recovers", async () => {
  await capture.start();
  vi.advanceTimersByTime(100);
  expect(capture.getSnapshot()).toMatchObject({
    readiness: "ready",
    activity: "silent",
    level: 0,
  });
  amplitude = 0.1;
  vi.advanceTimersByTime(250);
  expect(capture.getSnapshot().activity).toBe("speaking");
  capture.setMuted(true);
  vi.advanceTimersByTime(1000);
  expect(capture.getSnapshot()).toMatchObject({
    readiness: "muted",
    activity: "inactive",
    level: 0,
  });
  capture.setMuted(false);
  vi.advanceTimersByTime(50);
  expect(capture.getSnapshot().activity).toBe("silent");
  vi.advanceTimersByTime(200);
  expect(capture.getSnapshot().activity).toBe("speaking");
});

it("suspension clears speaking and explicit analysis retry preserves the capture", async () => {
  await capture.start();
  amplitude = 0.1;
  vi.advanceTimersByTime(250);
  contexts[0].suspend();
  expect(capture.getSnapshot()).toMatchObject({
    readiness: "ready",
    activity: "unavailable",
    level: 0,
  });
  const stream = capture.getSnapshot().stream;
  capture.retryAnalysis();
  vi.advanceTimersByTime(50);
  expect(contexts[0].state).toBe("closed");
  expect(capture.getSnapshot().stream).toBe(stream);
  expect(capture.getSnapshot().activity).toBe("silent");
  expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
});

it("releases a partially initialized graph without discarding a usable microphone", async () => {
  failSource = true;
  await capture.start();
  expect(contexts[0].state).toBe("closed");
  expect(vi.getTimerCount()).toBe(0);
  expect(capture.getSnapshot()).toMatchObject({
    readiness: "ready",
    activity: "unavailable",
  });
  expect(browser.tracks[0].readyState).toBe("live");
  failSource = false;
  capture.retryAnalysis();
  vi.advanceTimersByTime(50);
  expect(capture.getSnapshot().activity).toBe("silent");
});

it("stops sampling and closes/disconnects every graph resource on stop and switch", async () => {
  await capture.start();
  browser.media.getUserMedia.mockResolvedValueOnce(browser.stream("mic-2"));
  await capture.selectDevice("mic-2");
  expect(contexts[0].close).toHaveBeenCalledTimes(1);
  expect(contexts[0].source.disconnect).toHaveBeenCalledTimes(1);
  capture.stop();
  capture.dispose();
  expect(vi.getTimerCount()).toBe(0);
  expect(contexts[1].analyser.disconnect).toHaveBeenCalledTimes(1);
  expect(contexts[1].close).toHaveBeenCalledTimes(1);
  expect(browser.tracks.every((t) => t.readyState === "ended")).toBe(true);
  contexts[1].dispatchEvent(new Event("statechange"));
  expect(capture.getSnapshot().activity).toBe("inactive");
});

it("allows a consumer to stop immediately when a stream is published without leaking analysis", async () => {
  const unsubscribe = capture.subscribe(() => {
    if (capture.getSnapshot().stream) capture.stop();
  });
  await capture.start();
  unsubscribe();
  expect(capture.getSnapshot()).toMatchObject({
    status: "stopped",
    stream: null,
  });
  expect(vi.getTimerCount()).toBe(0);
  expect(contexts.every((context) => context.state === "closed")).toBe(true);
});
