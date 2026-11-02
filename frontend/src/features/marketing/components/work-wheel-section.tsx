import { useTranslations } from "next-intl";
import { ContainerTextFlip } from "@/components/ui/container-text-flip";
import { cn } from "@/lib/utils";
import { Reveal } from "./reveal";
import { WorksWheel } from "./works-wheel";
import { MarketingBackdrop } from "./marketing-backdrop";
import {
  landingContainer,
  sectionIntro,
  sectionTitle,
} from "./rolecue-landing-styles";

const moments = [
  { id: "brief", image: "/images/practice-loop/role-context.webp" },
  { id: "setup", image: "/images/practice-loop/practice-space.webp" },
  { id: "answer", image: "/images/practice-loop/conversation.webp" },
  { id: "review", image: "/images/practice-loop/reflection.webp" },
  { id: "next", image: "/images/practice-loop/next-room.webp" },
] as const;

export function WorkWheelSection() {
  const t = useTranslations("public.landing.practiceLoop");
  const items = moments.map((moment) => ({
    title: t(`moments.${moment.id}.title`),
    image: moment.image,
    alt: t(`moments.${moment.id}.alt`),
  }));
  const flipWords = [
    t("flipWords.first"),
    t("flipWords.second"),
    t("flipWords.third"),
  ];
  return (
    <section
      aria-labelledby="work-wheel-title"
      className="relative isolate scroll-mt-26 border-t border-(--rolecue-border-soft) pt-[clamp(6rem,10vw,9rem)] pb-[clamp(4rem,7vw,7rem)]"
      id="practice"
    >
      <MarketingBackdrop layout="practice" />
      <div className={cn(landingContainer, "relative")}>
        <Reveal className="mx-auto max-w-(--rolecue-width-editorial) text-center">
          <h2
            className={cn(sectionTitle, "leading-[1.15]")}
            id="work-wheel-title"
            aria-label={t("ariaLabel")}
          >
            <span className="block">
              {t.rich("titleStart", {
                flip: () => <ContainerTextFlip words={flipWords} />,
              })}
            </span>
            <span className="block">{t("titleEnd")}</span>
          </h2>
          <p className={cn(sectionIntro, "mx-auto")}>{t("intro")}</p>
        </Reveal>
      </div>
      <Reveal className="mt-8 sm:mt-12">
        <WorksWheel items={items} label={t("wheelLabel")} />
      </Reveal>
    </section>
  );
}
