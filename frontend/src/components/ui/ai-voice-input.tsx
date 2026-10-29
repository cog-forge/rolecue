"use client";

/**
 * Adapted from Kokonut UI AI Voice (MIT), Copyright (c) 2025 kokonutUI.
 * https://github.com/kokonut-labs/kokonutui/blob/main/components/kokonutui/ai-voice.tsx
 * Keeps the scrolling waveform engine; controlled by real input levels instead of its demo loop.
 * License: docs/LICENSE-KokonutUI.txt
 */
import { useEffect, useRef } from "react";
import { useReducedMotion } from "motion/react";
import { cn } from "@/lib/utils";

const BAR_COUNT = 32;
const BAR_IDS = Array.from(
  { length: BAR_COUNT + 1 },
  (_, i) => `voice-bar-${i}`,
);
const BAR_PITCH = 5;
const SAMPLE_MS = 70;
const REST_LEVEL = 0.06;
const EDGE_MASK =
  "linear-gradient(to right, transparent, black 12%, black 88%, transparent)";
function setScales(
  bars: (HTMLSpanElement | null)[],
  levels: readonly number[],
) {
  bars.forEach((bar, i) => {
    if (bar)
      bar.style.transform = `scaleY(${(levels[i] ?? REST_LEVEL).toFixed(3)})`;
  });
}
export function AIVoiceInput({
  active,
  level = 0,
  className,
}: {
  active: boolean;
  level?: number;
  className?: string;
}) {
  const reduced = useReducedMotion();
  const root = useRef<HTMLDivElement>(null);
  const row = useRef<HTMLDivElement>(null);
  const bars = useRef<(HTMLSpanElement | null)[]>([]);
  const input = useRef(level);
  useEffect(() => {
    input.current = level;
    if (reduced)
      setScales(
        bars.current,
        BAR_IDS.map(() =>
          active ? Math.max(REST_LEVEL, Math.min(1, level)) : REST_LEVEL,
        ),
      );
  }, [level, active, reduced]);
  useEffect(() => {
    const element = root.current;
    const track = row.current;
    if (!element || !track) return;
    if (!active) {
      track.style.transform = "translateX(0px)";
      setScales(
        bars.current,
        BAR_IDS.map(() => REST_LEVEL),
      );
      return;
    }
    if (reduced) return;
    const samples = BAR_IDS.map(() => REST_LEVEL);
    let phase = 0,
      last = performance.now(),
      frame = 0,
      visible = true;
    const tick = (now: number) => {
      const dt = Math.min(now - last, 100);
      last = now;
      phase += dt / SAMPLE_MS;
      while (phase >= 1) {
        phase -= 1;
        samples.shift();
        samples.push(Math.max(REST_LEVEL, Math.min(1, input.current)));
      }
      track.style.transform = `translateX(${(-phase * BAR_PITCH).toFixed(2)}px)`;
      setScales(bars.current, samples);
      if (visible && !document.hidden) frame = requestAnimationFrame(tick);
    };
    const start = () => {
      cancelAnimationFrame(frame);
      if (visible && !document.hidden) {
        last = performance.now();
        frame = requestAnimationFrame(tick);
      }
    };
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      start();
    });
    observer.observe(element);
    document.addEventListener("visibilitychange", start);
    start();
    return () => {
      observer.disconnect();
      document.removeEventListener("visibilitychange", start);
      cancelAnimationFrame(frame);
    };
  }, [active, reduced]);
  return (
    <div
      ref={root}
      role="meter"
      aria-label="Microphone input level"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={active ? Math.round(level * 100) : 0}
      aria-valuetext={
        active ? "Local microphone active" : "Microphone inactive"
      }
      className={cn("relative h-7 w-full overflow-hidden", className)}
      style={{ maskImage: EDGE_MASK, WebkitMaskImage: EDGE_MASK }}
    >
      <div ref={row} aria-hidden className="flex h-full items-center gap-[2px]">
        {BAR_IDS.map((id, i) => (
          <span
            key={id}
            ref={(element) => {
              bars.current[i] = element;
            }}
            className={cn(
              "h-full w-[3px] shrink-0 rounded-full",
              active ? "bg-white/85" : "bg-white/30",
            )}
            style={{ transform: `scaleY(${REST_LEVEL})` }}
          />
        ))}
      </div>
    </div>
  );
}
