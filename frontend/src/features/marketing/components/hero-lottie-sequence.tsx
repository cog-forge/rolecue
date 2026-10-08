"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { DotLottieReact } from "@lottiefiles/dotlottie-react";

export const LOTTIE_STEPS = [
  { id: "recruitment", src: "/lottie/Recruitment.lottie" },
  { id: "interview", src: "/lottie/Interview.lottie" },
  { id: "hired", src: "/lottie/Hired.lottie" },
] as const;

export const ROTATION_INTERVAL_MS = 3800;

export type HeroLottieSequenceProps = {
  reducedMotion?: boolean;
};

function subscribeMediaQuery(callback: () => void) {
  if (typeof window === "undefined" || !window.matchMedia) {
    return () => {};
  }
  const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
  mq.addEventListener?.("change", callback);
  return () => mq.removeEventListener?.("change", callback);
}

function getMediaQuerySnapshot() {
  if (typeof window === "undefined" || !window.matchMedia) {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function getMediaQueryServerSnapshot() {
  return false;
}

const emptySubscribe = () => () => {};

export function HeroLottieSequence({
  reducedMotion,
}: HeroLottieSequenceProps = {}) {
  const motionReduced = useReducedMotion();
  const mediaQueryReduced = useSyncExternalStore(
    subscribeMediaQuery,
    getMediaQuerySnapshot,
    getMediaQueryServerSnapshot,
  );
  const mounted = useSyncExternalStore(
    emptySubscribe,
    () => true,
    () => false,
  );

  const shouldReduceMotion =
    reducedMotion ?? (Boolean(motionReduced) || mediaQueryReduced);

  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (shouldReduceMotion) {
      return;
    }

    const timer = setInterval(() => {
      setIndex((current) => (current + 1) % LOTTIE_STEPS.length);
    }, ROTATION_INTERVAL_MS);

    return () => clearInterval(timer);
  }, [shouldReduceMotion]);

  const activeIndex = shouldReduceMotion ? 1 : index;
  const activeStep = LOTTIE_STEPS[activeIndex];

  return (
    <div
      aria-hidden="true"
      className="relative mx-auto flex aspect-square w-[clamp(24rem,44vw,40rem)] max-w-[570px] min-[1536px]:w-[clamp(24rem,48vw,40rem)] min-[1536px]:max-w-[640px] items-center justify-center pointer-events-none select-none max-[768px]:w-[min(22rem,90vw)]"
      data-active-step={activeStep.id}
      data-testid="hero-lottie-sequence"
    >
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.div
          animate={{ opacity: 1, scale: 1 }}
          className="size-full"
          exit={
            shouldReduceMotion
              ? undefined
              : {
                  opacity: 0,
                  scale: 0.98,
                  transition: { duration: 0.3, ease: [0.22, 1, 0.36, 1] },
                }
          }
          initial={
            shouldReduceMotion
              ? false
              : {
                  opacity: 0,
                  scale: 1.02,
                }
          }
          key={activeStep.id}
          transition={{
            duration: shouldReduceMotion ? 0 : 0.45,
            ease: [0.22, 1, 0.36, 1],
          }}
        >
          {mounted && (
            <DotLottieReact
              autoplay={!shouldReduceMotion}
              className="size-full object-contain"
              loop={!shouldReduceMotion}
              src={activeStep.src}
            />
          )}
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
