# Interviewer Lip-Sync and Speech Motion (KAN-59)

How the 3D interviewer moves its mouth, head and brows while a turn plays. The code is in
`src/components/avatar-viewer/lipsync/`, with the director in `src/components/avatar-viewer/avatar-director.ts`
(KAN-58). The Brain decision is Avatar-Voice, "Interviewer Performance" (2026-10-30).

## Layers

The director applies these layers in order every frame:

1. Idle base pose.
2. Gestures (VRMA clips, KAN-251).
3. Procedural blink, gaze, breathing and listening.
4. Facial emotion.
5. Mouth.

While the interviewer speaks, the mouth wins. Emotion `overrideMouth` is cleared, and emotion is applied
through upper-face-only binds, so the jaw and lips follow the mouth source only. Brow, cheek and nose
values from the mouth source and from speech motion are added lightly on top of emotion. Eye channels
from any source are ignored, because they belong to the blink and gaze layers.

## Avatar Inventory

All our VRMs come from Avaturn GLB through `api/internal/features/inventory/converter/avaturn_to_vrm.py`.
Three team VRMs were checked on 2026-11-02. The files were downloaded read-only to a temporary folder and
deleted afterwards; they are likenesses and must never be committed. All three have the same expression
set:

| Avatar                  | Spec    | Presets bound           | Custom expressions | Meshes with morph targets                                  |
| ----------------------- | ------- | ----------------------- | ------------------ | ---------------------------------------------------------- |
| 1 (2026-10-26)          | VRM 1.0 | 17 (`neutral` is empty) | 72                 | Head 72, Teeth 20, Tongue 21, EyeAO 32, Eyelash 31, Eye 11 |
| 2 (2026-10-28, glasses) | VRM 1.0 | 17                      | 72                 | same                                                       |
| 3 (2026-10-27, glasses) | VRM 1.0 | 17                      | 72                 | same                                                       |

- **Vowel presets:** `aa/ih/ou/ee/oh` bind `viseme_aa/I/U/E/O` on the Head, Teeth and Tongue meshes.
- **Other presets:**
  - `blink` binds `eyesClosed`;
  - `lookUp/Down/Left/Right` bind the eye-look shapes (`lookAt` type `expression`);
  - emotions bind small mouth or brow shapes.
  - Every `override*` is `none`.
- **The 15 Oculus visemes** are custom expressions: `viseme_sil`, `viseme_PP`, `viseme_FF`, `viseme_TH`,
  `viseme_DD`, `viseme_kk`, `viseme_CH`, `viseme_SS`, `viseme_nn`, `viseme_RR`, `viseme_aa`, `viseme_E`,
  `viseme_I`, `viseme_O` and `viseme_U`. Each binds Head, Teeth and Tongue.
- **52 ARKit shapes** are custom expressions named exactly as Azure's indices 0–51, with no naming
  differences. Brows and cheeks also bind EyeAO and Eyelash.
- **Avaturn extras:** `eyesClosed`, `eyesLookDown`, `eyesLookUp`, `mouthOpen` and `mouthSmile` (not used
  by lip-sync).
- **Missing:** Azure's three roll channels (`headRoll`, `leftEyeRoll`, `rightEyeRoll`).

The mouth source returns VRM expression names. The director applies two groups:

- **Mouth channels:** the vowel presets plus names starting with `viseme_`, `mouth`, `jaw` or `tongue`.
- **Accent channels:** names starting with `brow`, `cheek` or `nose`.

Other VRMs (VRoid, for example) may lack the custom `viseme_*` and ARKit expressions. On such a model the
blend-shape and viseme sources move nothing, but the audio-analysis sources still work, because they
drive the standard vowel presets.

## Azure Blend Shapes → VRM

Azure sends 55 weights per 60 FPS frame, in the order documented in Microsoft Learn's "Get facial position
with viseme". `sampleBlendShapes` interpolates linearly between the two frames around `audio.currentTime`.
The table shows the weight applied to each channel.

