import { useId } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { HeroLottieSequence } from "./hero-lottie-sequence";
import { HeroEntrance } from "./hero-entrance";
import { landingFocus } from "./rolecue-landing-styles";

const headline =
  "block text-[clamp(3rem,12vw,6rem)] min-[1024px]:text-[clamp(4rem,7.6vw,8rem)] font-extrabold uppercase leading-[0.92] tracking-[-0.045em] [font-variation-settings:'wght'_850]";

const heroShape =
  "M 0 0 L 1440 0 L 1440 615 C 1440 638, 1422 655, 1395 655 L 1060 655 C 980 655, 930 740, 885 815 C 850 875, 820 900, 775 900 L 665 900 C 620 900, 590 875, 555 815 C 510 740, 460 655, 380 655 L 45 655 C 18 655, 0 638, 0 615 Z";

export function LandingHero() {
  const surfaceClipId = useId();

  return (
    <section
      aria-label="RoleCue hero"
      className="relative isolate flex min-h-svh flex-col bg-(--rolecue-canvas) px-3 pt-3 sm:px-5 sm:pt-4 lg:px-6 lg:pt-5"
      id="top"
    >
      <div
        className="relative isolate flex min-h-[calc(100svh-0.75rem)] flex-1 flex-col overflow-hidden rounded-[2rem] bg-(--rolecue-canvas) sm:min-h-[calc(100svh-1rem)] sm:rounded-[2.5rem] lg:min-h-[calc(100svh-1.25rem)] lg:rounded-[3rem]"
        data-hero-shape={heroShape}
      >
        {/* The lower taper leaves two neutral pockets for supporting copy. */}
        <HeroEntrance
          className="pointer-events-none absolute inset-0 hidden size-full min-[1024px]:block"
          surface
        >
          <svg
            aria-hidden="true"
            className="size-full"
            preserveAspectRatio="none"
            viewBox="0 0 1440 900"
          >
            <defs>
              <clipPath id={surfaceClipId} clipPathUnits="objectBoundingBox">
                <path
                  d={heroShape}
                  transform="scale(0.0006944444 0.0011111111)"
                />
              </clipPath>
            </defs>
            <path d={heroShape} fill="var(--rolecue-hero-surface)" />
          </svg>
        </HeroEntrance>
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 top-0 h-[calc(100%-12rem)] rounded-b-[2.5rem] bg-(--rolecue-hero-surface) min-[1024px]:hidden"
        />

        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 hidden overflow-hidden select-none min-[1024px]:block"
          style={{ clipPath: `url(#${surfaceClipId})` }}
        >
          <span className="absolute -right-8 bottom-[17%] text-[clamp(10rem,17vw,18rem)] leading-none font-black tracking-[-0.05em] text-(--rolecue-brand-blue)/[0.055] uppercase">
            READY.
          </span>
        </div>

        <div className="h-20 shrink-0 sm:h-24" aria-hidden="true" />

        <div className="relative flex flex-1 flex-col justify-center">
          <h1 className="sr-only">Walk in ready.</h1>
          <HeroEntrance
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 hidden overflow-hidden select-none min-[1024px]:block"
            delay={0.18}
          >
            <span className="absolute top-0 left-8 text-[clamp(8rem,15vw,15rem)] leading-none font-black tracking-[-0.05em] text-(--rolecue-ink)/[0.035] uppercase">
              WALK IN
            </span>
          </HeroEntrance>

          <div
            aria-hidden="true"
            className="relative flex w-full flex-col items-center justify-center pt-8 text-center select-none min-[1024px]:min-h-[clamp(26rem,48vh,35rem)] min-[1024px]:flex-row min-[1024px]:justify-between min-[1024px]:pt-0"
          >
            <HeroEntrance
              className="z-10 flex flex-col items-center min-[1024px]:-translate-y-16 min-[1024px]:items-start min-[1024px]:pl-[clamp(2rem,4vw,4.5rem)]"
              delay={0.2}
            >
              <span className={cn(headline, "text-(--rolecue-ink)")}>
                WALK IN
              </span>
              <div className="relative mt-8 hidden w-48 border-b border-white/90 pb-3 text-left min-[1180px]:block">
                <span className="block text-base font-semibold text-(--rolecue-ink)">
                  100% Role-Based
                </span>
                <span className="mt-1 block text-sm text-(--rolecue-ink-muted)">
                  Built around your job description
                </span>
                <svg
                  className="pointer-events-none absolute top-1/2 left-full h-12 w-28 -translate-y-1/2"
                  viewBox="0 0 112 48"
                  fill="none"
                >
                  <path
                    d="M 0 24 H 72 L 101 8"
                    stroke="white"
                    strokeOpacity="0.9"
                  />
                  <circle cx="105" cy="6" r="4" fill="white" />
                  <circle
                    cx="105"
                    cy="6"
                    r="6.5"
                    stroke="white"
                    strokeOpacity="0.65"
                  />
                </svg>
              </div>
            </HeroEntrance>

            <div className="pointer-events-none relative z-20 my-1 min-[1024px]:absolute min-[1024px]:top-1/2 min-[1024px]:left-1/2 min-[1024px]:my-0 min-[1024px]:-translate-x-1/2 min-[1024px]:-translate-y-[60%]">
              <HeroLottieSequence />
            </div>

            <HeroEntrance
              className="z-10 flex flex-col items-center min-[1024px]:translate-y-14 min-[1024px]:items-end min-[1024px]:pr-[clamp(2rem,4vw,4.5rem)] min-[1024px]:text-right"
              delay={0.42}
            >
              <div className="relative mb-8 hidden w-44 border-b border-white/90 pb-3 min-[1180px]:block">
                <span className="block text-base font-semibold text-(--rolecue-ink)">
                  Instant Feedback
                </span>
                <span className="mt-1 block text-sm text-(--rolecue-ink-muted)">
                  A clearer next response
                </span>
                <svg
                  className="pointer-events-none absolute top-1/2 right-full h-12 w-28 -translate-y-1/2"
                  viewBox="0 0 112 48"
                  fill="none"
                >
                  <path
                    d="M 112 24 H 40 L 11 40"
                    stroke="white"
                    strokeOpacity="0.9"
                  />
                  <circle cx="7" cy="42" r="4" fill="white" />
                  <circle
                    cx="7"
                    cy="42"
                    r="6.5"
                    stroke="white"
                    strokeOpacity="0.65"
                  />
                </svg>
              </div>
              <span className={cn(headline, "text-(--rolecue-brand-blue)")}>
                READY.
              </span>
            </HeroEntrance>
          </div>
        </div>

        <div className="relative z-30 grid grid-cols-2 items-end gap-x-6 gap-y-12 px-5 pt-8 pb-8 sm:px-8 min-[1024px]:grid-cols-[1fr_auto_1fr] min-[1024px]:gap-8 min-[1024px]:px-[clamp(2rem,4vw,4.5rem)] min-[1024px]:pt-4">
          <HeroEntrance
            className="relative order-2 w-full max-w-60 border-t border-(--rolecue-border) pt-4 min-[1024px]:-top-6 min-[1024px]:order-1"
            delay={0.58}
          >
            <p className="m-0 text-sm font-semibold tracking-tight text-(--rolecue-ink) sm:text-base">
              Real interview practice
            </p>
            <p className="mt-2 mb-0 max-w-[25ch] text-sm leading-relaxed text-(--rolecue-ink-muted)">
              Get feedback, build confidence, and perform better.
            </p>
          </HeroEntrance>

          <HeroEntrance
            className="order-1 col-span-2 flex flex-col items-center text-center min-[1024px]:order-2 min-[1024px]:col-span-1"
            delay={0.7}
          >
            <p className="m-0 text-base font-medium tracking-tight text-(--rolecue-ink) sm:text-lg">
              Practice the role before the room.
            </p>
            <Button
              asChild
              className={cn(
                landingFocus,
                "group relative mt-5 h-16 w-64 justify-between gap-6 rounded-full border-white/30 bg-(--rolecue-button-primary) py-2 pr-2 pl-7 text-base font-semibold text-(--rolecue-on-primary) shadow-[0_8px_18px_-10px_rgb(0_0_0/35%)] transition-[transform,translate,scale,background-color,box-shadow] duration-(--rolecue-cta-duration) ease-(--ease-smooth-out) hover:-translate-y-0.5 hover:bg-(--rolecue-button-primary-hover) hover:shadow-[0_12px_22px_-10px_rgb(0_0_0/40%)] focus-visible:-translate-y-0.5 active:translate-y-0! active:scale-[0.98] active:duration-(--rolecue-press-duration) motion-reduce:scale-none! motion-reduce:translate-none! motion-reduce:transition-none!",
              )}
            >
              <Link href={routes.interviews.new.jobDescription}>
                <span className="transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover:translate-x-0.5 group-focus-visible:translate-x-0.5 motion-reduce:translate-none! motion-reduce:transition-none!">
                  Start now
                </span>
                <span
                  aria-hidden="true"
                  className="relative grid size-12 shrink-0 place-items-center overflow-hidden rounded-full bg-(--rolecue-on-primary) text-(--rolecue-button-primary) transition-colors duration-(--duration-fast) group-hover:bg-(--rolecue-hero-surface) group-hover:text-(--rolecue-ink) group-focus-visible:bg-(--rolecue-hero-surface) group-focus-visible:text-(--rolecue-ink) motion-reduce:transition-none!"
                >
                  <span className="absolute inset-0 grid place-items-center transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover:translate-x-full group-focus-visible:translate-x-full motion-reduce:translate-none! motion-reduce:transition-none!">
                    <ArrowRight className="size-5" strokeWidth={1.7} />
                  </span>
                  <span className="absolute inset-0 grid -translate-x-full place-items-center transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover:translate-x-0 group-focus-visible:translate-x-0 motion-reduce:hidden">
                    <ArrowRight className="size-5" strokeWidth={1.7} />
                  </span>
                </span>
              </Link>
            </Button>
          </HeroEntrance>

          <HeroEntrance
            className="relative order-3 w-full max-w-60 justify-self-end border-t border-(--rolecue-border) pt-4 min-[1024px]:-top-6"
            delay={0.66}
          >
            <p className="m-0 text-xs font-medium tracking-[0.12em] text-(--rolecue-ink-muted) uppercase">
              ROLE-BASED
            </p>
            <p className="mt-2 mb-0 max-w-[25ch] text-sm leading-relaxed text-(--rolecue-ink) sm:text-base">
              Practice from the actual job requirements.
            </p>
          </HeroEntrance>
        </div>
      </div>
    </section>
  );
}
