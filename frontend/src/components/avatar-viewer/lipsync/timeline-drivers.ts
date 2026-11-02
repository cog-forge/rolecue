import type { MouthSample } from "../avatar-director";
import {
  azureBlendShapeChannels,
  azureVisemeToOculus,
  visemeExpression,
  type OculusViseme,
} from "./azure";
import { lastAtOrBefore, type InterviewerTimelines } from "./timelines";

const frameMs = 1000 / 60;

/**
 * Weights at `time` (ms), linearly interpolated between the two surrounding 60 FPS frames. Outside the
 * timeline the mouth is at rest (empty sample).
 */
export function sampleBlendShapes(
  frames: InterviewerTimelines["blend_shapes"],
  time: number,
): MouthSample {
  const index = lastAtOrBefore(frames, time);
  if (index < 0) return {};
  const current = frames[index];
  const next = frames[index + 1];
  if (!next && time - current.offset_ms > frameMs * 2) return {};
  const span = next ? next.offset_ms - current.offset_ms : 0;
  const u = next && span > 0 ? (time - current.offset_ms) / span : 0;
  const out: Record<string, number> = {};
  for (const { index: channel, name, gain } of azureBlendShapeChannels) {
    const a = current.values[channel] ?? 0;
    const b = next ? (next.values[channel] ?? 0) : a;
    const value = (a + (b - a) * u) * gain;
    if (Number.isFinite(value) && value > 0.001) out[name] = value;
  }
  return out;
}

type Envelope = {
  offset_ms: number;
  name: string;
  start: number;
  peakAt: number;
  peak: number;
  end: number;
};

// Lips must fully meet on bilabials and touch the teeth on labiodentals; other shapes stay softer.
const peaks: Partial<Record<OculusViseme, number>> = { PP: 1, FF: 0.9 };
const defaultPeak = 0.75;
const minimumDuration = 40;
const finalDuration = 120;
const smoothstep = (u: number) => u * u * (3 - 2 * u);

/**
 * Viseme IDs with coarticulation, after TalkingHead (met4citizen, MIT): each viseme ramps in up to 60 ms
 * before its offset, peaks shortly after it and releases into the next one, so neighbours overlap.
 */
export function createVisemeTrack(visemes: InterviewerTimelines["visemes"]) {
  const envelopes: Envelope[] = [];
  visemes.forEach((viseme, i) => {
    const oculus = azureVisemeToOculus[viseme.id];
    if (!oculus || oculus === "sil") return;
    const next = visemes[i + 1];
    const duration = Math.max(
      minimumDuration,
      next ? next.offset_ms - viseme.offset_ms : finalDuration,
    );
    const t = viseme.offset_ms;
    envelopes.push({
      offset_ms: t,
      name: visemeExpression(oculus),
      start: t - Math.min(60, (duration * 2) / 3),
      peakAt: t + Math.min(25, duration / 2),
      peak: peaks[oculus] ?? defaultPeak,
      end: t + duration + Math.min(60, duration / 2),
    });
  });
  // Offsets are sorted and an envelope starts at most 60 ms early and ends at most 60 ms after the next
  // viseme, so only a few envelopes around `time` can be active.
  const lookBack = 8;
  return (time: number): MouthSample => {
    const last = lastAtOrBefore(envelopes, time + 60);
    const out: Record<string, number> = {};
    for (let i = last; i >= 0 && i > last - lookBack; i--) {
      const envelope = envelopes[i];
      if (time < envelope.start || time >= envelope.end) continue;
      const value =
        time < envelope.peakAt
          ? smoothstep(
              (time - envelope.start) / (envelope.peakAt - envelope.start),
            ) * envelope.peak
          : smoothstep(
              1 - (time - envelope.peakAt) / (envelope.end - envelope.peakAt),
            ) * envelope.peak;
      // The same viseme twice in a row must not add up past its peak.
      if (value > (out[envelope.name] ?? 0)) out[envelope.name] = value;
    }
    return out;
  };
}
