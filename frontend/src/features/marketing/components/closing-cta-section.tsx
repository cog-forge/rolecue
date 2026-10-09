"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { landingContainer, landingFocus } from "./rolecue-landing-styles";

const ease = [0.22, 1, 0.36, 1] as const;

/** A single light opens above the final invitation when it enters the viewport. */
export function ClosingCtaSection() {
  const reducedMotion = useReducedMotion();
  const lighting = {
    dim: { opacity: 0, scaleX: 0.48 },
    lit: { opacity: 1, scaleX: 1 },
  };

  return (
    <motion.section
      id="start"
      aria-labelledby="closing-cta-title"
      initial={reducedMotion ? false : "dim"}
      whileInView="lit"
      viewport={{ once: true, amount: 0.5, margin: "0px 0px -10% 0px" }}
      transition={{ duration: reducedMotion ? 0 : 1.2, ease }}
      className="relative isolate flex min-h-[clamp(36rem,70svh,44rem)] scroll-mt-20 items-center justify-center overflow-hidden py-24 max-[760px]:min-h-144"
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 -z-10 overflow-hidden"
      >
        <motion.div
          variants={{ dim: { opacity: 0 }, lit: { opacity: 1 } }}
          transition={{ duration: reducedMotion ? 0 : 1.35, ease }}
          className="absolute inset-0 bg-[radial-gradient(ellipse_70%_65%_at_50%_28%,#e5eef7_0%,transparent_78%)]"
        />
        <motion.div
          variants={lighting}
          transition={{ duration: reducedMotion ? 0 : 1.35, ease }}
          className="absolute top-16 left-1/2 h-72 w-[min(66rem,140%)] -translate-x-1/2 [mask-image:linear-gradient(to_bottom,black_20%,transparent)] max-[760px]:top-12 max-[760px]:h-64"
        >
          <div className="absolute top-0 right-1/2 h-full w-1/2 bg-[conic-gradient(from_70deg_at_center_top,transparent_0deg,#b3cde8_20deg,transparent_110deg)] [mask-image:linear-gradient(to_right,transparent,black_65%)]" />
          <div className="absolute top-0 left-1/2 h-full w-1/2 bg-[conic-gradient(from_290deg_at_center_top,transparent_0deg,transparent_250deg,#b3cde8_340deg,transparent_360deg)] [mask-image:linear-gradient(to_left,transparent,black_65%)]" />
        </motion.div>
        <motion.div
          variants={lighting}
          transition={{ duration: reducedMotion ? 0 : 1.1, ease }}
          className="absolute top-12 left-1/2 h-40 w-[min(34rem,78%)] -translate-x-1/2 rounded-[50%] bg-[#bed5ed]/85 blur-[46px] max-[760px]:top-8 max-[760px]:h-32"
        />
        <motion.div
          variants={lighting}
          transition={{ duration: reducedMotion ? 0 : 1, ease }}
          className="absolute top-16 left-1/2 h-px w-[min(32rem,68%)] -translate-x-1/2 bg-[#8eafd2] shadow-[0_2px_16px_2px_#acc9e7] max-[760px]:top-12"
        />
      </div>

      <motion.div
        variants={{
          dim: { opacity: 0, y: 36 },
          lit: { opacity: 1, y: 0 },
        }}
        transition={{
          duration: reducedMotion ? 0 : 0.9,
          delay: reducedMotion ? 0 : 0.18,
          ease,
        }}
        className={cn(landingContainer, "pt-16 text-center max-[760px]:pt-12")}
      >
        <h2
          id="closing-cta-title"
          className="mx-auto m-0 max-w-[16ch] text-balance text-[clamp(2.7rem,6vw,5.5rem)] leading-[1.04] tracking-[-0.055em] font-(--rolecue-weight-heading)"
        >
          Your next interview
          <br />
          starts here.
        </h2>
        <p className="mx-auto mt-6 mb-0 max-w-[34ch] text-balance text-[clamp(1.05rem,1.7vw,1.35rem)] leading-relaxed text-(--rolecue-ink-muted)">
          A little practice before the real conversation.
        </p>
        <Button
          asChild
          className={cn(
            landingFocus,
            "group/closing relative mt-9 h-16 min-w-72 gap-5 overflow-hidden rounded-2xl border border-white/10 bg-(--rolecue-button-primary) px-8 text-lg font-semibold text-(--rolecue-on-primary) shadow-[0_6px_20px_-8px_rgb(44_68_98/45%)] transition-[transform,background-color,border-color,box-shadow] duration-(--rolecue-cta-duration) ease-(--ease-smooth-out) hover:scale-[1.02] hover:border-[#b3cde8]/60 hover:bg-(--rolecue-button-primary-hover) hover:shadow-[0_8px_24px_-8px_rgb(44_68_98/45%)] focus-visible:scale-[1.02] active:scale-[0.98] active:duration-(--rolecue-cta-press-duration) motion-reduce:transform-none! motion-reduce:transition-none!",
          )}
        >
          <Link href={routes.register} aria-label="Start for free">
            <span
              aria-hidden="true"
              className="relative block h-7 overflow-hidden leading-7"
            >
              <span className="block transition-transform duration-(--duration-slow) ease-(--ease-smooth-out) group-hover/closing:-translate-y-full group-focus-visible/closing:-translate-y-full motion-reduce:transform-none! motion-reduce:transition-none!">
                <span className="block">Start for free</span>
                <span className="absolute top-full left-0 block">
                  Start for free
                </span>
              </span>
            </span>
            <ArrowRight
              aria-hidden="true"
              className="size-5 shrink-0 transition-transform duration-(--duration-slow) ease-(--ease-smooth-out) group-hover/closing:-rotate-45 group-focus-visible/closing:-rotate-45 motion-reduce:transform-none! motion-reduce:transition-none!"
              strokeWidth={1.7}
            />
          </Link>
        </Button>
      </motion.div>
    </motion.section>
  );
}
