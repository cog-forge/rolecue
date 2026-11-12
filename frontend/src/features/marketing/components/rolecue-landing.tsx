import { useTranslations } from "next-intl";
import { AboutRolecueSection } from "./about-rolecue-section";
import { FaqSection } from "./faq-section";
import { ClosingCtaSection } from "./closing-cta-section";
import { LandingHero } from "./landing-hero";
import { MarketingHeader } from "./marketing-header";
import { WorkWheelSection } from "./work-wheel-section";
import { HeroCursorGlow } from "./hero-cursor-glow";
import { landingContainer } from "./rolecue-landing-styles";

export function RoleCueLanding() {
  const t = useTranslations("public.landing");
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
          <div className="block">
            <div>
              <p className="mt-0 mb-5 text-xs font-medium tracking-[0.12em] text-(--rolecue-ink-muted) uppercase">
                {t("focus")}
              </p>
              <p className="m-0 text-[clamp(4.25rem,8.5vw,8.25rem)] leading-[0.88] tracking-[-0.065em] font-(--rolecue-weight-display)">
                Role<span className="text-(--rolecue-brand-blue)">Cue.</span>
              </p>
            </div>
          </div>
          <div className="mt-12 flex items-center justify-between gap-x-6 gap-y-3 border-t border-(--rolecue-border) pt-6 text-sm leading-relaxed text-(--rolecue-ink-muted) max-[760px]:mt-8 max-[760px]:flex-col max-[760px]:items-start">
            <span>© 2026 RoleCue</span>
            <span>{t("madeFor")}</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
