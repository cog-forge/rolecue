import {
  createVoiceActivityDetector,
  type VoiceActivity,
  type VoiceActivityOptions,
} from "./voice-activity";

export interface AudioMonitorSample {
  activity: VoiceActivity;
  level: number;
  warning: string | null;
}

/** Owns only analysis resources, never the borrowed stream's tracks. */
export function createAudioMonitor(
  stream: MediaStream,
  onSample: (sample: AudioMonitorSample) => void,
  options?: Partial<VoiceActivityOptions>,
) {
  const detector = createVoiceActivityDetector(options);
  let context: AudioContext | null = null;
  let source: MediaStreamAudioSourceNode | null = null;
  let analyser: AnalyserNode | null = null;
  let timer: ReturnType<typeof setInterval> | undefined;
  let disposed = false;
  let enabled = true;
  const unavailable = () =>
    onSample({
      activity: "unavailable",
      level: 0,
      warning:
        "Input level and voice activity are unavailable. Retry the audio check to enable them.",
    });
  const stateChanged = () => {
    detector.reset();
    if (!disposed && enabled && context?.state !== "running") unavailable();
  };
  function release() {
    clearInterval(timer);
    context?.removeEventListener("statechange", stateChanged);
    source?.disconnect();
    analyser?.disconnect();
    if (context && context.state !== "closed")
      void context.close().catch(() => {});
    source = null;
    analyser = null;
    context = null;
  }
  function resume() {
    detector.reset();
    if (!context) return;
    void context.resume().catch(() => {
      if (!disposed && enabled) unavailable();
    });
  }
  try {
    context = new AudioContext();
    source = context.createMediaStreamSource(stream);
    analyser = context.createAnalyser();
    analyser.fftSize = 1024;
    source.connect(analyser); // No connection to destination: never play the mic through speakers.
    const samples = new Float32Array(analyser.fftSize);
    context.addEventListener("statechange", stateChanged);
    timer = setInterval(() => {
      if (disposed || !enabled) return;
      if (context?.state !== "running") {
        detector.reset();
        unavailable();
        return;
      }
      try {
        analyser?.getFloatTimeDomainData(samples);
        onSample({
          ...detector.sample(samples, performance.now()),
          warning: null,
        });
      } catch {
        release();
        unavailable();
      }
    }, 50);
    stateChanged();
    resume();
  } catch {
    release();
    unavailable();
  }
  return {
    resume,
    setEnabled(value: boolean) {
      if (enabled === value) return;
      enabled = value;
      stateChanged();
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      release();
    },
  };
}
