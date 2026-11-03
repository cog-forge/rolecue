"use client";

import { useCallback, useMemo, useRef } from "react";
import type { VRMAnimation } from "@pixiv/three-vrm-animation";
import type {
  AvatarDirector,
  ClipSlot,
  MouthSample,
  MouthSource,
} from "./avatar-director";

/** Stable bridge for KAN-59 sampling and KAN-251 clips, including model remounts. */
export function useAvatarPerformance() {
  const director = useRef<AvatarDirector | null>(null);
  const mouth = useRef<MouthSource | null>(null);
  const clips = useRef(
    new Map<string, { animation: VRMAnimation; slot: ClipSlot }>(),
  );
  const onDirectorReady = useCallback((instance: AvatarDirector | null) => {
    director.current = instance;
    if (instance)
      for (const [name, clip] of clips.current)
        instance.registerClip(name, clip.animation, clip.slot);
  }, []);
  return useMemo(
    () => ({
      onDirectorReady,
      mouth: {
        sample: (): MouthSample | null => mouth.current?.sample() ?? null,
      },
      setMouthSource: (source: MouthSource | null) => {
        mouth.current = source;
      },
      registerClip: (
        name: string,
        animation: VRMAnimation,
        slot: ClipSlot = "gesture",
      ) => {
        clips.current.set(name, { animation, slot });
        director.current?.registerClip(name, animation, slot);
      },
      triggerGesture: (name: string, fadeSeconds?: number) =>
        director.current?.triggerGesture(name, fadeSeconds) ?? false,
    }),
    [onDirectorReady],
  );
}
