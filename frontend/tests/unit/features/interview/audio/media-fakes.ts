import { vi } from "vitest";

export function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

export function installMediaFakes() {
  let deviceId = "mic-1";
  class Track extends EventTarget {
    kind = "audio";
    label = "Desk microphone";
    enabled = true;
    muted = false;
    readyState = "live";
    id = deviceId;
    stop = vi.fn(() => {
      this.readyState = "ended";
    });
    getSettings() {
      return { deviceId: this.id };
    }
    end() {
      this.readyState = "ended";
      this.dispatchEvent(new Event("ended"));
    }
    interrupt(muted: boolean) {
      this.muted = muted;
      this.dispatchEvent(new Event(muted ? "mute" : "unmute"));
    }
  }
  const tracks: Track[] = [];
  class Stream {
    track = new Track();
    constructor() {
      tracks.push(this.track);
    }
    getAudioTracks() {
      return [this.track];
    }
    getTracks() {
      return [this.track];
    }
  }
  const media = Object.assign(new EventTarget(), {
    getUserMedia: vi
      .fn<MediaDevices["getUserMedia"]>()
      .mockImplementation(async () => new MediaStream()),
    enumerateDevices: vi
      .fn<MediaDevices["enumerateDevices"]>()
      .mockResolvedValue([device("mic-1"), device("mic-2")]),
  });
  vi.stubGlobal("MediaStream", Stream);
  vi.stubGlobal("isSecureContext", true);
  vi.stubGlobal("AudioContext", undefined);
  Object.defineProperty(navigator, "mediaDevices", {
    configurable: true,
    value: media,
  });
  return {
    media,
    tracks,
    stream(id = "mic-1") {
      deviceId = id;
      return new MediaStream();
    },
  };
}

export function device(id: string, label = id): MediaDeviceInfo {
  return {
    deviceId: id,
    groupId: "group",
    kind: "audioinput",
    label,
    toJSON: () => ({}),
  };
}
