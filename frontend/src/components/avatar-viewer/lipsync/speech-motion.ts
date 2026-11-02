import type { InterviewerTimelines } from "./timelines";

type Words = InterviewerTimelines["words"];
type Pulse = { start: number; end: number; amount: number };

/** Precomputed from the word timeline once per turn; sampled per frame with the audio clock. */
export type SpeechMotionPlan = {
  /** Small head dips on stressed words; alternate ones also tilt. */
  nods: (Pulse & { tilt: number })[];
  /** A brief look aside as a sentence starts, alternating sides. */
  glances: (Pulse & { side: number })[];
  /** Brows lift over the last word of a question. */
  browRaises: Pulse[];
};

export type SpeechMotionFrame = {
  /** Radians added to the head bone. */
  pitch: number;
  roll: number;
  yaw: number;
  /** Metres added to the gaze target (3 m in front of the head). */
  gazeX: number;
  gazeY: number;
  /** Brow-raise weight 0..1. */
  brow: number;
};

// Function words carry no stress in a question; nodding on them looks mechanical.
const unstressed = new Set(
  "a an and are as at be but by can could did do does for from had has have how i if in into is it its me my of on or our so than that the their them then there these they this to us was we were what when where which while who why will with would you your".split(
    " ",
  ),
);
const normalize = (text: string) =>
  text.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, "");

/** Index into `words` of each sentence's first and last word, matched by text in order. */
function sentenceSpans(words: Words, text: string) {
  const sentences = text
    .split(/(?<=[.!?])\s+/)
    .map((sentence) => ({
      tokens: sentence.split(/\s+/).map(normalize).filter(Boolean),
      question: sentence.trim().endsWith("?"),
    }))
    .filter((sentence) => sentence.tokens.length > 0);
  const spans: { first: number; last: number; question: boolean }[] = [];
  let cursor = 0;
  for (const sentence of sentences) {
    let first = -1;
    let last = -1;
    for (const token of sentence.tokens) {
      // Azure may split or spell out a token differently; look a couple of words ahead, else skip it.
      for (let i = cursor; i < Math.min(words.length, cursor + 3); i++)
        if (normalize(words[i].text) === token) {
          if (first < 0) first = i;
          last = i;
          cursor = i + 1;
          break;
        }
    }
    if (first >= 0) spans.push({ first, last, question: sentence.question });
  }
  // Text that never matches still gets one sentence covering every word.
  if (spans.length === 0 && words.length > 0)
    spans.push({
      first: 0,
      last: words.length - 1,
      question: text.trim().endsWith("?"),
    });
  return spans;
}

const nodGapMs = 900;
const nodLengthMs = 380;

export function planSpeechMotion(words: Words, text: string): SpeechMotionPlan {
  const plan: SpeechMotionPlan = { nods: [], glances: [], browRaises: [] };
  const spans = sentenceSpans(words, text);
  const sentenceStarts = new Set(spans.map((span) => span.first));
  spans.forEach((span, index) => {
    const first = words[span.first];
    plan.glances.push({
      start: first.offset_ms,
      end: first.offset_ms + 900,
      amount: 1,
      side: index % 2 === 0 ? 1 : -1,
    });
    if (span.question) {
      const last = words[span.last];
      plan.browRaises.push({
        start: last.offset_ms - 200,
        end: last.offset_ms + last.duration_ms + 350,
        amount: 1,
      });
    }
  });
  let lastNod = Number.NEGATIVE_INFINITY;
  words.forEach((word, index) => {
    const token = normalize(word.text);
    if (!token || unstressed.has(token) || sentenceStarts.has(index)) return;
    if (word.duration_ms < 280 && token.length < 6) return;
    if (word.offset_ms - lastNod < nodGapMs) return;
    lastNod = word.offset_ms;
    plan.nods.push({
      start: word.offset_ms - 60,
      end: word.offset_ms - 60 + nodLengthMs,
      // Longer words carry more stress, within a small range.
      amount: Math.min(1, 0.6 + word.duration_ms / 1500),
      tilt:
        plan.nods.length % 2 === 0 ? 0 : plan.nods.length % 4 === 1 ? 1 : -1,
    });
  });
  return plan;
}

/** 0 → 1 → 0 over [start, end] with soft ends. */
const bump = (time: number, { start, end }: Pulse) => {
  if (time <= start || time >= end) return 0;
  const s = Math.sin((Math.PI * (time - start)) / (end - start));
  return s * s;
};
/** Rises over `rise` ms, holds, and falls over `fall` ms at the end of [start, end]. */
const plateau = (time: number, pulse: Pulse, rise: number, fall: number) => {
  if (time <= pulse.start || time >= pulse.end) return 0;
  const up = Math.min(1, (time - pulse.start) / rise);
  const down = Math.min(1, (pulse.end - time) / fall);
  const u = Math.min(up, down);
  return u * u * (3 - 2 * u);
};

const nodPitch = 0.045;
const nodRoll = 0.025;
const glanceYaw = 0.035;
const glanceGazeX = 0.2;
const glanceGazeY = 0.06;
const browLift = 0.4;

/** Motion at `time` ms. `scale` reduces head and gaze motion (reduced-motion preference). */
export function sampleSpeechMotion(
  plan: SpeechMotionPlan,
  time: number,
  scale = 1,
): SpeechMotionFrame {
  const frame: SpeechMotionFrame = {
    pitch: 0,
    roll: 0,
    yaw: 0,
    gazeX: 0,
    gazeY: 0,
    brow: 0,
  };
  for (const nod of plan.nods) {
    const u = bump(time, nod) * nod.amount;
    if (u === 0) continue;
    frame.pitch += u * nodPitch * scale;
    frame.roll += u * nod.tilt * nodRoll * scale;
  }
  for (const glance of plan.glances) {
    const u = plateau(time, glance, 180, 350) * glance.amount;
    if (u === 0) continue;
    frame.yaw += u * glance.side * glanceYaw * scale;
    frame.gazeX += u * glance.side * glanceGazeX * scale;
    frame.gazeY += u * glanceGazeY * scale;
  }
  for (const raise of plan.browRaises)
    frame.brow = Math.max(
      frame.brow,
      plateau(time, raise, 150, 350) * browLift,
    );
  return frame;
}
