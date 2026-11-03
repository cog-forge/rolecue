/**
 * The bounded performance vocabulary (KAN-250) and the motion set that performs it (KAN-251). The server
 * sends cues by name; the client only ever plays names it knows, so a stray name degrades to nothing.
 */
export const CUE_EMOTIONS = [
  "neutral",
  "friendly",
  "curious",
  "encouraging",
  "thinking",
] as const;
export const CUE_GESTURES = [
  "nod",
  "explain_hand",
  "lean_in",
  "greet",
  "listen",
] as const;
export type CueEmotion = (typeof CUE_EMOTIONS)[number];
export type CueGesture = (typeof CUE_GESTURES)[number];

/** One cue as the `interviewer.turn` message carries it: an offset from the audio start. */
export type PerformanceCue = {
  readonly offset_ms: number;
  readonly kind: "emotion" | "gesture";
  readonly name: string;
};

export const isCueEmotion = (name: string): name is CueEmotion =>
  (CUE_EMOTIONS as readonly string[]).includes(name);
export const isCueGesture = (name: string): name is CueGesture =>
  (CUE_GESTURES as readonly string[]).includes(name);

/** Clip names: every gesture, the idle loop and the thinking loop for the processing state. */
export type MotionName = CueGesture | "idle" | "thinking";

export type MotionSpec = {
  /** Path under the site origin; files live in `public/motions/`. */
  readonly file: string;
  /** Hard per-clip budget: a larger download is refused and the clip stays unavailable. */
  readonly budgetBytes: number;
  /** Looping clips hold a state (idle, listening, thinking); the others play once. */
  readonly loop: boolean;
};

const KB = 1024;
export const MOTION_CLIPS: Readonly<Record<MotionName, MotionSpec>> = {
  idle: { file: "/motions/idle.vrma", budgetBytes: 64 * KB, loop: true },
  nod: { file: "/motions/nod.vrma", budgetBytes: 32 * KB, loop: false },
  explain_hand: {
    file: "/motions/explain_hand.vrma",
    budgetBytes: 64 * KB,
    loop: false,
  },
  lean_in: { file: "/motions/lean_in.vrma", budgetBytes: 32 * KB, loop: false },
  greet: { file: "/motions/greet.vrma", budgetBytes: 48 * KB, loop: false },
  listen: { file: "/motions/listen.vrma", budgetBytes: 32 * KB, loop: true },
  thinking: {
    file: "/motions/thinking.vrma",
    budgetBytes: 48 * KB,
    loop: true,
  },
};
export const MOTION_NAMES = Object.keys(MOTION_CLIPS) as MotionName[];
export const isMotionName = (name: string): name is MotionName =>
  Object.hasOwn(MOTION_CLIPS, name);
