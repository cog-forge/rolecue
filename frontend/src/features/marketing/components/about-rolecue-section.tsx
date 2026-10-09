import { cn } from "@/lib/utils";
import { AboutHeading } from "./about-heading";
import { FeatureCarousel } from "./feature-carousel";
import { MarketingBackdrop } from "./marketing-backdrop";
import { eyebrow, landingContainer } from "./rolecue-landing-styles";

export function AboutRolecueSection() {
  return (
    <section
      aria-labelledby="about-rolecue-title"
      className="relative pt-[clamp(8rem,12vw,12rem)] pb-[clamp(5rem,8vw,8rem)]"
    >
      <MarketingBackdrop />
      <div className={cn(landingContainer, "relative scroll-mt-28")} id="about">
        <FeatureCarousel
          intro={
            <>
              <p className={eyebrow}>Meet RoleCue</p>
              <AboutHeading />
              <p className="mt-6 mb-0 max-w-[42ch] text-pretty text-(length:--rolecue-type-body-lg) leading-(--rolecue-leading-copy) text-(--rolecue-ink-muted)">
                RoleCue combines job context, spoken conversation and interview
                evaluation in one private practice space.
              </p>
            </>
          }
        />
      </div>
    </section>
  );
}