| Index | Azure name            | VRM expression        | Weight                                           |
| ----- | --------------------- | --------------------- | ------------------------------------------------ |
| 0     | `eyeBlinkLeft`        | `eyeBlinkLeft`        | not applied (blink and gaze layers own the eyes) |
| 1     | `eyeLookDownLeft`     | `eyeLookDownLeft`     | not applied (blink and gaze layers own the eyes) |
| 2     | `eyeLookInLeft`       | `eyeLookInLeft`       | not applied (blink and gaze layers own the eyes) |
| 3     | `eyeLookOutLeft`      | `eyeLookOutLeft`      | not applied (blink and gaze layers own the eyes) |
| 4     | `eyeLookUpLeft`       | `eyeLookUpLeft`       | not applied (blink and gaze layers own the eyes) |
| 5     | `eyeSquintLeft`       | `eyeSquintLeft`       | not applied (blink and gaze layers own the eyes) |
| 6     | `eyeWideLeft`         | `eyeWideLeft`         | not applied (blink and gaze layers own the eyes) |
| 7     | `eyeBlinkRight`       | `eyeBlinkRight`       | not applied (blink and gaze layers own the eyes) |
| 8     | `eyeLookDownRight`    | `eyeLookDownRight`    | not applied (blink and gaze layers own the eyes) |
| 9     | `eyeLookInRight`      | `eyeLookInRight`      | not applied (blink and gaze layers own the eyes) |
| 10    | `eyeLookOutRight`     | `eyeLookOutRight`     | not applied (blink and gaze layers own the eyes) |
| 11    | `eyeLookUpRight`      | `eyeLookUpRight`      | not applied (blink and gaze layers own the eyes) |
| 12    | `eyeSquintRight`      | `eyeSquintRight`      | not applied (blink and gaze layers own the eyes) |
| 13    | `eyeWideRight`        | `eyeWideRight`        | not applied (blink and gaze layers own the eyes) |
| 14    | `jawForward`          | `jawForward`          | 1.0                                              |
| 15    | `jawLeft`             | `jawLeft`             | 1.0                                              |
| 16    | `jawRight`            | `jawRight`            | 1.0                                              |
| 17    | `jawOpen`             | `jawOpen`             | 1.0                                              |
| 18    | `mouthClose`          | `mouthClose`          | 1.0                                              |
| 19    | `mouthFunnel`         | `mouthFunnel`         | 1.0                                              |
| 20    | `mouthPucker`         | `mouthPucker`         | 1.0                                              |
| 21    | `mouthLeft`           | `mouthLeft`           | 1.0                                              |
| 22    | `mouthRight`          | `mouthRight`          | 1.0                                              |
| 23    | `mouthSmileLeft`      | `mouthSmileLeft`      | 1.0                                              |
| 24    | `mouthSmileRight`     | `mouthSmileRight`     | 1.0                                              |
| 25    | `mouthFrownLeft`      | `mouthFrownLeft`      | 1.0                                              |
| 26    | `mouthFrownRight`     | `mouthFrownRight`     | 1.0                                              |
| 27    | `mouthDimpleLeft`     | `mouthDimpleLeft`     | 1.0                                              |
| 28    | `mouthDimpleRight`    | `mouthDimpleRight`    | 1.0                                              |
| 29    | `mouthStretchLeft`    | `mouthStretchLeft`    | 1.0                                              |
| 30    | `mouthStretchRight`   | `mouthStretchRight`   | 1.0                                              |
| 31    | `mouthRollLower`      | `mouthRollLower`      | 1.0                                              |
| 32    | `mouthRollUpper`      | `mouthRollUpper`      | 1.0                                              |
| 33    | `mouthShrugLower`     | `mouthShrugLower`     | 1.0                                              |
| 34    | `mouthShrugUpper`     | `mouthShrugUpper`     | 1.0                                              |
| 35    | `mouthPressLeft`      | `mouthPressLeft`      | 1.0                                              |
| 36    | `mouthPressRight`     | `mouthPressRight`     | 1.0                                              |
| 37    | `mouthLowerDownLeft`  | `mouthLowerDownLeft`  | 1.0                                              |
| 38    | `mouthLowerDownRight` | `mouthLowerDownRight` | 1.0                                              |
| 39    | `mouthUpperUpLeft`    | `mouthUpperUpLeft`    | 1.0                                              |
| 40    | `mouthUpperUpRight`   | `mouthUpperUpRight`   | 1.0                                              |
| 41    | `browDownLeft`        | `browDownLeft`        | 0.3                                              |
| 42    | `browDownRight`       | `browDownRight`       | 0.3                                              |
| 43    | `browInnerUp`         | `browInnerUp`         | 0.3                                              |
| 44    | `browOuterUpLeft`     | `browOuterUpLeft`     | 0.3                                              |
| 45    | `browOuterUpRight`    | `browOuterUpRight`    | 0.3                                              |
| 46    | `cheekPuff`           | `cheekPuff`           | 0.3                                              |
| 47    | `cheekSquintLeft`     | `cheekSquintLeft`     | 0.3                                              |
| 48    | `cheekSquintRight`    | `cheekSquintRight`    | 0.3                                              |
| 49    | `noseSneerLeft`       | `noseSneerLeft`       | 0.3                                              |
| 50    | `noseSneerRight`      | `noseSneerRight`      | 0.3                                              |
| 51    | `tongueOut`           | `tongueOut`           | 1.0                                              |
| 52    | `headRoll`            | —                     | not applied (no VRM channel)                     |
| 53    | `leftEyeRoll`         | —                     | not applied (no VRM channel)                     |
| 54    | `rightEyeRoll`        | —                     | not applied (no VRM channel)                     |

