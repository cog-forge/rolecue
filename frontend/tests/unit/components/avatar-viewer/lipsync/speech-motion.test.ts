import { describe, expect, it } from "vitest";
import fixture from "@tests/fixtures/interviewer-turn-timelines.json";
import {
  planSpeechMotion,
  sampleSpeechMotion,
} from "@/components/avatar-viewer/lipsync/speech-motion";

describe("planSpeechMotion", () => {
  const plan = planSpeechMotion(fixture.words, fixture.text);
  const wordAt = (ms: number) =>
    fixture.words.find((word) => word.offset_ms === ms)?.text;

  it("nods on stressed content words, spaced out, never on function words", () => {
    const nodded = plan.nods.map((nod) => wordAt(nod.start + 60));
    expect(nodded).toEqual([
      "trade-offs",
      "CSS-in-JS",
      "versus",
      "modules",
      "Design",
    ]);
    for (let i = 1; i < plan.nods.length; i++)
      expect(
        plan.nods[i].start - plan.nods[i - 1].start,
      ).toBeGreaterThanOrEqual(900);
  });
  it("glances aside as the sentence starts and lifts the brows on the question's last word", () => {
    expect(plan.glances).toHaveLength(1);
    expect(plan.glances[0].start).toBe(50);
    expect(plan.browRaises).toHaveLength(1);
    expect(plan.browRaises[0].start).toBe(4888 - 200);
  });
  it("finds every sentence of a multi-sentence turn and only raises brows on questions", () => {
    const words = ["Thanks", "for", "that", "Now", "why", "Go"].map(
      (text, i) => ({ offset_ms: i * 400, duration_ms: 300, text }),
    );
    const multi = planSpeechMotion(words, "Thanks for that. Now, why Go?");
    expect(multi.glances.map((glance) => glance.start)).toEqual([0, 1200]);
    expect(multi.glances.map((glance) => glance.side)).toEqual([1, -1]);
    expect(multi.browRaises.map((raise) => raise.start)).toEqual([2000 - 200]);
  });
  it("still plans one sentence when the text and words disagree", () => {
    const words = [{ offset_ms: 0, duration_ms: 400, text: "three" }];
    expect(planSpeechMotion(words, "3?").browRaises).toHaveLength(1);
    expect(planSpeechMotion([], "Hello?")).toEqual({
      nods: [],
      glances: [],
      browRaises: [],
    });
  });
});

describe("sampleSpeechMotion", () => {
  const plan = planSpeechMotion(fixture.words, fixture.text);

  it("stays subtle and returns to neutral between gestures", () => {
    let maxPitch = 0;
    let maxYaw = 0;
    for (let t = 0; t < 6000; t += 5) {
      const frame = sampleSpeechMotion(plan, t);
      maxPitch = Math.max(maxPitch, Math.abs(frame.pitch));
      maxYaw = Math.max(maxYaw, Math.abs(frame.yaw));
      expect(frame.brow).toBeLessThanOrEqual(0.4);
    }
    expect(maxPitch).toBeGreaterThan(0.02);
    expect(maxPitch).toBeLessThan(0.06);
    expect(maxYaw).toBeLessThan(0.04);
    expect(sampleSpeechMotion(plan, 6000)).toEqual({
      pitch: 0,
      roll: 0,
      yaw: 0,
      gazeX: 0,
      gazeY: 0,
      brow: 0,
    });
  });
  it("lifts the brows at the end of the question and scales only motion for reduced motion", () => {
    const end = sampleSpeechMotion(plan, 5100);
    expect(end.brow).toBeCloseTo(0.4);
    const nod = plan.nods[0];
    const peak = (nod.start + nod.end) / 2;
    const full = sampleSpeechMotion(plan, peak);
    const reduced = sampleSpeechMotion(plan, peak, 0.3);
    expect(reduced.pitch).toBeCloseTo(full.pitch * 0.3);
    expect(sampleSpeechMotion(plan, 5100, 0.3).brow).toBeCloseTo(end.brow);
  });
});
