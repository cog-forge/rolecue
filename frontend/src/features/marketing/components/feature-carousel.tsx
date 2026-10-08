"use client";

import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import Image from "next/image";
import { motion, useInView, useReducedMotion } from "motion/react";
import { ArrowLeft, ArrowRight, Pause, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Reveal } from "./reveal";
import { landingFocus } from "./rolecue-landing-styles";

const FEATURES = [
  {
    id: "role-context",
    label: "Job description context",
    description:
      "Technical skills and topics drawn from your target job description.",
    image: "/images/features/role-context.webp",
    alt: "A candidate reviewing a target role brief at a softly lit blue desk",
  },
  {
    id: "spoken-interviews",
    label: "Voice-first conversations",
    description:
      "A spoken interview with an AI interviewer, in your own words.",
    image: "/images/features/spoken-interviews.webp",
    alt: "A candidate speaking thoughtfully toward a laptop during interview practice",
  },
  {
    id: "follow-up-questions",
    label: "Adaptive questions",
    description: "Each question responds to your answer and the role context.",
    image: "/images/features/follow-up-questions.webp",
    alt: "A candidate sketching technical ideas while explaining an approach",
  },
  {
    id: "personal-interviewer",
    label: "Interviewer options",
    description:
      "System or personal 3D avatars, with a voice profile you choose.",
    image: "/images/features/personal-interviewer.webp",
    alt: "An inviting interview workspace with an interviewer portrait on a laptop",
  },
  {
    id: "useful-feedback",
    label: "Session evaluation",
    description: "A post-interview report on your technical responses.",
    image: "/images/features/useful-feedback.webp",
    alt: "Hands reviewing feedback and noting an improvement at a pastel blue desk",
  },
] as const;

const INTERVAL = 6000;
const wrap = (value: number) =>
  ((value % FEATURES.length) + FEATURES.length) % FEATURES.length;
const ease = [0.22, 1, 0.36, 1] as const;