Weights are used as Azure sends them, so they are not re-smoothed. Two notes from the captured turn:

- **Closing the lips:** Azure closes the lips the ARKit way, with `mouthClose` ≈ `jawOpen` (for example
  0.615 and 0.595 on the /b/ of "scalable"). On our Avaturn meshes this closes the lips correctly. On one
  avatar the combination slightly stretches the skin texture under a nostril; the geometry is fine.
- **Timing and rest:** blend-shape frames lead the matching viseme event by about 50 ms, so the lips close
  before the burst. After the last word, Azure leaves a slightly open rest (`jawOpen` ≈ 0.18). The mouth
  is released when the audio ends, so that rest is never held.

## Azure Viseme IDs → Oculus

The viseme source is used when a turn has viseme IDs but no blend shapes. The mapping is based on
TalkingHead's newer Azure example (`examples/azure-audio-streaming.html`). The diphthongs aʊ (9) and aɪ
(11) use their open onset `aa` instead of `O`/`I`.

| ID  | en-US phonemes | Oculus | VRM expression          |
| --- | -------------- | ------ | ----------------------- |
| 0   | silence        | sil    | (rest, nothing applied) |
| 1   | æ ə ʌ          | aa     | `viseme_aa`             |
| 2   | ɑ              | aa     | `viseme_aa`             |
| 3   | ɔ              | O      | `viseme_O`              |
| 4   | ɛ ʊ            | E      | `viseme_E`              |
| 5   | ɝ              | RR     | `viseme_RR`             |
| 6   | j i ɪ          | I      | `viseme_I`              |
| 7   | w u            | U      | `viseme_U`              |
| 8   | o              | O      | `viseme_O`              |
| 9   | aʊ             | aa     | `viseme_aa`             |
| 10  | ɔɪ             | O      | `viseme_O`              |
| 11  | aɪ             | aa     | `viseme_aa`             |
| 12  | h              | kk     | `viseme_kk`             |
| 13  | ɹ              | RR     | `viseme_RR`             |
| 14  | l              | nn     | `viseme_nn`             |
| 15  | s z            | SS     | `viseme_SS`             |
| 16  | ʃ tʃ dʒ ʒ      | CH     | `viseme_CH`             |
| 17  | ð              | TH     | `viseme_TH`             |
| 18  | f v            | FF     | `viseme_FF`             |
| 19  | d t n θ        | DD     | `viseme_DD`             |
| 20  | k g ŋ          | kk     | `viseme_kk`             |
| 21  | p b m          | PP     | `viseme_PP`             |

**Coarticulation (after TalkingHead):** each viseme gets its own envelope, and neighbouring envelopes
overlap:

- it ramps in from `offset − min(60, 2d/3)` ms;
- it peaks at `offset + min(25, d/2)` ms;
- it releases until `offset + d + min(60, d/2)` ms.

Here `d` is the gap to the next viseme, with a minimum of 40 ms (120 ms for the last one). Peaks are
1.0 for PP, 0.9 for FF and 0.75 for every other viseme. When the same viseme repeats, the maximum is
taken, never the sum.

