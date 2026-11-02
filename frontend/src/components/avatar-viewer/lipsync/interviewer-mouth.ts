import type { MouthSample, MouthSource } from "../avatar-director";
import {
  createAmplitudeDriver,
  createWLipSyncDriver,
  type AnalysisState,
  type AudioDriver,
} from "./audio-analysis";
import { MouthSmoother } from "./smoother";
import { createVisemeTrack, sampleBlendShapes } from "./timeline-drivers";
import { normalizeTimelines, type InterviewerTimelines } from "./timelines";

/** Fallback order from the Brain (Avatar-Voice, Interviewer Performance). */
export const mouthSourceKinds = [
  "blend-shapes",
  "visemes",
  "wlipsync",
  "amplitude",
] as const;
export type MouthSourceKind = (typeof mouthSourceKinds)[number];

export type InterviewerMouth = MouthSource & {
  dispose: () => void;
  /** The source in use and, for audio analysis, whether it is running yet. */
  status: () => { source: MouthSourceKind; state: AnalysisState };
};

/** Server timelines when present, otherwise analysis of the playing audio. */
export function pickMouthSource(
  timelines: InterviewerTimelines,
): MouthSourceKind {
  if (timelines.blend_shapes.length > 0) return "blend-shapes";
  if (timelines.visemes.length > 0) return "visemes";
  return "wlipsync";
}

// Seconds. Azure frames are already smooth; the other sources need progressively more damping.
const smoothing: Record<MouthSourceKind, [attack: number, release: number]> = {
  "blend-shapes": [0, 0],
  visemes: [0.02, 0.04],
  wlipsync: [0.02, 0.033],
  amplitude: [0.03, 0.08],
};
const maxFrameSeconds = 0.1;
// A jump this large between two samples is a seek or a stall, not playback.
const seekThresholdMs = 250;

type Driver = {
  target: (timeMs: number, dt: number) => MouthSample | null;
  state: () => AnalysisState;
  dispose: () => void;
};

function timelineDriver(sample: (timeMs: number) => MouthSample): Driver {
  return { target: sample, state: () => "ready", dispose: () => {} };
}

function analysisDriver(driver: AudioDriver): Driver {
  return {
    target: (_, dt) => driver.target(dt),
    state: () => driver.state,
    dispose: driver.dispose,
  };
}

function createDriver(
  kind: MouthSourceKind,
  audio: HTMLMediaElement,
  timelines: InterviewerTimelines,
): Driver {
  switch (kind) {
    case "blend-shapes":
      return timelineDriver((time) =>
        sampleBlendShapes(timelines.blend_shapes, time),
      );
    case "visemes":
      return timelineDriver(createVisemeTrack(timelines.visemes));
    case "wlipsync":
      return analysisDriver(createWLipSyncDriver(audio));
    case "amplitude":
      return analysisDriver(createAmplitudeDriver(audio));
  }
}

/**
 * Mouth weights for one interviewer turn, driven by `audio.currentTime`. Returns null while the element
 * is paused or ended (or another source was loaded into it), so the director releases the mouth.
 * Create one per turn and dispose it when the turn is replaced, interrupted or unmounted.
 */
export function createInterviewerMouth(
  audio: HTMLMediaElement,
  timelines: InterviewerTimelines,
): MouthSource & { dispose(): void } {
  return createMouthForSource(audio, timelines, "auto");
}

/**
 * A mouth for one source, or `auto` to follow the fallback order: the best timeline present, else
 * wLipSync, degrading to amplitude if wLipSync cannot load. Fixed sources are for comparing them.
 */
export function createMouthForSource(
  audio: HTMLMediaElement,
  timelines: InterviewerTimelines,
  kind: MouthSourceKind | "auto",
): InterviewerMouth {
  const normalized = normalizeTimelines(timelines);
  const fallback = kind === "auto";
  let source = kind === "auto" ? pickMouthSource(normalized) : kind;
  let driver = createDriver(source, audio, normalized);
  let smoother = new MouthSmoother(...smoothing[source]);
  let disposed = false;
  let boundSource: string | undefined;
  let lastMediaMs = Number.NaN;
  let lastSampleAt = Number.NaN;
  let lastMediaChangeAt = Number.NaN;
  let lastTimeMs = Number.NaN;

  const rest = () => {
    smoother.reset();
    lastTimeMs = Number.NaN;
    lastSampleAt = Number.NaN;
    lastMediaMs = Number.NaN;
  };
  // Some browsers advance currentTime in coarse steps; extrapolate briefly between them.
  const clock = (now: number) => {
    const mediaMs = audio.currentTime * 1000;
    if (mediaMs !== lastMediaMs) {
      lastMediaMs = mediaMs;
      lastMediaChangeAt = now;
      return mediaMs;
    }
    const ahead = Math.min(now - lastMediaChangeAt, maxFrameSeconds * 1000);
    return mediaMs + Math.max(0, ahead) * (audio.playbackRate || 1);
  };

  return {
    sample() {
      if (disposed || audio.paused || audio.ended) {
        rest();
        return null;
      }
      // A new turn loaded into the same element must not replay this turn's timeline.
      const current = audio.currentSrc || audio.src;
      boundSource ??= current;
      if (current !== boundSource) {
        rest();
        return null;
      }
      if (fallback && source === "wlipsync" && driver.state() === "failed") {
        driver.dispose();
        source = "amplitude";
        driver = createDriver(source, audio, normalized);
        smoother = new MouthSmoother(...smoothing[source]);
      }
      const now = performance.now();
      const time = clock(now);
      const dt = Number.isNaN(lastSampleAt)
        ? 0
        : Math.min(Math.max((now - lastSampleAt) / 1000, 0), maxFrameSeconds);
      lastSampleAt = now;
      if (
        !Number.isNaN(lastTimeMs) &&
        (time < lastTimeMs - 1 || time - lastTimeMs > seekThresholdMs)
      )
        smoother.reset();
      lastTimeMs = time;
      const target = driver.target(time, dt);
      if (!target) return null;
      // The first frame after (re)start lands on the target instead of ramping up from rest.
      return smoother.step(target, dt === 0 ? maxFrameSeconds : dt);
    },
    status: () => ({ source, state: driver.state() }),
    dispose() {
      if (disposed) return;
      disposed = true;
      driver.dispose();
      smoother.reset();
    },
  };
}
