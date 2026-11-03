# Interviewer motion set (KAN-251)

Every clip in this folder is a VRM Animation (`.vrma`, `VRMC_vrm_animation` 1.0) played through
`@pixiv/three-vrm-animation`. The runtime never loads FBX. Clip names are the performance vocabulary
(KAN-250) plus the `idle` and `thinking` loops. Built with
`bun scripts/motions/build-motion-set.ts <folder with the FBX downloads>`.

| Clip                | Source                                             | Part used                          | Duration | Size  | Budget |
| ------------------- | -------------------------------------------------- | ---------------------------------- | -------- | ----- | ------ |
| `idle.vrma`         | Mixamo "Breathing Idle"                            | whole clip, loop                   | 9.95 s   | 57 KB | 64 KB  |
| `nod.vrma`          | Mixamo "Head Nod Yes"                              | whole clip                         | 2.6 s    | 12 KB | 32 KB  |
| `greet.vrma`        | Mixamo "Standing Greeting"                         | 1.0–4.6 s                          | 3.6 s    | 36 KB | 48 KB  |
| `explain_hand.vrma` | Mixamo "Standing Arguing"                          | 5.2–8.8 s, 75 %                    | 3.6 s    | 48 KB | 64 KB  |
| `thinking.vrma`     | Mixamo "Thinking"                                  | 1.0–2.8 s, loop                    | 1.8 s    | 26 KB | 48 KB  |
| `lean_in.vrma`      | Mixamo "Leaning"                                   | 0–2.6 s, torso and head only, 60 % | 2.6 s    | 7 KB  | 32 KB  |
| `listen.vrma`       | Keyframed in `scripts/motions/build-motion-set.ts` | loop                               | 3.2 s    | 9 KB  | 32 KB  |

Total on disk: 183 KB. A clip over its budget (`src/components/avatar-viewer/motions/vocabulary.ts`) is
refused at load time.

## Mixamo

- **Source:** Adobe Mixamo (<https://www.mixamo.com>), downloaded on 2026-11-03 by the RoleCue team with
  the default character: FBX Binary, Without Skin, 30 FPS.
- **License:** Adobe's Mixamo FAQ allows the animations to be used royalty-free in personal, commercial
  and non-profit projects, but not redistributed as raw animation files on their own. RoleCue ships only
  the converted, trimmed upper-body clips inside the product; the FBX downloads are not committed.
  Adobe General Terms: <https://www.adobe.com/legal/terms.html>.
- **Conversion** (`scripts/motions/convert-mixamo.ts`): retargeting adapted from the three-vrm examples
  (`packages/three-vrm/examples/humanoidAnimation/loadMixamoAnimation.js` and `mixamoVRMRigMap.js`),
  © pixiv Inc., MIT License. Hips and legs are dropped (the hips rotation is folded into the spine),
  so feet cannot slide; tracks are resampled to 20 FPS and reduced.

To replace a clip, download the new FBX with the same settings, edit the source list in
`build-motion-set.ts`, rebuild, update this table and repeat the rig check in
`docs/avatar-performance.md`.

Nothing here was copied from Amica or AIRI; their `.vrma` files keep their authors' licenses.
