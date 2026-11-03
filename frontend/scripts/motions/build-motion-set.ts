/**
 * Builds the interviewer motion set (KAN-251) into `public/motions/`.
 *
 *   bun scripts/motions/build-motion-set.ts <folder with the Mixamo FBX downloads>
 *
 * Six clips come from Adobe Mixamo (downloaded 2026-11-03: FBX Binary, Without Skin, 30 FPS) and are
 * converted by `convert-mixamo.ts`; `listen` is keyframed here because Mixamo has no standing listening
 * pose that fits. The FBX files are not committed; `public/motions/CREDITS.md` records each source.
 * Trim windows were chosen from the hand positions over time (see `docs/avatar-performance.md`).
 */
import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { Euler, Quaternion } from "three";
import {
  buildMixamoClip,
  firstFrame,
  sampleMixamo,
  type Sampled,
} from "./convert-mixamo";
import { encodeVrma, type HumanoidBone, type MotionClip } from "./vrma-writer";

const isArmBone = (bone: HumanoidBone) =>
  /Shoulder|Arm|Hand|Thumb|Index|Middle|Ring|Little/.test(bone);

type Source = {
  name: string;
  file: string;
  start?: number;
  end?: number;
  /** Absolute loop (the idle). */
  absolute?: boolean;
  /**
   * The pose a delta is taken from: the clip's own first frame (untrimmed) by default, so a source's
   * standing posture cancels out and only its motion is layered over the idle; `idle` for a clip that
   * is already in its pose at frame 0 (the lean).
   */
  reference?: "own" | "idle";
  /**
   * Take the arm, hand and finger deltas from the idle instead, so the arms reach the clip's exact pose
   * (a raised hand would otherwise land off by the posture difference between the two sources).
   */
  armsFromIdle?: boolean;
  /** Per-bone multipliers on the delta. */
  boneAmount?: Record<string, number>;
  /** Keep only these bones (the lean moves the torso and head, never the arms). */
  only?: readonly string[];
  loop?: boolean;
  ease?: number;
  amount?: number;
};

const FPS = 20;
const sources: Source[] = [
  { name: "idle", file: "Breathing Idle.fbx", absolute: true, loop: true },
  { name: "nod", file: "Head Nod Yes.fbx", ease: 0.25 },
  // The source tips the head back while waving; keep a little of it.
  {
    name: "greet",
    file: "Standing Greeting.fbx",
    start: 1.0,
    end: 4.6,
    ease: 0.35,
    armsFromIdle: true,
    boneAmount: { head: 0.3, neck: 0.3 },
  },
  // The explaining part of "Standing Arguing", before the hands go up; toned down for an interviewer.
  {
    name: "explain_hand",
    file: "Standing Arguing.fbx",
    start: 5.2,
    end: 8.8,
    ease: 0.4,
    amount: 0.75,
    armsFromIdle: true,
  },
  // The held hand-to-chin part of "Thinking", looped while the answer is processed; a calmer head turn.
  {
    name: "thinking",
    file: "Thinking.fbx",
    start: 1.0,
    end: 2.8,
    loop: true,
    armsFromIdle: true,
    boneAmount: { head: 0.5, neck: 0.5 },
  },
  // "Leaning" is a static forward lean; eased in and out it becomes a one-shot lean-in.
  {
    name: "lean_in",
    file: "Leaning.fbx",
    end: 2.6,
    ease: 0.6,
    amount: 0.6,
    reference: "idle",
    only: ["spine", "chest", "upperChest", "neck", "head"],
  },
];

/** A small head tilt and spine lean towards the Candidate, looped. Deltas from the idle pose. */
function listenClip(): MotionClip {
  const duration = 3.2;
  const tau = Math.PI * 2;
  const curves = {
    head: (t: number) => [
      0.03 + 0.01 * Math.sin((tau * t) / duration + 1),
      -0.04,
      0.06 + 0.012 * Math.sin((tau * t) / duration),
    ],
    neck: () => [0, -0.03, 0],
    spine: () => [0.03, 0, 0],
  } as const;
  const rotations: MotionClip["rotations"] = {};
  for (const [bone, curve] of Object.entries(curves)) {
    const times: number[] = [];
    const values: number[] = [];
    for (let i = 0; i <= duration * FPS; i++) {
      const t = i / FPS;
      const [x, y, z] = curve(t);
      times.push(Number(t.toFixed(4)));
      values.push(
        ...new Quaternion()
          .setFromEuler(new Euler(x, y, z))
          .toArray()
          .map((v) => Number(v.toFixed(5))),
      );
    }
    rotations[bone as keyof typeof curves] = { times, values };
  }
  return { name: "listen", rotations };
}

async function load(
  dir: string,
  source: Source,
): Promise<{ sampled: Sampled; start: Sampled }> {
  const bytes = await readFile(join(dir, source.file));
  const buffer = bytes.buffer.slice(
    bytes.byteOffset,
    bytes.byteOffset + bytes.byteLength,
  );
  return {
    sampled: sampleMixamo(buffer, {
      fps: FPS,
      start: source.start,
      end: source.end,
    }),
    start: sampleMixamo(buffer, { fps: FPS, start: 0, end: 1 / FPS }),
  };
}

async function main() {
  const dir = process.argv[2];
  if (!dir) {
    console.error(
      "usage: build-motion-set.ts <folder with the Mixamo FBX files>",
    );
    process.exit(2);
  }
  const out = join(import.meta.dirname, "..", "..", "public", "motions");
  const idle = await load(dir, sources[0]);
  const idleReference = firstFrame(idle.sampled);
  const clips: { clip: MotionClip; duration: number }[] = [];
  for (const source of sources) {
    const { sampled, start } =
      source === sources[0] ? idle : await load(dir, source);
    const own = firstFrame(start);
    const reference = source.absolute
      ? undefined
      : source.reference === "idle"
        ? idleReference
        : new Map(
            [...own].map(([bone, q]) => [
              bone,
              source.armsFromIdle && isArmBone(bone)
                ? (idleReference.get(bone) ?? q)
                : q,
            ]),
          );
    const clip = buildMixamoClip(source.name, sampled, {
      reference,
      ease: source.ease,
      seam: source.loop ? 0.5 : 0,
      amount: source.amount,
      boneAmount: source.boneAmount as Partial<Record<HumanoidBone, number>>,
    });
    if (source.only)
      for (const bone of Object.keys(
        clip.rotations,
      ) as (keyof typeof clip.rotations)[])
        if (!source.only.includes(bone)) delete clip.rotations[bone];
    clips.push({ clip, duration: sampled.duration });
  }
  clips.push({ clip: listenClip(), duration: 3.2 });
  for (const { clip, duration } of clips) {
    const bytes = encodeVrma(clip);
    await writeFile(join(out, `${clip.name}.vrma`), bytes);
    console.log(
      `${clip.name}.vrma\t${bytes.byteLength} bytes\t${duration.toFixed(2)} s\t${Object.keys(clip.rotations).length} bones`,
    );
  }
}

await main();
