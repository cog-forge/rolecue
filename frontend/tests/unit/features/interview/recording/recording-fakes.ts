import { vi } from "vitest";

/** A camera or microphone track that can be unplugged. */
export class FakeTrack extends EventTarget {
  enabled = true;
  readyState: MediaStreamTrackState = "live";
  constructor(readonly kind: "audio" | "video") {
    super();
  }
  stop = vi.fn(() => {
    this.readyState = "ended";
  });
  unplug() {
    this.readyState = "ended";
    this.dispatchEvent(new Event("ended"));
  }
}

export class FakeAvStream {
  constructor(
    readonly video: FakeTrack | null = new FakeTrack("video"),
    readonly audio: FakeTrack | null = new FakeTrack("audio"),
  ) {}
  getTracks() {
    return [this.video, this.audio].filter(
      (track): track is FakeTrack => track !== null,
    );
  }
  getVideoTracks() {
    return this.video ? [this.video] : [];
  }
  getAudioTracks() {
    return this.audio ? [this.audio] : [];
  }
}
export const asStream = (stream: FakeAvStream) =>
  stream as unknown as MediaStream;

/** A MediaRecorder whose slices the test emits. */
export class FakeMediaRecorder {
  static instances: FakeMediaRecorder[] = [];
  static supported = new Set<string>(["video/webm;codecs=vp8,opus"]);
  static isTypeSupported(type: string) {
    return FakeMediaRecorder.supported.has(type);
  }
  state: RecordingState = "inactive";
  mimeType: string;
  timeslice: number | undefined;
  ondataavailable: ((event: BlobEvent) => void) | null = null;
  onstop: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  constructor(
    readonly stream: MediaStream,
    readonly options: MediaRecorderOptions,
  ) {
    this.mimeType = options.mimeType ?? "video/webm";
    FakeMediaRecorder.instances.push(this);
  }
  start(timeslice?: number) {
    this.state = "recording";
    this.timeslice = timeslice;
  }
  /** Delivers one slice, as the browser does every timeslice. */
  emit(bytes: number[] | Uint8Array) {
    const data = new Blob([new Uint8Array(bytes)], { type: this.mimeType });
    this.ondataavailable?.({ data } as BlobEvent);
  }
  stop() {
    this.state = "inactive";
    this.emit([0xff]);
    this.onstop?.(new Event("stop"));
  }
  static latest() {
    const recorder = FakeMediaRecorder.instances.at(-1);
    if (!recorder) throw new Error("no MediaRecorder was created");
    return recorder;
  }
  static reset() {
    FakeMediaRecorder.instances = [];
    FakeMediaRecorder.supported = new Set(["video/webm;codecs=vp8,opus"]);
  }
}

export function namedError(name: string) {
  return Object.assign(new Error(name), { name });
}

/** getUserMedia and MediaRecorder for a component test. */
export function installRecordingFakes() {
  FakeMediaRecorder.reset();
  const streams: FakeAvStream[] = [];
  const getUserMedia = vi
    .fn<MediaDevices["getUserMedia"]>()
    .mockImplementation(async () => {
      const stream = new FakeAvStream();
      streams.push(stream);
      return asStream(stream);
    });
  vi.stubGlobal("MediaRecorder", FakeMediaRecorder);
  vi.stubGlobal("isSecureContext", true);
  Object.defineProperty(navigator, "mediaDevices", {
    configurable: true,
    value: Object.assign(new EventTarget(), {
      getUserMedia,
      enumerateDevices: vi
        .fn<MediaDevices["enumerateDevices"]>()
        .mockResolvedValue([]),
    }),
  });
  return { getUserMedia, streams };
}
