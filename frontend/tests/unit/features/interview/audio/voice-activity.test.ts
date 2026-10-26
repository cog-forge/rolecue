import { describe, expect, it } from "vitest";
import { createVoiceActivityDetector } from "@/features/interview/audio/voice-activity";

const signal = (amplitude: number) => new Float32Array(128).fill(amplitude);

describe("energy voice activity", () => {
  it("keeps silence silent and bounds the input level", () => {
    const detector = createVoiceActivityDetector();
    expect(detector.sample(signal(0), 0)).toEqual({
      activity: "silent",
      level: 0,
    });
    expect(detector.sample(signal(10), 50).level).toBe(1);
    expect(detector.sample(signal(Number.NaN), 100).level).toBe(0);
  });

  it("ignores a short burst and requires sustained input", () => {
    const detector = createVoiceActivityDetector({ attackMs: 100 });
    expect(detector.sample(signal(0.1), 0).activity).toBe("silent");
    expect(detector.sample(signal(0), 50).activity).toBe("silent");
    expect(detector.sample(signal(0.1), 100).activity).toBe("silent");
    expect(detector.sample(signal(0.1), 200).activity).toBe("speaking");
  });

  it("uses separate onset/offset thresholds and waits for continuous silence", () => {
    const detector = createVoiceActivityDetector({
      attackMs: 100,
      releaseMs: 300,
    });
    detector.sample(signal(0.1), 0);
    expect(detector.sample(signal(0.1), 100).activity).toBe("speaking");
    expect(detector.sample(signal(0.015), 200).activity).toBe("speaking");
    detector.sample(signal(0), 250);
    expect(detector.sample(signal(0), 500).activity).toBe("speaking");
    expect(detector.sample(signal(0), 550).activity).toBe("silent");
  });

  it("resets after missing samples or explicit reset instead of inventing speech end", () => {
    const detector = createVoiceActivityDetector({
      attackMs: 100,
      maxGapMs: 250,
    });
    detector.sample(signal(0.1), 0);
    detector.sample(signal(0.1), 100);
    expect(detector.sample(signal(0.1), 1000).activity).toBe("silent");
    expect(detector.sample(signal(0.1), 1100).activity).toBe("speaking");
    detector.reset();
    expect(detector.sample(signal(0.1), 1200).activity).toBe("silent");
  });

  it("rejects invalid timing and threshold configuration", () => {
    expect(() => createVoiceActivityDetector({ attackMs: -1 })).toThrow();
    expect(() =>
      createVoiceActivityDetector({ onset: 0.01, offset: 0.02 }),
    ).toThrow();
  });
});
