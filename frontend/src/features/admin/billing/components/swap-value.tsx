"use client";

import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { cn } from "@/lib/utils";

// A figure that changes in place when the data behind it changes (a new period): the old value fades
// out and the new one resolves through a 2px blur, both at once, without moving.
export function SwapValue({
  value,
  className,
}: {
  value: string;
  className?: string;
}) {
  const reduceMotion = useReducedMotion();
  return (
    <span className={cn("inline-grid", className)}>
      <AnimatePresence initial={false} mode="popLayout">
        <motion.span
          key={value}
          className="col-start-1 row-start-1"
          initial={reduceMotion ? false : { opacity: 0, filter: "blur(2px)" }}
          animate={{ opacity: 1, filter: "blur(0px)" }}
          exit={reduceMotion ? undefined : { opacity: 0, filter: "blur(2px)" }}
          transition={{ duration: 0.15, ease: "easeInOut" }}
        >
          {value}
        </motion.span>
      </AnimatePresence>
    </span>
  );
}
