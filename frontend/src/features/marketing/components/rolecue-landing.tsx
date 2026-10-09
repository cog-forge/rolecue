import Link from "next/link";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { AboutRolecueSection } from "./about-rolecue-section";
import { FaqSection } from "./faq-section";
import { ClosingCtaSection } from "./closing-cta-section";
import { LandingHero } from "./landing-hero";
import { MarketingHeader } from "./marketing-header";
import { WorkWheelSection } from "./work-wheel-section";
import { HeroCursorGlow } from "./hero-cursor-glow";
import { landingContainer, landingFocus } from "./rolecue-landing-styles";

export function RoleCueLanding() {
  return (
    <div
      data-rolecue-theme="light"
      className="relative isolate min-w-0 overflow-x-clip bg-(--rolecue-canvas) text-(--rolecue-ink) font-(--rolecue-weight-body) leading-(--rolecue-leading-body)"
    >
      <MarketingHeader />
      <main id="main-content">
        <LandingHero />
        <AboutRolecueSection />

        <WorkWheelSection />

        <FaqSection />

        <ClosingCtaSection />
      </main>
      <HeroCursorGlow />
      <footer className="border-t border-(--rolecue-border) bg-[color-mix(in_srgb,var(--rolecue-hero-surface)_14%,var(--rolecue-canvas))] pt-16 pb-8 max-[760px]:pt-12">
        <div className={landingContainer}>
          <div className="grid grid-cols-[1fr_auto] items-end gap-x-12 gap-y-8 max-[760px]:grid-cols-1">
            <div>
              <p className="mt-0 mb-5 text-xs font-medium tracking-[0.12em] text-(--rolecue-ink-muted) uppercase">
                Role-focused practice
              </p>
              <p className="m-0 text-[clamp(4.25rem,8.5vw,8.25rem)] leading-[0.88] tracking-[-0.065em] font-(--rolecue-weight-display)">
                Role<span className="text-(--rolecue-brand-blue)">Cue.</span>
              </p>
            </div>
            <nav
              aria-label="Footer navigation"
              className="flex flex-wrap justify-end gap-x-6 gap-y-2 text-[0.95rem] font-medium text-(--rolecue-ink-muted) max-[760px]:grid max-[760px]:grid-cols-2 max-[760px]:justify-start"
            >
              <a
                className={cn(
                  landingFocus,
                  "inline-flex min-h-11 min-w-11 items-center border-b border-transparent no-underline transition-[color,border-color] duration-(--duration-fast) hover:border-(--rolecue-brand-blue) hover:text-(--rolecue-brand-blue) focus-visible:text-(--rolecue-brand-blue) max-[760px]:w-fit motion-reduce:transition-none!",
                )}
                href="#about"
              >
                Features
              </a>
              <a
                className={cn(
                  landingFocus,
                  "inline-flex min-h-11 min-w-11 items-center border-b border-transparent no-underline transition-[color,border-color] duration-(--duration-fast) hover:border-(--rolecue-brand-blue) hover:text-(--rolecue-brand-blue) focus-visible:text-(--rolecue-brand-blue) max-[760px]:w-fit motion-reduce:transition-none!",
                )}
                href="#practice"
              >
                How it works
              </a>
              <a
                className={cn(
                  landingFocus,
                  "inline-flex min-h-11 min-w-11 items-center border-b border-transparent no-underline transition-[color,border-color] duration-(--duration-fast) hover:border-(--rolecue-brand-blue) hover:text-(--rolecue-brand-blue) focus-visible:text-(--rolecue-brand-blue) max-[760px]:w-fit motion-reduce:transition-none!",
                )}
                href="#questions"
              >
                Questions
              </a>
              <Link
                className={cn(
                  landingFocus,
                  "inline-flex min-h-11 min-w-11 items-center border-b border-transparent no-underline transition-[color,border-color] duration-(--duration-fast) hover:border-(--rolecue-brand-blue) hover:text-(--rolecue-brand-blue) focus-visible:text-(--rolecue-brand-blue) max-[760px]:w-fit motion-reduce:transition-none!",
                )}
                href={routes.interviews.new.jobDescription}
              >
                Start
              </Link>
            </nav>
          </div>
          <div className="mt-12 flex items-center justify-between gap-x-6 gap-y-3 border-t border-(--rolecue-border) pt-6 text-sm leading-relaxed text-(--rolecue-ink-muted) max-[760px]:mt-8 max-[760px]:flex-col max-[760px]:items-start">
            <span>© 2026 RoleCue</span>
            <span>Made for clearer technical conversations</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