## Fallback Order and Source Selection

`createInterviewerMouth(audio, timelines)` picks the first source available, following the Brain order:

| Order | Source             | When it is used                                                | Output                         | Attack / release      |
| ----- | ------------------ | -------------------------------------------------------------- | ------------------------------ | --------------------- |
| 1     | Azure blend shapes | `blend_shapes` is not empty (API built with `azure_sdk`)       | ARKit channels (table above)   | none (already 60 FPS) |
| 2     | Viseme IDs         | `visemes` is not empty                                         | `viseme_*` with coarticulation | 20 / 40 ms            |
| 3     | wLipSync           | no timelines (REST synthesizer, or one call fell back to REST) | `aa/ee/ih/oh/ou`               | 20 / 33 ms            |
| 4     | Amplitude          | wLipSync cannot load (no AudioWorklet, profile fetch failed)   | `aa`                           | 30 / 80 ms            |

**Timing rules (all sources):**

- `sample()` reads `audio.currentTime`, extrapolated by at most 100 ms between coarse updates.
- It returns `null` while the element is paused or ended, after `dispose()`, or when another source was
  loaded into the element. `null` makes the director release the mouth.
- A jump of more than 250 ms, or any backward jump, is treated as a seek and resets smoothing. The mouth
  then lands on the timeline instead of easing from stale weights.
- Silence inside a playing turn is a still, closed mouth: an empty sample, never `null` and never
  movement.

**Audio analysis (sources 3 and 4):**

- The Web Audio graph is created lazily, only after the page has had a user gesture
  (`navigator.userActivation`, otherwise the next pointer or key event).
- `createMediaElementSource` works once per element and reroutes its sound through the context. The
  element is therefore attached only once the context is running, stays connected to the speakers, and the
  context is never closed. One graph serves the element for its whole lifetime; per-turn analysis nodes
  are attached and detached.
- **Anything else that needs Web Audio on the playback element must reuse this graph**
  (`connectAudioGraph` in `audio-analysis.ts`) instead of creating its own source.
- wLipSync gating (after AIRI):
  - amplitude = `min(volume · 0.9, 1)^0.7`;
  - the winner vowel is capped at 0.7 and the runner-up at 0.35;
  - the mouth closes after 160 ms of amplitude < 0.04 or winner < 0.05.
- Amplitude (after Amica): `1 / (1 + e^(−45·peak + 5))`, floored at 0.1, then × 0.6 into `aa`.
- wlipsync is imported dynamically. It evaluates `AudioWorkletNode` at import time and so must never
  load on the server.
- Its profile is served from `public/lipsync/wlipsync-profile.bin`. That is the MIT example profile from
  the wLipSync repository (713 bytes, A/I/U/E/O/S at 16 kHz); the npm package ships none.

### HeadAudio Spike (2026-11-02)

**Decision:** keep wlipsync. HeadAudio is the better choice only if the fallback ever needs all 15
visemes from raw audio.

|          | wlipsync 1.3.1                                                         | HeadAudio (`@met4citizen/headaudio` 1.0.0)                                                                 |
| -------- | ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| License  | MIT                                                                    | MIT                                                                                                        |
| Size     | 55 KB unpacked; one 21 KB file with the worklet and 12 KB WASM inlined | 133 KB unpacked; the runtime needs 6 KB + a 13 KB worklet + a 14 KB model                                  |
| Runtime  | AudioWorklet + WASM (data URLs); nothing extra to serve                | AudioWorklet (plain JS, MFCC + Gaussian prototypes); the worklet and model must be served as separate URLs |
| Output   | A/E/I/O/U/S → VRM vowels                                               | all 15 Oculus visemes                                                                                      |
| Next 16  | dynamic import in a client effect works (verified in Chrome)           | broken `main` (points to a missing `.js`), no `exports`; deep import plus `public/` assets required        |
| Maturity | used in production by AIRI                                             | published 2026-10-22; the README notes 50–100 ms latency and weak accuracy at low SNR                      |

## Speech-Driven Motion

`AvatarPerformance.speech = { audio, words, text }` adds subtle motion synced to `audio.currentTime`, only
in the `speaking` state while the audio plays:

