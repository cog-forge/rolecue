# Interviewer Performance: Cues, Motion Set and Director (KAN-250, KAN-251)

How the 3D interviewer shows emotion and body language on top of lip-sync (KAN-59) and the speech
motion of `docs/avatar-lip-sync.md`. The server decides how each line is performed; the client plays
it. The code is in `src/components/avatar-viewer/avatar-director.ts` and
`src/components/avatar-viewer/motions/`; the clips are in `public/motions/` with their credits.

## Vocabulary

The server only ever sends names from this list (`docs/interview-ws-protocol.md`, `interviewer.turn`
→ `cues`). The client plays what it knows and ignores the rest.

| Kind      | Names                                                       |
| --------- | ----------------------------------------------------------- |
| `emotion` | `neutral`, `friendly`, `curious`, `encouraging`, `thinking` |
| `gesture` | `nod`, `explain_hand`, `lean_in`, `greet`, `listen`         |

Each cue carries `offset_ms` from the audio start. An emotion cue stays in force until the next one
of the turn; a gesture plays once. The mouth is never a cue.

### How the server decides (no extra model call)

- **Core questions, rules** (`api/internal/features/interview/runtime/cues.go`): the first turn is
  `friendly` + `greet`; a later turn opens `encouraging` + `nod` (it follows an answer) and turns
  `curious`, or `neutral` for a statement, at the start of the last sentence (after two words for a
  one-sentence question); 18 words or more add `explain_hand` at the second sentence (else a third of
  the way in).
- **Follow-ups**: the KAN-56 decider's JSON gets an optional `cues` array (word index + `emotion` or
  `gesture`, at most 4). The service drops the whole list on anything invalid, and the engine then
  uses `curious` + `lean_in`. A proposed list without an emotion on word 0 gets `curious` first.
- **Anchoring**: the word index becomes `offset_ms` through the TTS `words` timing; past the last
  timing it lands on the last word; without timings (REST synthesizer) the offset is estimated at
  380 ms per word.
- **Persistence**: cues are a function of the turn's frozen text, kind and position plus the synthesis
  (cached per voice and text), so a reconnect re-sends the same cues. No column was added.
- **Fallback**: a turn without audio carries no cues; the line is shown idle and neutral.

### How each emotion looks

`motions/presets.ts` maps the vocabulary to small VRM preset weights and ARKit accent channels:

| Emotion       | Presets                  | Accents                                |
| ------------- | ------------------------ | -------------------------------------- |
| `neutral`     | relaxed 0.12             |                                        |
| `friendly`    | happy 0.45               | cheekSquint L/R 0.15                   |
| `curious`     | surprised 0.12           | browInnerUp 0.35, browOuterUp L/R 0.25 |
| `encouraging` | happy 0.55, relaxed 0.15 | browInnerUp 0.15, cheekSquint L/R 0.2  |
| `thinking`    | relaxed 0.2              | browDown L/R 0.3, browInnerUp 0.1      |

While the interviewer speaks, presets go through the upper-face-only route of KAN-58, so the jaw and
lips follow the mouth source only. Without a cue, the `processing` state shows `thinking`; otherwise
the baseline `emotion` the room passes applies, else `neutral`.

## Motion set

Seven clips, lazy-loaded with a per-clip budget (`motions/vocabulary.ts`), 183 KB in total. Six come
from Adobe Mixamo; sources, trims and licensing are in `public/motions/CREDITS.md`.

| Clip           | Plays                                  | Source (Mixamo)   | Duration | Size  | Budget |
| -------------- | -------------------------------------- | ----------------- | -------- | ----- | ------ |
| `idle`         | always, looping, absolute base pose    | Breathing Idle    | 9.95 s   | 57 KB | 64 KB  |
| `nod`          | cue, listening reaction                | Head Nod Yes      | 2.6 s    | 12 KB | 32 KB  |
| `explain_hand` | cue                                    | Standing Arguing  | 3.6 s    | 48 KB | 64 KB  |
| `lean_in`      | cue, listening reaction                | Leaning           | 2.6 s    | 7 KB  | 32 KB  |
| `greet`        | cue                                    | Standing Greeting | 3.6 s    | 36 KB | 48 KB  |
| `listen`       | held while the Candidate speaks (loop) | keyframed in-repo | 3.2 s    | 9 KB  | 32 KB  |
| `thinking`     | held while `processing` (loop)         | Thinking          | 1.8 s    | 26 KB | 48 KB  |

- `idle` is requested when the director is created, and a turn's gesture clips when the turn starts
  (ahead of their offsets), `listen`/`nod`/`lean_in` when the Candidate starts speaking, `thinking` on
  `processing`. Once `idle` has arrived the rest of the set is prefetched, so a first-turn greeting is
  not missed. A cue whose clip is still in flight plays when the clip arrives, unless it is stale
  (1.5 s) by then. One `MotionLibrary` per page shares every parsed clip across remounts and
  reconnects.
