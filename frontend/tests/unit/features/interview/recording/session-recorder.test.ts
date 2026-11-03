import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createMemoryRecordingStore } from "@/features/interview/recording/recording-buffer";
import { createSessionRecorder } from "@/features/interview/recording/session-recorder";
import {
  asStream,
  FakeAvStream,
  FakeMediaRecorder,
  namedError,
} from "./recording-fakes";

let clock = 1_000;
function setup(
  getUserMedia = vi.fn(async () => asStream(new FakeAvStream())),
  store = createMemoryRecordingStore(),
) {
  const buffer = store.open("candidate-1", "interview-a");
  const recorder = createSessionRecorder({
    buffer,
    getUserMedia,
    createRecorder: (stream, options) =>
      new FakeMediaRecorder(stream, options) as unknown as MediaRecorder,
    isTypeSupported: (type) => FakeMediaRecorder.isTypeSupported(type),
    secure: () => true,
    now: () => clock,
  });
  return { recorder, buffer, store, getUserMedia };
}
beforeEach(() => {
  clock = 1_000;
  FakeMediaRecorder.reset();
});
afterEach(() => vi.restoreAllMocks());

describe("starting", () => {
  it("records camera and microphone together at the modest bitrate, in 5 s slices", async () => {
    const { recorder, getUserMedia } = setup();
    await recorder.start();
    expect(getUserMedia).toHaveBeenCalledExactlyOnceWith({
      video: expect.objectContaining({ width: { ideal: 640 } }),
      audio: { echoCancellation: true, noiseSuppression: true },
    });
    const media = FakeMediaRecorder.latest();
    expect(media.options).toEqual({
      mimeType: "video/webm;codecs=vp8,opus",
      videoBitsPerSecond: 500_000,
      audioBitsPerSecond: 48_000,
    });
    expect(media.timeslice).toBe(5_000);
    expect(recorder.getSnapshot()).toMatchObject({
      status: "recording",
      started: true,
      resumed: false,
      error: null,
    });
    expect(recorder.getSnapshot().stream).toBe(media.stream);
  });

  it.each([
    ["NotAllowedError", "permission-denied"],
    ["SecurityError", "permission-denied"],
    ["NotFoundError", "no-device"],
    ["NotReadableError", "device-unavailable"],
    ["TypeError", "failed"],
  ])(
    "reports a %s camera request as %s and records nothing",
    async (name, code) => {
      const { recorder } = setup(vi.fn().mockRejectedValue(namedError(name)));
      await recorder.start();
      expect(recorder.getSnapshot()).toMatchObject({
        status: "error",
        error: code,
        started: false,
      });
      expect(FakeMediaRecorder.instances).toHaveLength(0);
    },
  );

  it("treats a stream without a camera track as no device and releases it", async () => {
    const stream = new FakeAvStream(null);
    const { recorder } = setup(vi.fn(async () => asStream(stream)));
    await recorder.start();
    expect(recorder.getSnapshot()).toMatchObject({
      status: "error",
      error: "no-device",
    });
    expect(stream.audio?.stop).toHaveBeenCalled();
  });

  it("refuses a browser that cannot record video before asking for the camera", async () => {
    FakeMediaRecorder.supported = new Set();
    const { recorder, getUserMedia } = setup();
    await recorder.start();
    expect(recorder.getSnapshot().error).toBe("unsupported");
    expect(getUserMedia).not.toHaveBeenCalled();
  });

  it("recovers after the Candidate allows the camera", async () => {
    const getUserMedia = vi
      .fn()
      .mockRejectedValueOnce(namedError("NotAllowedError"))
      .mockImplementation(async () => asStream(new FakeAvStream()));
    const { recorder } = setup(getUserMedia);
    await recorder.start();
    expect(recorder.getSnapshot().status).toBe("error");
    await recorder.start();
    expect(recorder.getSnapshot()).toMatchObject({
      status: "recording",
      error: null,
    });
  });
});

describe("recording", () => {
  it("saves every slice as it arrives and the last one on stop", async () => {
    const { recorder, buffer } = setup();
    await recorder.start();
    const media = FakeMediaRecorder.latest();
    media.emit([1, 2]);
    media.emit([3]);
    expect(recorder.getSnapshot().bytes).toBe(3);
    await recorder.stop();
    const [segment] = await buffer.load();
    expect(segment.mimeType).toBe("video/webm;codecs=vp8,opus");
    expect(segment.bytes).toBe(4);
    expect(recorder.getSnapshot().status).toBe("stopped");
    expect(
      (media.stream as unknown as FakeAvStream).video?.stop,
    ).toHaveBeenCalled();
  });

  it("continues a reloaded interview as a new segment of the same recording", async () => {
    const store = createMemoryRecordingStore();
    const before = setup(undefined, store);
    await before.recorder.start();
    FakeMediaRecorder.latest().emit([1, 2, 3]);
    // The page reloads: the recorder is dropped, the slices it saved stay (the fake's last slice
    // adds one byte).
    before.recorder.dispose();
    await vi.waitFor(async () =>
      expect((await before.buffer.load())[0]?.bytes).toBe(4),
    );
    clock = 500; // an earlier wall clock still sorts after the saved segment

    const after = setup(undefined, store);
    await after.recorder.start();
    expect(after.recorder.getSnapshot()).toMatchObject({
      resumed: true,
      bytes: 4,
    });
    FakeMediaRecorder.latest().emit([4]);
    await after.recorder.stop();
    const segments = await after.buffer.load();
    expect(segments).toHaveLength(2);
    expect(segments[1].segment).toBeGreaterThan(segments[0].segment);
    expect(segments.map((segment) => segment.bytes)).toEqual([4, 2]);
  });

  it("keeps what was recorded when the camera is unplugged, and can start again", async () => {
    const { recorder, buffer } = setup();
    await recorder.start();
    const media = FakeMediaRecorder.latest();
    media.emit([1]);
    (media.stream as unknown as FakeAvStream).video?.unplug();
    expect(recorder.getSnapshot()).toMatchObject({
      status: "error",
      error: "device-lost",
      started: true,
      stream: null,
    });
    await recorder.start();
    expect(recorder.getSnapshot().status).toBe("recording");
    await recorder.stop();
    expect((await buffer.load()).length).toBe(2);
  });

  it("follows the room's microphone mute on the recorded track", async () => {
    const { recorder } = setup();
    recorder.setAudioEnabled(false);
    await recorder.start();
    const stream = FakeMediaRecorder.latest().stream as unknown as FakeAvStream;
    expect(stream.audio?.enabled).toBe(false);
    recorder.setAudioEnabled(true);
    expect(stream.audio?.enabled).toBe(true);
    expect(stream.video?.enabled).toBe(true);
  });
});
