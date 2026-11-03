import type { CueEmotion } from "./vocabulary";

/** VRM preset expressions the director smooths (the upper-face routing while speaking is in the director). */
export type PresetEmotion = "happy" | "relaxed" | "sad" | "angry" | "surprised";

export type EmotionPreset = {
  /** VRM preset expression weights. */
  readonly expressions: Partial<Record<PresetEmotion, number>>;
  /** ARKit-named accent channels (brow, cheek, nose) added lightly; never mouth, jaw or eyes. */
  readonly accents: Readonly<Record<string, number>>;
};

/**
 * How each vocabulary emotion looks. Weights are deliberately small: the interviewer is composed, and
 * lip-sync keeps the mouth. `thinking` lowers the brows a little and relaxes; `curious` lifts them.
 */
export const EMOTION_PRESETS: Readonly<Record<CueEmotion, EmotionPreset>> = {
  neutral: { expressions: { relaxed: 0.12 }, accents: {} },
  friendly: {
    expressions: { happy: 0.45 },
    accents: { cheekSquintLeft: 0.15, cheekSquintRight: 0.15 },
  },
  curious: {
    expressions: { surprised: 0.12 },
    accents: {
      browInnerUp: 0.35,
      browOuterUpLeft: 0.25,
      browOuterUpRight: 0.25,
    },
  },
  encouraging: {
    expressions: { happy: 0.55, relaxed: 0.15 },
    accents: {
      browInnerUp: 0.15,
      cheekSquintLeft: 0.2,
      cheekSquintRight: 0.2,
    },
  },
  thinking: {
    expressions: { relaxed: 0.2 },
    accents: { browDownLeft: 0.3, browDownRight: 0.3, browInnerUp: 0.1 },
  },
};
