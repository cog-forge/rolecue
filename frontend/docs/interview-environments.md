# Interview room 3D environments (KAN-275)

The interview room can stand the VRM interviewer in a photographic 3D room. Each scene is:

- a 360° photo (`backplate.webp`, 4096×2048) projected onto a ground-projected skybox
  (`GroundedSkybox`), drawn without tone mapping because it is already tone-mapped;
- the HDR of the same photo (`lighting.hdr`, 512×256) as `scene.environment`, which lights the furniture;
- real foreground furniture (`props.glb`) whose edge hides the standing interviewer's hips;
- a light rig per scene (`light` in `scenes.ts`) that matches the photo, because MToon avatars ignore
  image-based light;
- `still.webp` (the scene rendered without the interviewer, 1920×1080) and `thumb.webp` (320×180).

Code: `src/components/avatar-viewer/environment/` (catalog `scenes.ts`, loader, R3F layer, state machine).
The picker lists the scenes in `src/features/interview/components/room-backgrounds.ts`; the three
generated 2D photos stay as the lightweight choice.

## Behaviour

- Only the chosen scene downloads, with byte progress shown on the background button and in the picker.
  Downloads are cached in memory for the page, so switching back is instant.
- While it loads, and whenever it can't render, the scene's `still.webp` is the room backdrop.
- It falls back to the still when WebGL is missing, WebGL 2 is missing, the renderer is software
  (SwiftShader, llvmpipe…), `MAX_TEXTURE_SIZE` < 4096, the interviewer canvas isn't running, a file
  fails, or loading takes over 20 s.
- Leaving a scene or the room removes it and disposes its geometries, materials, textures and the
  PMREM generated from the HDR.
- Camera framing, camera views and the avatar's pose are unchanged; the scene is placed around them.

## Sizes

| Scene                    | backplate | lighting | props  | 3D total | still | thumb |
| ------------------------ | --------- | -------- | ------ | -------- | ----- | ----- |
| Café (`cafe`)            | 420 KB    | 417 KB   | 178 KB | 1.02 MB  | 43 KB | 5 KB  |
| Office (`office`)        | 859 KB    | 430 KB   | 179 KB | 1.47 MB  | 93 KB | 10 KB |
| Meeting room (`meeting`) | 300 KB    | 391 KB   | 364 KB | 1.05 MB  | 49 KB | 6 KB  |

`bytes` in `scenes.ts` must equal the 3D total; `tests/unit/components/avatar-viewer/environment/scenes.test.ts`
checks it against `public/environments/`.

## Asset pipeline

Raw downloads are not committed. With ImageMagick 7 and `@gltf-transform/cli` 4.5:

```sh
# 1. Photo: Poly Haven's tonemapped 8K JPG -> 4K WebP
magick <hdri>_tonemapped.jpg -resize 4096x2048 -quality 82 -define webp:method=6 backplate.webp
# 2. Light: Poly Haven's 1K .hdr -> 512x256 .hdr
magick <hdri>_1k.hdr -resize 512x256 lighting.hdr
# 3. Furniture: Poly Haven glTF (1K) -> meshopt GLB, WebP textures at most 1024 px, deduped and pruned
bunx @gltf-transform/cli optimize <model>_1k.gltf props.glb \
  --compress meshopt --texture-compress webp --texture-size 1024
```

The meeting-room table is `dining_table` with its `dining_table_cloth` node removed from the scene
before step 3. `still.webp` and `thumb.webp` are renders of the finished scene from the room camera
(desktop, transcript open), without the interviewer.

## Credits

All assets are CC0 (public domain) from [Poly Haven](https://polyhaven.com); no attribution is required.
We credit them in the picker and here.

| Scene        | Asset                     | Author           | Source                                        |
| ------------ | ------------------------- | ---------------- | --------------------------------------------- |
| Café         | Comfy Café (HDRI)         | Sergej Majboroda | https://polyhaven.com/a/comfy_cafe            |
| Café         | Round Wooden Table 01     | Ulan Cabanilla   | https://polyhaven.com/a/round_wooden_table_01 |
| Office       | Poly Haven Studio (HDRI)  | Greg Zaal        | https://polyhaven.com/a/poly_haven_studio     |
| Office       | Metal Office Desk         | Ulan Cabanilla   | https://polyhaven.com/a/metal_office_desk     |
| Meeting room | Small Empty Room 3 (HDRI) | Sergej Majboroda | https://polyhaven.com/a/small_empty_room_3    |
| Meeting room | Dining Table              | Aron Łyczek      | https://polyhaven.com/a/dining_table          |

A future CC BY asset must add its exact attribution text to this table and to the picker credits.
