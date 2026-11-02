import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  createAnswerRecorder,
  pickAnswerMimeType,
  type FinishedRecording,
} from "@/features/interview/answer/answer-recorder";

class FakeRecorder {
  static instances: FakeRecorder[] = [];
  static supported = new Set(["audio/webm;codecs=opus", "audio/mp4"]);
  static isTypeSupported(type: string) {
    return FakeRecorder.supported.has(type);
  }
  state: "inactive" | "recording" = "inactive";
  mimeType: string;
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(
    readonly stream: MediaStream,
    readonly options: MediaRecorderOptions,
  ) {
    this.mimeType = options.mimeType ?? "audio/webm";
    FakeRecorder.instances.push(this);
  }
  start() {
    this.state = "recording";
  }
  stop() {
    this.state = "inactive";
    this.ondataavailable?.({
      data: new Blob([new Uint8Array([1, 2, 3])], { type: this.mimeType }),
    });
    this.onstop?.();
  }
}
const track = { stop: vi.fn() };
const stream = { getTracks: () => [track] } as unknown as MediaStream;
let clock = 0;
function setup() {
  const recorder = createAnswerRecorder({ now: () => clock });
  const recorded: FinishedRecording[] = [];
  recorder.subscribeRecorded((recording) => recorded.push(recording));
  return { recorder, recorded };
}

beforeEach(() => {
  vi.useFakeTimers();
  clock = 0;
  FakeRecorder.instances = [];
  track.stop.mockClear();
  vi.stubGlobal("MediaRecorder", FakeRecorder);
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it("prefers WebM/Opus, then the next supported type", () => {
  expect(pickAnswerMimeType((type) => type === "audio/mp4")).toBe("audio/mp4");
  expect(pickAnswerMimeType(() => true)).toBe("audio/webm;codecs=opus");
  expect(pickAnswerMimeType(() => false)).toBe("");
});

describe("createAnswerRecorder", () => {
  it("records WebM/Opus at 32 kbps and hands over on Done", () => {
    const { recorder, recorded } = setup();
    expect(recorder.start(stream)).toBe(true);
    const [media] = FakeRecorder.instances;
    expect(media.options).toEqual({
      mimeType: "audio/webm;codecs=opus",
      audioBitsPerSecond: 32000,
    });
    expect(recorder.getSnapshot().status).toBe("recording");
    clock = 1000;
    vi.advanceTimersByTime(1000);
    expect(recorder.getSnapshot().elapsedMs).toBe(1000);
    recorder.finish();
    expect(recorded).toHaveLength(1);
    expect(recorded[0]).toMatchObject({
      mimeType: "audio/webm;codecs=opus",
      reason: "done",
    });
    expect(recorded[0].blob.size).toBe(3);
    expect(recorder.getSnapshot()).toEqual({
      status: "idle",
      elapsedMs: 0,
      error: null,
    });
    expect(track.stop).not.toHaveBeenCalled();
  });

  it("stops itself at 3:00 and keeps the recording", () => {
    const { recorder, recorded } = setup();
    recorder.start(stream);
    clock = 179_999;
    vi.advanceTimersByTime(179_999);
    expect(recorded).toEqual([]);
    expect(recorder.getSnapshot().elapsedMs).toBeLessThan(180_000);
    clock = 180_000;
    vi.advanceTimersByTime(1);
    expect(recorded).toHaveLength(1);
    expect(recorded[0].reason).toBe("time-limit");
    expect(FakeRecorder.instances[0].state).toBe("inactive");
    vi.advanceTimersByTime(10_000);
    expect(recorded).toHaveLength(1);
    expect(recorder.getSnapshot().status).toBe("idle");
  });

  it("cancel() discards the recording", () => {
    const { recorder, recorded } = setup();
    recorder.start(stream);
    recorder.cancel();
    expect(recorded).toEqual([]);
    expect(recorder.getSnapshot().status).toBe("idle");
    vi.advanceTimersByTime(200_000);
    expect(recorded).toEqual([]);
    expect(track.stop).not.toHaveBeenCalled();
  });

  it("records one answer at a time", () => {
    const { recorder } = setup();
    recorder.start(stream);
    expect(recorder.start(stream)).toBe(true);
    expect(FakeRecorder.instances).toHaveLength(1);
  });

  it("reports an unsupported browser", () => {
    vi.stubGlobal("MediaRecorder", undefined);
    const { recorder } = setup();
    expect(recorder.start(stream)).toBe(false);
    expect(recorder.getSnapshot().error).toBe("unsupported");
  });

  it("reports a recorder that fails, without handing anything over", () => {
    const { recorder, recorded } = setup();
    recorder.start(stream);
    FakeRecorder.instances[0].onerror?.();
    expect(recorder.getSnapshot()).toMatchObject({
      status: "idle",
      error: "failed",
    });
    expect(recorded).toEqual([]);
  });

  it("never sends a recording the browser stopped by itself (microphone stopped or lost)", () => {
    const { recorder, recorded } = setup();
    recorder.start(stream);
    // The track ended under the recorder: MediaRecorder stops on its own.
    FakeRecorder.instances[0].stop();
    expect(recorded).toEqual([]);
    expect(recorder.getSnapshot()).toMatchObject({
      status: "idle",
      error: "failed",
    });
    vi.advanceTimersByTime(180_000);
    expect(recorded).toEqual([]);
  });
});
