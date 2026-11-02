/**
 * Azure Speech viseme output mapped to the expressions our Avaturn → VRM converter keeps.
 * Verified against real team VRMs; see frontend/docs/avatar-lip-sync.md.
 */

/** Azure `FacialExpression` blend-shape order (Microsoft Learn, "Get facial position with viseme"). */
export const azureBlendShapeNames = [
  "eyeBlinkLeft",
  "eyeLookDownLeft",
  "eyeLookInLeft",
  "eyeLookOutLeft",
  "eyeLookUpLeft",
  "eyeSquintLeft",
  "eyeWideLeft",
  "eyeBlinkRight",
  "eyeLookDownRight",
  "eyeLookInRight",
  "eyeLookOutRight",
  "eyeLookUpRight",
  "eyeSquintRight",
  "eyeWideRight",
  "jawForward",
  "jawLeft",
  "jawRight",
  "jawOpen",
  "mouthClose",
  "mouthFunnel",
  "mouthPucker",
  "mouthLeft",
  "mouthRight",
  "mouthSmileLeft",
  "mouthSmileRight",
  "mouthFrownLeft",
  "mouthFrownRight",
  "mouthDimpleLeft",
  "mouthDimpleRight",
  "mouthStretchLeft",
  "mouthStretchRight",
  "mouthRollLower",
  "mouthRollUpper",
  "mouthShrugLower",
  "mouthShrugUpper",
  "mouthPressLeft",
  "mouthPressRight",
  "mouthLowerDownLeft",
  "mouthLowerDownRight",
  "mouthUpperUpLeft",
  "mouthUpperUpRight",
  "browDownLeft",
  "browDownRight",
  "browInnerUp",
  "browOuterUpLeft",
  "browOuterUpRight",
  "cheekPuff",
  "cheekSquintLeft",
  "cheekSquintRight",
  "noseSneerLeft",
  "noseSneerRight",
  "tongueOut",
  "headRoll",
  "leftEyeRoll",
  "rightEyeRoll",
] as const;

/** Brows, cheeks and nose follow speech only lightly; the emotion layer owns the upper face. */
export const upperFaceGain = 0.3;

/**
 * Channels applied from a blend-shape frame. Eye blink/look/squint/wide belong to the blink and gaze
 * layers, and VRM has no roll channels, so those indices are never applied.
 */
export const azureBlendShapeChannels: readonly {
  index: number;
  name: string;
  gain: number;
}[] = azureBlendShapeNames.flatMap((name, index) => {
  if (name.startsWith("eye") || name.endsWith("Roll")) return [];
  const gain = /^(brow|cheek|nose)/.test(name) ? upperFaceGain : 1;
  return [{ index, name, gain }];
});

export const oculusVisemes = [
  "sil",
  "PP",
  "FF",
  "TH",
  "DD",
  "kk",
  "CH",
  "SS",
  "nn",
  "RR",
  "aa",
  "E",
  "I",
  "O",
  "U",
] as const;
export type OculusViseme = (typeof oculusVisemes)[number];

/**
 * Azure viseme ID (en-US) → Oculus viseme. Based on TalkingHead (met4citizen, MIT,
 * examples/azure-audio-streaming.html); the diphthongs aʊ (9) and aɪ (11) use their open onset.
 */
export const azureVisemeToOculus: readonly OculusViseme[] = [
  "sil", // 0 silence
  "aa", // 1 æ ə ʌ
  "aa", // 2 ɑ
  "O", // 3 ɔ
  "E", // 4 ɛ ʊ
  "RR", // 5 ɝ
  "I", // 6 j i ɪ
  "U", // 7 w u
  "O", // 8 o
  "aa", // 9 aʊ
  "O", // 10 ɔɪ
  "aa", // 11 aɪ
  "kk", // 12 h
  "RR", // 13 ɹ
  "nn", // 14 l
  "SS", // 15 s z
  "CH", // 16 ʃ tʃ dʒ ʒ
  "TH", // 17 ð
  "FF", // 18 f v
  "DD", // 19 d t n θ
  "kk", // 20 k g ŋ
  "PP", // 21 p b m
];

/** Expression name the converter keeps for each Oculus viseme (custom VRM expressions). */
export const visemeExpression = (viseme: OculusViseme) => `viseme_${viseme}`;
