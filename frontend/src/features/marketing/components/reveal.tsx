"use client";

import { MotionConfig, motion } from "motion/react";
import { cn } from "@/lib/utils";

type RevealProps = {
  children: React.ReactNode;
  className?: string;
  delay?: number;
};

const revealEase = [0.16, 1, 0.3, 1] as const;

/**
 * A deliberately small client boundary for the landing's once-only viewport
 * entrances. The server still owns all document composition and content.
 */
export function Reveal({ children, className, delay = 0 }: RevealProps) {
  return (
    <MotionConfig reducedMotion="user">
      <motion.div
        className={cn(className)}
        initial={{ opacity: 0, y: 22 }}
        transition={{ delay, duration: 0.88, ease: revealEase }}
        viewport={{ amount: 0.08, margin: "0px 0px -2% 0px", once: true }}
        whileInView={{ opacity: 1, y: 0 }}
      >
        {children}
      </motion.div>
    </MotionConfig>
  );
}
