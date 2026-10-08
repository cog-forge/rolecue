"use client";

import { useEffect, useRef, useState } from "react";
import {
  AnimatePresence,
  motion,
  useInView,
  useReducedMotion,
} from "motion/react";

const defaultWords = ["closer", "deeper", "clearer"];

/** Aceternity's Container Text Flip, adapted for semantic inline text and RoleCue. */
export function ContainerTextFlip({
  words = defaultWords,
}: {
  words?: readonly string[];
}) {
  const containerRef = useRef<HTMLSpanElement>(null);
  const [wordElement, setWordElement] = useState<HTMLSpanElement | null>(null);
  const inView = useInView(containerRef, { amount: 0.8 });
  const reducedMotion = useReducedMotion();
  const [index, setIndex] = useState(0);
  const [width, setWidth] = useState<number>();
  const word = words[index % words.length] ?? "closer";

  useEffect(() => {
    if (!inView || reducedMotion || words.length < 2) return;
    // One cycle per visit, then rest instead of endlessly changing the heading.
    let changes = 0;
    const timer = window.setInterval(() => {
      if (document.hidden) return;
      setIndex((current) => (current + 1) % words.length);
      changes += 1;
      if (changes >= words.length) window.clearInterval(timer);
    }, 1500);
    return () => window.clearInterval(timer);
  }, [inView, reducedMotion, words]);

  useEffect(() => {
    const text = wordElement;
    if (!text) return;
    const measure = () => setWidth(text.getBoundingClientRect().width);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(text);
    return () => observer.disconnect();
  }, [wordElement]);

  return (
    <motion.span
      ref={containerRef}
      aria-hidden="true"
      className="relative box-content inline-block overflow-clip align-baseline rounded-[0.12em] border-b border-[rgb(39_102_204/20%)] bg-[rgb(39_102_204/7%)] px-[0.13em] text-(--rolecue-brand-blue)"
      animate={width === undefined ? undefined : { width }}
      transition={{
        duration: reducedMotion ? 0 : 0.35,
        ease: [0.22, 1, 0.36, 1],
      }}
    >
      <AnimatePresence initial={false} mode="wait">
        <motion.span
          aria-hidden="true"
          className="inline-block whitespace-nowrap"
          key={word}
          ref={setWordElement}
          initial={{
            opacity: 0,
            y: reducedMotion ? 0 : 8,
            rotateX: reducedMotion ? 0 : -30,
          }}
          animate={{ opacity: 1, y: 0, rotateX: 0 }}
          exit={{
            opacity: 0,
            y: reducedMotion ? 0 : -8,
            rotateX: reducedMotion ? 0 : 30,
          }}
          transition={{
            duration: reducedMotion ? 0 : 0.35,
            ease: [0.22, 1, 0.36, 1],
          }}
        >
          {word.split("").map((letter, letterIndex) => (
            <motion.span
              className="inline-block"
              key={`${letterIndex}-${letter}`}
              initial={{
                opacity: 0,
                filter: reducedMotion ? "none" : "blur(4px)",
              }}
              animate={{ opacity: 1, filter: "blur(0px)" }}
              transition={{
                duration: reducedMotion ? 0 : 0.3,
                delay: reducedMotion ? 0 : letterIndex * 0.025,
              }}
            >
              {letter}
            </motion.span>
          ))}
        </motion.span>
      </AnimatePresence>
    </motion.span>
  );
}
