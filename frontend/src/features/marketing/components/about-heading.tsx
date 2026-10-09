"use client";

import { motion, useReducedMotion } from "motion/react";
import { cn } from "@/lib/utils";
import { sectionTitle } from "./rolecue-landing-styles";

const revealEase = [0.22, 1, 0.36, 1] as const;

/** Aceternity's highlight sweep, adapted to RoleCue's blue and scroll entrance. */
export function AboutHeading() {
  const reducedMotion = useReducedMotion();

  return (
    <motion.h2
      className={cn(sectionTitle, "max-w-[15ch]")}
      id="about-rolecue-title"
      initial="rest"
      whileInView="visible"
      viewport={{ once: true, amount: 0.7 }}
    >
      <motion.span
        className="block"
        variants={{
          rest: { y: reducedMotion ? 0 : 12, opacity: 0.7 },
          visible: { y: 0, opacity: 1 },
        }}
        transition={{ duration: reducedMotion ? 0 : 0.65, ease: revealEase }}
      >
        A practice room
      </motion.span>
      <span>for the </span>
      <span className="relative isolate inline-block whitespace-nowrap text-(--rolecue-brand-blue)">
        <motion.span
          aria-hidden="true"
          className="absolute -inset-x-1 bottom-[0.02em] -z-1 h-[0.42em] origin-left -rotate-1 rounded-[0.08em] bg-[rgb(39_102_204/14%)]"
          variants={{
            rest: { scaleX: reducedMotion ? 1 : 0 },
            visible: { scaleX: 1 },
          }}
          transition={{
            delay: reducedMotion ? 0 : 0.35,
            duration: reducedMotion ? 0 : 1.05,
            ease: revealEase,
          }}
        />
        role ahead.
      </span>
    </motion.h2>
  );
}
