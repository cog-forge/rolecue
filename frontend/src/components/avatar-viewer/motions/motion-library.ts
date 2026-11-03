import type { VRMAnimation } from "@pixiv/three-vrm-animation";
import { MOTION_CLIPS, type MotionName } from "./vocabulary";

/**
 * Lazy, budgeted loading of the motion clips (KAN-251). A clip is fetched the first time it is
 * requested, parsed once and shared by every director (model remounts, reconnects, the preview). A
 * download over its budget, a network error or an unparsable file leaves the clip unavailable for the
 * rest of the page; the director then degrades to its procedural motion. Nothing here throws.
 */
export interface MotionLibrary {
  /** Starts loading a clip if it was never requested. Safe to call every frame. */
  request(name: MotionName): void;
  /** The parsed clip, `undefined` while it loads, `null` when it is unavailable. */
  take(name: MotionName): VRMAnimation | null | undefined;
}

export type MotionParser = (bytes: ArrayBuffer) => Promise<VRMAnimation>;
export type MotionFetch = (url: string) => Promise<Response>;

type Entry = { animation: VRMAnimation | null | undefined };

/** Parses a .vrma with three's GLTFLoader and the VRMA plugin, both loaded on demand (client only). */
export const parseVrma: MotionParser = async (bytes) => {
  const [{ GLTFLoader }, { VRMAnimationLoaderPlugin }] = await Promise.all([
    import("three/examples/jsm/loaders/GLTFLoader.js"),
    import("@pixiv/three-vrm-animation"),
  ]);
  const loader = new GLTFLoader();
  loader.register((parser) => new VRMAnimationLoaderPlugin(parser));
  const gltf = await loader.parseAsync(bytes, "");
  const animations = gltf.userData.vrmAnimations as VRMAnimation[] | undefined;
  const animation = animations?.[0];
  if (!animation) throw new Error("the file carries no VRM animation");
  return animation;
};

export function createMotionLibrary({
  fetch: fetchClip = (url) => fetch(url, { credentials: "omit" }),
  parse = parseVrma,
  onUnavailable = (name: MotionName, reason: string) => {
    console.warn(`interviewer motion "${name}" unavailable: ${reason}`);
  },
}: {
  fetch?: MotionFetch;
  parse?: MotionParser;
  onUnavailable?: (name: MotionName, reason: string) => void;
} = {}): MotionLibrary {
  const entries = new Map<MotionName, Entry>();
  const load = async (name: MotionName, entry: Entry) => {
    const spec = MOTION_CLIPS[name];
    try {
      const response = await fetchClip(spec.file);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const declared = Number(response.headers.get("content-length"));
      if (Number.isFinite(declared) && declared > spec.budgetBytes)
        throw new Error(
          `${declared} bytes is over the ${spec.budgetBytes} byte budget`,
        );
      const bytes = await response.arrayBuffer();
      if (bytes.byteLength > spec.budgetBytes)
        throw new Error(
          `${bytes.byteLength} bytes is over the ${spec.budgetBytes} byte budget`,
        );
      const animation = await parse(bytes);
      if (!(animation.duration > 0)) throw new Error("the clip is empty");
      entry.animation = animation;
    } catch (error) {
      entry.animation = null;
      onUnavailable(
        name,
        error instanceof Error ? error.message : "failed to load",
      );
    }
  };
  return {
    request(name) {
      if (entries.has(name)) return;
      const entry: Entry = { animation: undefined };
      entries.set(name, entry);
      void load(name, entry);
    },
    take(name) {
      return entries.get(name)?.animation;
    },
  };
}

let shared: MotionLibrary | null = null;
/** One library per page: clips are downloaded and parsed once, whatever remounts. */
export function sharedMotionLibrary(): MotionLibrary {
  shared ??= createMotionLibrary();
  return shared;
}