export function FeatureCarousel({ intro }: { intro: ReactNode }) {
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const pendingFocus = useRef(false);
  const inView = useInView(root, { amount: 0.3 });
  const reduced = useReducedMotion();
  const [active, setActive] = useState(0);
  const [paused, setPaused] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [visible, setVisible] = useState(true);
  const playing =
    !paused && !hovered && !focused && !reduced && inView && visible;

  useEffect(() => {
    const read = () => setVisible(document.visibilityState === "visible");
    read();
    document.addEventListener("visibilitychange", read);
    return () => document.removeEventListener("visibilitychange", read);
  }, []);

  useEffect(() => {
    if (!playing) return;
    const timer = window.setInterval(
      () => setActive((value) => wrap(value + 1)),
      INTERVAL,
    );
    return () => window.clearInterval(timer);
  }, [playing]);

  useEffect(() => {
    if (!pendingFocus.current) return;
    tabs.current[active]?.focus({ preventScroll: true });
    pendingFocus.current = false;
  }, [active]);

  const select = (index: number, focus = false) => {
    setPaused(true);
    pendingFocus.current = focus && index !== active;
    if (focus && index === active)
      tabs.current[index]?.focus({ preventScroll: true });
    setActive(index);
  };

  return (
    <div
      ref={root}
      aria-label="Explore RoleCue features"
      className="grid items-center gap-7 sm:gap-9 lg:grid-cols-[1fr_1fr] lg:gap-x-[clamp(3rem,6vw,6rem)] lg:gap-y-0"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onFocusCapture={() => setFocused(true)}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget))
          setFocused(false);
      }}
    >
      <Reveal className="min-w-0 lg:col-start-1 lg:row-start-1">{intro}</Reveal>
      <Reveal
        className="order-2 min-w-0 lg:order-none lg:col-start-1 lg:row-start-2 lg:mt-8"
        delay={0.1}
      >
        <div
          role="tablist"
          aria-label="RoleCue features"
          aria-orientation="vertical"
          onKeyDown={(event) => {
            let next: number;
            if (event.key === "ArrowDown") next = wrap(active + 1);
            else if (event.key === "ArrowUp") next = wrap(active - 1);
            else if (event.key === "Home") next = 0;
            else if (event.key === "End") next = FEATURES.length - 1;
            else return;
            event.preventDefault();
            select(next, true);
          }}
        >
          {FEATURES.map((feature, index) => {
            const selected = index === active;
            return (
              <button
                key={feature.id}
                ref={(node) => {
                  tabs.current[index] = node;
                }}
                id={`${id}-tab-${index}`}
                type="button"
                role="tab"
                aria-selected={selected}
                aria-controls={`${id}-panel-${index}`}
                tabIndex={selected ? 0 : -1}
                onClick={() => select(index)}
                className={cn(
                  "relative flex min-h-14 w-full gap-5 border-l px-5 py-4 text-left transition-[border-color,color,background-color] duration-300 sm:gap-6 sm:px-6",
                  selected
                    ? "border-[#7399ca] bg-[#dce7f3]/45 text-(--rolecue-ink)"
                    : "border-[#c9d6e6] text-(--rolecue-ink-muted) hover:bg-[#dce7f3]/25 hover:text-(--rolecue-ink)",
                  landingFocus,
                )}
              >
                <span
                  className={cn(
                    "pt-0.5 text-xs font-medium",
                    selected
                      ? "text-(--rolecue-brand-blue)"
                      : "text-(--rolecue-ink-subtle)",
                  )}
                  aria-hidden="true"
                >
                  {String(index + 1).padStart(2, "0")}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-base font-semibold sm:text-[1.05rem]">
                    {feature.label}
                  </span>
                  <motion.span
                    initial={false}
                    animate={{
                      height: selected ? "auto" : 0,
                      opacity: selected ? 1 : 0,
                      marginTop: selected ? 9 : 0,
                    }}
                    transition={{ duration: reduced ? 0 : 0.35, ease }}
                    aria-hidden={!selected}
                    className="block overflow-hidden text-[0.95rem] leading-relaxed font-normal text-(--rolecue-ink-muted)"
                  >
                    {feature.description}
                  </motion.span>
                </span>
              </button>
            );
          })}
        </div>
        <div className="mt-5 flex items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Previous feature"
            onClick={() => select(wrap(active - 1))}
            className={cn(
              "size-11 rounded-full text-(--rolecue-ink) hover:bg-[#dce7f3]",
              landingFocus,
            )}
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Next feature"
            onClick={() => select(wrap(active + 1))}
            className={cn(
              "size-11 rounded-full text-(--rolecue-ink) hover:bg-[#dce7f3]",
              landingFocus,
            )}
          >
            <ArrowRight className="size-4" aria-hidden="true" />
          </Button>
          {!reduced && (
            <Button
              variant="ghost"
              size="icon"
              aria-label={
                paused ? "Play feature previews" : "Pause feature previews"
              }
              onClick={() => setPaused((value) => !value)}
              className={cn(
                "size-11 rounded-full text-(--rolecue-ink-muted) hover:bg-[#dce7f3]",
                landingFocus,
              )}
            >
              {paused ? (
                <Play className="size-3.5" aria-hidden="true" />
              ) : (
                <Pause className="size-3.5" aria-hidden="true" />
              )}
            </Button>
          )}
        </div>
      </Reveal>

      <Reveal
        delay={0.15}
        className="order-1 min-w-0 lg:order-none lg:sticky lg:top-28 lg:col-start-2 lg:row-start-1 lg:row-span-2 lg:self-center"
      >
        <div className="relative aspect-[5/4] overflow-clip rounded-[1.5rem] bg-[#dce6f0] sm:aspect-square sm:rounded-[2rem]">
          {FEATURES.map((feature, index) => (
            <motion.div
              key={feature.id}
              id={`${id}-panel-${index}`}
              role="tabpanel"
              aria-labelledby={`${id}-tab-${index}`}
              aria-hidden={index !== active}
              initial={false}
              animate={{
                opacity: index === active ? 1 : 0,
                scale: index === active ? 1 : 1.035,
              }}
              transition={{ duration: reduced ? 0 : 0.65, ease }}
              className="absolute inset-0"
              style={{
                zIndex: index === active ? 1 : 0,
                pointerEvents: index === active ? "auto" : "none",
              }}
            >
              <Image
                src={feature.image}
                alt={feature.alt}
                fill
                sizes="(max-width: 639px) 100vw, (max-width: 1023px) 120vw, 840px"
                className="object-cover"
                draggable={false}
              />
            </motion.div>
          ))}
        </div>
      </Reveal>
    </div>
  );
}
