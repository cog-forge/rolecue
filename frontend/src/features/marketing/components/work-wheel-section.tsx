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
  {
    title: "Bring the job brief",
    image: "/images/practice-loop/role-context.webp",
    alt: "A sunlit desk with a laptop and notebook beside an open window.",
  },
  {
    title: "Set up your session",
    image: "/images/practice-loop/practice-space.webp",
    alt: "A bright, welcoming practice room with two chairs and a wooden table.",
  },
  {
    title: "Answer out loud",
    image: "/images/practice-loop/conversation.webp",
    alt: "Two professionals having a thoughtful conversation in a warm, light-filled office.",
  },
  {
    title: "Review the conversation",
    image: "/images/practice-loop/reflection.webp",
    alt: "A person quietly reviewing a notebook beside a laptop in afternoon sunlight.",
  },
  {
    title: "Take it into the interview",
    image: "/images/practice-loop/next-room.webp",
    alt: "A person walking toward an open office doorway filled with warm daylight.",
  },
] as const;

export function WorkWheelSection() {
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
            aria-label="A closer look at the RoleCue loop."
          >
            <span className="block">
              A <ContainerTextFlip /> look
            </span>
            <span className="block"> at the RoleCue loop.</span>
          </h2>
          <p className={cn(sectionIntro, "mx-auto")}>
            Five moments in one rehearsal, from your first job brief to the real
            interview.
          </p>
        </Reveal>
      </div>
      <Reveal className="mt-8 sm:mt-12">
        <WorksWheel items={moments} label="The practice loop" />
      </Reveal>
    </section>
  );
}
