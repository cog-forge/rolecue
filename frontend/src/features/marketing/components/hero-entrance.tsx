"use client";

import { useEffect } from "react";
import {
  motion,
  useAnimationControls,
  useReducedMotion,
  type HTMLMotionProps,
} from "motion/react";
import { cn } from "@/lib/utils";

const easeOut = [0.16, 1, 0.3, 1] as const;

type HeroEntranceProps = Omit<
  HTMLMotionProps<"div">,
  "animate" | "children" | "initial"
> & {
  children: React.ReactNode;
  delay?: number;
  surface?: boolean;
};

/** Keeps server-rendered content visible, then plays the entrance once hydrated. */
export function HeroEntrance({
  children,
  className,
  delay = 0,
  surface = false,
  ...props
}: HeroEntranceProps) {
  const controls = useAnimationControls();
  const reducedMotion = useReducedMotion();

  useEffect(() => {
    if (reducedMotion) return;

    controls.set(
      surface ? { opacity: 0, scaleY: 0.94 } : { opacity: 0, y: 24 },
    );
    void controls.start({
      opacity: 1,
      y: 0,
      scaleY: 1,
      transition: {
        delay,
        duration: surface ? 1.15 : 0.82,
        ease: easeOut,
      },
    });
  }, [controls, delay, reducedMotion, surface]);

  return (
    <motion.div
      animate={controls}
      className={cn(surface && "origin-top", className)}
      initial={false}
      {...props}
    >
      {children}
    </motion.div>
  );
}
