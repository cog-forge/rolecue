import type { Page } from "@playwright/test";

interface MicrophoneProbe {
  calls: MediaStreamConstraints[];
  streams: MediaStream[];
  contexts: AudioContext[];
  failure: string | null;
  synthetic: boolean;
  deferred: boolean;
  devices: string[];
  release: (() => void) | null;
  setLevel: (level: number) => void;
}
declare global {
  interface Window {
    microphoneProbe: MicrophoneProbe;
  }
}

/** Browser APIs are instrumented only by the test runner, never by production code. */
export async function installMicrophoneProbe(page: Page) {
  await page.addInitScript(() => {
    const NativeContext = window.AudioContext;
    const nativeCapture = navigator.mediaDevices.getUserMedia.bind(
      navigator.mediaDevices,
    );
    const nativeEnumerate = navigator.mediaDevices.enumerateDevices.bind(
      navigator.mediaDevices,
    );
    const gains: GainNode[] = [];
    const probe: MicrophoneProbe = {
      calls: [],
      streams: [],
      contexts: [],
      failure: null,
      synthetic: false,
      deferred: false,
      devices: ["mic-1", "mic-2"],
      release: null,
      setLevel(level) {
        gains.forEach((gain) => {
          gain.gain.value = level;
        });
      },
    };
    window.microphoneProbe = probe;
    window.AudioContext = class extends NativeContext {
      constructor(options?: AudioContextOptions) {
        super(options);
        probe.contexts.push(this);
      }
    };
    navigator.mediaDevices.enumerateDevices = async () =>
      probe.synthetic
        ? probe.devices.map((id) => ({
            deviceId: id,
            label: id === "mic-1" ? "Desk microphone" : "Headset microphone",
            kind: "audioinput",
            groupId: "test",
            toJSON: () => ({}),
          }))
        : nativeEnumerate();
    navigator.mediaDevices.getUserMedia = async (constraints) => {
      probe.calls.push(constraints ?? {});
      if (probe.failure)
        throw new DOMException(
          "Injected browser device failure",
          probe.failure,
        );
      if (probe.deferred)
        await new Promise<void>((resolve) => {
          probe.release = resolve;
        });
      let stream: MediaStream;
      if (probe.synthetic) {
        // Native Web Audio signal and native MediaStream tracks, with a controllable source.
        const context = new NativeContext();
        await context.resume();
        const oscillator = context.createOscillator();
        const gain = context.createGain();
        gain.gain.value = 0;
        gains.push(gain);
        const destination = context.createMediaStreamDestination();
        oscillator.connect(gain);
        gain.connect(destination);
        oscillator.start();
        stream = destination.stream;
        const audio = constraints?.audio;
        const device = typeof audio === "object" ? audio.deviceId : null;
        const selected =
          typeof device === "object" &&
          device !== null &&
          "exact" in device &&
          typeof device.exact === "string"
            ? device.exact
            : "mic-1";
        stream.getAudioTracks()[0].getSettings = () => ({ deviceId: selected });
      } else stream = await nativeCapture(constraints);
      probe.streams.push(stream);
      return stream;
    };
  });
}

export async function candidateSession(page: Page) {
  await page.route("**/auth/me", (route) =>
    route.fulfill({
      contentType: "application/json",
      headers: {
        "access-control-allow-origin": "http://127.0.0.1:3100",
        "access-control-allow-credentials": "true",
      },
      body: JSON.stringify({
        success: true,
        data: {
          id: "9b9e9289-994d-4e62-9e65-034b9a907330",
          email: "audio@example.com",
          full_name: "Audio Tester",
          role: "candidate",
          email_verified: true,
          is_locked: false,
          onboarding_role_selected: true,
          onboarding_completed: true,
        },
      }),
    }),
  );
}