- A clip over budget, a failed download or an unparsable file is unavailable for the page; the
  director degrades (below). Nothing throws.
- **Build:** `bun scripts/motions/build-motion-set.ts <folder with the FBX downloads>` converts the
  Mixamo clips (`convert-mixamo.ts`) and writes `.vrma` with `vrma-writer.ts`. No FBX ships.
- **Conversion rules:**
  - `idle` is absolute. Every other clip is a delta, played additively over the idle.
  - A delta is taken from the clip's own first frame, so the source's standing posture cancels out.
  - Arm gestures (`greet`, `explain_hand`, `thinking`) take their arm, hand and finger deltas from the
    idle's first frame instead, so the arms reach the source pose exactly. Without this a raised hand
    landed about 13 cm off and onto the forehead.
  - The hips rotation is folded into the spine. Hips and legs are never animated, so feet cannot slide.
  - One-shots ease in from and out to the idle; loops blend their last 0.5 s into the first frame.
  - Toned down for an interviewer: `explain_hand` 75 %, `lean_in` 60 % (torso and head only), the head
    of `greet` 30 % and of `thinking` 50 %.

### Rig check

Checked on 2026-11-03 in the practice room on the kan250 clone, with an injected cue script (greet,
explain_hand, nod on every turn) and real listening and processing states, on two converted RoleCue
avatars: the male "Đức" rig and the female "Giám khảo khó tánh" rig. Findings:

- No foot sliding: no clip has a hips or leg track.
- `greet`: the right hand waves beside the head with an open palm on both rigs; the head stays level.
- `explain_hand`: both hands open in front of the waist, palms up; they clear the torso on both rigs.
- `thinking`: the right hand rests at the chin, the left on the hip, the head turns away a little.
- `nod`, `listen`, `lean_in` stay subtle on both proportions.

Record new findings here when a clip changes.

## Director

`AvatarDirector.update(delta, performance, reducedMotion)` runs these layers every frame:

1. Undo last frame's procedural rotations (never a full pose reset: three's `PropertyMixer` skips a
   write when a track did not change, so a held pose would vanish under a reset).
2. Mixer: the `idle` loop (normal blend) plus, additively, one held loop (`listen`, `thinking`, or a
   `listen` cue) and one-shot gestures. Crossfades: 0.3 s for gestures, 0.45 s for holds; a gesture
   fades out 0.3 s before its end.
3. Procedural: breathing when `idle` is missing, a procedural nod when `nod` is missing, the
   processing head turn when `thinking` is missing, and the KAN-59 speech motion.
4. Face: emotion (cue → state → baseline), blink, gaze, then the mouth, which lip-sync owns outright.
   Clips carry no expression tracks; any that did would be stripped on registration.

- **Cues on the audio clock:** `CueCursor` (`motions/cue-timeline.ts`) walks a turn's cues with
  `audio.currentTime`. A new turn object (including the same turn re-sent after a reconnect) starts the
  cursor over; a backward jump replays; a forward jump (hidden tab) skips gestures older than 1.5 s
  instead of firing them in a burst. Leaving `speaking` drops the cursor.
- **Listening:** while `candidateSpeaking`, the `listen` loop is held at 60% and a `nod` plays at
  2.5–4 s, then every 4–8 s, at 60%; after 9 s of speech one `lean_in`. Silence releases the hold.
- **Processing/transitioning:** `thinking` is held and the thinking face is shown; the gaze still looks
  aside.
- **Eyes:** the gaze target sits 3 m in front of the head, so the eyes keep contact with the viewer while
  the head sways. Our Avaturn exports declare an expression-type lookAt with `outputScale: 10` over 90°,
  which saturates the look shapes at 9° and made the eyes swing to their extremes on the idle's head
  motion. The director remaps those range maps to the full shape at 30° (`EYE_RANGE_DEG`) and restores
  them on disposal. The converter should export `outputScale: 1` instead (KAN-122 follow-up).
- **Reduced motion:** no clips, no procedural nods; the base pose and the expressions remain.
- **Disposal** stops every action, restores the base pose and the expression overrides.

## Verification

Unit tests in `tests/unit/components/avatar-viewer/` (`avatar-performance.test.ts`,
`motions/cue-timeline.test.ts`, `motions/motion-library.test.ts`) cover cue scheduling against the
audio clock, additive layering with the mouth untouched, interruption and reconnect replay, the
listening reaction, missing clips, reduced motion, budgets and that every shipped clip parses with no
hips, leg or expression track. Server rules, anchoring and the fallback are tested in
`api/internal/features/interview/runtime/cues_test.go`.