- **Nods.** A head dip of about 2.6° (0.045 rad) peak over 380 ms on stressed words. A word counts as
  stressed when it is a content word, not the first word of a sentence, and lasts ≥ 280 ms or has ≥ 6
  letters. Nods are at least 900 ms apart, and every other nod adds a slight tilt.
- **Glances.** At each sentence start, a brief look aside lasting 0.9 s: gaze ±0.2 m at 3 m and head yaw
  ±0.035 rad, alternating sides.
- **Brow lift.** On the last word of each sentence that ends with "?": `browInnerUp` 0.4 and
  `browOuterUp*` 0.24.

Sentences come from `text` (which has punctuation). They are matched in order to `words` (which have no
punctuation); a token Azure spelled differently is skipped. With `prefers-reduced-motion`, head and gaze
motion drop to 30% of their amplitude. Brows stay as they are, because they are an expression rather than
motion, like emotion. Blink, gaze, breathing, emotion and gestures are unchanged.

## Integration

`useInterviewerSpeech(audio, turn)` (in `lipsync/use-interviewer-speech.ts`) creates one mouth per turn and
disposes the previous one on a new turn, a reconnect re-send, a new element or unmount. Its `mouth` is a
stable bridge. The interview room adds a few lines:

```tsx
const { mouth, speech } = useInterviewerSpeech(playback.audio, playback.turn);
const performance = useMemo<AvatarPerformance>(
  () => ({ state, candidateSpeaking, mouth, speech }),
  [state, candidateSpeaking, mouth, speech],
);
```

- `playback.turn` is the `interviewer.turn` data (`text`, `visemes`, `blend_shapes`, `words`). The hook
  takes it structurally, so no transport type is imported.
- `state` must be `speaking` while the turn audio plays.
- If no audio was produced (`audio_status` other than `ready`), pass `turn: null` or leave the element
  paused. The mouth then stays released; it never flaps on text.

## Verification

Unit tests in `tests/unit/components/avatar-viewer/` cover timeline sampling, source selection, mapping,
smoothing, silence, seek, interruption, dispose and the director's speech motion. They use
`tests/fixtures/interviewer-turn-timelines.json`, trimmed from a real SDK capture with the audio removed.
The sources were also compared side by side on three team VRMs in a temporary development page (not
committed).

## Credits

The repository has no `THIRD_PARTY_NOTICES` file, so the credits are recorded here and in short comments
at each use. No third-party code was copied verbatim; mappings, constants and approaches were adapted.
No assets were copied except the wLipSync profile (data file, MIT).

| Project                                                                                                         | License | Used for                                                                                    | Where                                                      |
| --------------------------------------------------------------------------------------------------------------- | ------- | ------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| [wLipSync](https://github.com/mrxz/wLipSync) (`wlipsync` npm), © 2021 hecomi, © 2024 Noeri Huisman              | MIT     | MFCC vowel analysis (dependency) and `example/profile.bin`                                  | `audio-analysis.ts`, `public/lipsync/wlipsync-profile.bin` |
| [TalkingHead](https://github.com/met4citizen/TalkingHead), © 2023-2024 Mika Suominen                            | MIT     | Azure viseme → Oculus mapping, coarticulation envelope timing, ideas for speech head motion | `azure.ts`, `timeline-drivers.ts`, `speech-motion.ts`      |
| [AIRI](https://github.com/moeru-ai/airi) `packages/model-driver-lipsync`, © 2024-present Neko Ayaka             | MIT     | wLipSync winner/runner-up selection, silence gate, vowel mapping                            | `audio-analysis.ts`                                        |
| [Amica](https://github.com/semperai/amica) `src/features/lipSync` and `autoBlink`, © 2023 Semper AI, pixiv Inc. | MIT     | Amplitude fallback formula; blink timing (KAN-58)                                           | `audio-analysis.ts`, `avatar-director.ts`                  |

All four use the MIT License: permission is granted, free of charge, to use, copy, modify, merge, publish,
distribute, sublicense and sell copies, provided the copyright notice and permission notice are included;
the software is provided "as is", without warranty of any kind. The full text ships in
`node_modules/wlipsync/LICENSE` and in each upstream repository.
