export type VoiceActivity = "inactive" | "silent" | "speaking" | "unavailable";
export type ActivitySample = { activity: "silent" | "speaking"; level: number };

export interface VoiceActivityOptions {
  onset: number;
  offset: number;
  attackMs: number;
  releaseMs: number;
  maxGapMs: number;
}

/** Energy heuristic for UX only; never a speech recognizer or turn-end authority. */
export function createVoiceActivityDetector(
  options: Partial<VoiceActivityOptions> = {},
) {
  const config = {
    onset: 0.025,
    offset: 0.012,
    attackMs: 120,
    releaseMs: 450,
    maxGapMs: 500,
    ...options,
  };
  if (
    Object.values(config).some(
      (value) => !Number.isFinite(value) || value < 0,
    ) ||
    config.onset <= config.offset ||
    config.onset > 1 ||
    config.maxGapMs === 0
  ) {
    throw new RangeError("Invalid voice activity configuration");
  }
  let speaking = false;
  let candidateSince: number | null = null;
  let previousTime: number | null = null;
  function reset() {
    speaking = false;
    candidateSince = null;
    previousTime = null;
  }
  return {
    reset,
    sample(samples: Float32Array, now: number): ActivitySample {
      if (
        previousTime !== null &&
        (now - previousTime > config.maxGapMs || now < previousTime)
      )
        reset();
      previousTime = now;
      let sum = 0;
      for (const value of samples)
        sum += Number.isFinite(value) ? value * value : 0;
      const rms = samples.length ? Math.sqrt(sum / samples.length) : 0;
      const changing = speaking ? rms < config.offset : rms >= config.onset;
      if (changing) {
        candidateSince ??= now;
        if (
          now - candidateSince >=
          (speaking ? config.releaseMs : config.attackMs)
        ) {
          speaking = !speaking;
          candidateSince = null;
        }
      } else candidateSince = null;
      return {
        activity: speaking ? "speaking" : "silent",
        level: Math.round(Math.min(1, rms * 4) * 100) / 100,
      };
    },
  };
}
