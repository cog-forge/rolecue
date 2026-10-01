"use client";

import { useEffect } from "react";
import { motion, stagger, useAnimate, useReducedMotion } from "motion/react";
import { cn } from "@/lib/utils";

type TextGenerateEffectProps = {
  words: string;
  className?: string;
  filter?: boolean;
  duration?: number;
};

export function TextGenerateEffect({
  words,
  className,
  filter = true,
  duration = 0.5,
}: TextGenerateEffectProps) {
  const [scope, animate] = useAnimate();
  const shouldReduceMotion = useReducedMotion();

  useEffect(() => {
    const animation = animate(
      "[data-generate-word]",
      {
        opacity: 1,
        filter: filter ? "blur(0px)" : "none",
      },
      {
        duration: shouldReduceMotion ? 0 : duration,
        delay: stagger(shouldReduceMotion ? 0 : 0.12),
      },
    );

    return () => animation.stop();
  }, [animate, duration, filter, shouldReduceMotion, words]);

  return (
    <motion.span
      aria-hidden="true"
      className={cn("block", className)}
      ref={scope}
    >
      {words.split(" ").map((word, index, allWords) => (
        <span key={`${word}-${index}`}>
          <motion.span
            className="inline-block opacity-0"
            data-generate-word
            style={{ filter: filter ? "blur(8px)" : "none" }}
          >
            {word}
          </motion.span>
          {index < allWords.length - 1 ? " " : null}
        </span>
      ))}
    </motion.span>
  );
}
