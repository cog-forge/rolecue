import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { FaqSection } from "./faq-section";
import { MarketingHeader } from "./marketing-header";
import { PracticeSection } from "./practice-section";
import { Reveal } from "./reveal";
import {
  actionIcon,
  eyebrow,
  landingContainer,
  landingFocus,
  primaryButton,
  secondaryButton,
  sectionIntro,
  sectionTitle,
} from "./rolecue-landing-styles";

const processSteps = [
  {
    number: "01",
    title: "Name the role",
    detail: "Give the conversation its real context before you rehearse it.",
  },
  {
    number: "02",
    title: "Find the example",
    detail: "Make a specific moment do the work of a broad claim.",
  },
  {
    number: "03",
    title: "Keep the cue",
    detail: "Carry the useful adjustment into the next attempt.",
  },
] as const;

const workingModes = [
  {
    number: "01 / Context",
    title: "Start with the work that matters.",
    shape: "square",
  },
  {
    number: "02 / Rehearsal",
    title: "Make room for the answer to change.",
    shape: "circle",
  },
  {
    number: "03 / Review",
    title: "Leave with a better next response.",
    shape: "line",
  },
] as const;

function Arrow() {
  return (
    <ArrowUpRight
      aria-hidden="true"
      className={actionIcon}
      size={16}
      strokeWidth={1.8}
    />
  );
}

function HeroMedia() {
  return (
    <Reveal>
      <figure
        aria-label="A RoleCue editorial practice workspace"
        className="relative mx-auto mt-[3.3rem] aspect-video w-[min(67.5rem,92vw)] overflow-hidden rounded-2xl border border-(--rolecue-border) bg-(--rolecue-wash-neutral) shadow-(--rolecue-shadow-media) max-[1080px]:aspect-3/2 max-[760px]:mt-11 max-[760px]:w-[calc(100%-2.5rem)] max-[620px]:aspect-4/5"
      >
        <div
          aria-hidden="true"
          className="absolute inset-0 bg-[radial-gradient(circle_at_72%_28%,var(--rolecue-wash-pink),transparent_25%),radial-gradient(circle_at_22%_70%,var(--rolecue-wash-blue),transparent_35%),linear-gradient(130deg,var(--rolecue-surface-soft)_8%,var(--rolecue-wash-neutral)_56%,var(--rolecue-surface-soft))]"
        />
        <div
          aria-hidden="true"
          className="absolute inset-0 opacity-35 bg-[linear-gradient(rgb(var(--rolecue-ink-rgb)/12%)_1px,transparent_1px),linear-gradient(90deg,rgb(var(--rolecue-ink-rgb)/12%)_1px,transparent_1px)] bg-size-[4.8rem_4.8rem] mask-[linear-gradient(135deg,#000,transparent_74%)]"
        />
        <div className="absolute inset-[clamp(1rem,2.8vw,2rem)] grid grid-cols-[clamp(3rem,8vw,5.1rem)_1fr] overflow-hidden rounded-xl border border-white/72 bg-[color-mix(in_srgb,var(--rolecue-surface)_77%,transparent)] shadow-[inset_0_1px_rgb(255_255_255/90%),0_1.1rem_3rem_rgb(var(--rolecue-ink-rgb)/15%)] backdrop-blur-[14px] max-[620px]:grid-cols-[2.8rem_1fr]">
          <aside
            aria-hidden="true"
            className="flex flex-col items-center gap-4 border-r border-[rgb(var(--rolecue-ink-rgb)/10%)] bg-[color-mix(in_srgb,var(--rolecue-surface)_48%,transparent)] pt-4 max-[620px]:gap-3"
          >
            <div className="mb-[0.8rem] flex gap-1">
              <i className="size-[0.28rem] rounded-full bg-[rgb(var(--rolecue-ink-rgb)/26%)]" />
              <i className="size-[0.28rem] rounded-full bg-[rgb(var(--rolecue-ink-rgb)/26%)]" />
              <i className="size-[0.28rem] rounded-full bg-[rgb(var(--rolecue-ink-rgb)/26%)]" />
            </div>
            <span className="h-[0.18rem] w-[1.45rem] rounded-full bg-(--rolecue-accent)" />
            <span className="h-[0.18rem] w-[1.15rem] rounded-full bg-[rgb(var(--rolecue-ink-rgb)/18%)]" />
            <span className="h-[0.18rem] w-[1.15rem] rounded-full bg-[rgb(var(--rolecue-ink-rgb)/18%)]" />
            <span className="h-[0.18rem] w-[1.15rem] rounded-full bg-[rgb(var(--rolecue-ink-rgb)/18%)]" />
            <span className="h-[0.18rem] w-[1.15rem] rounded-full bg-[rgb(var(--rolecue-ink-rgb)/18%)]" />
          </aside>
          <div className="relative flex min-w-0 flex-col overflow-hidden p-[clamp(1.25rem,3.2vw,2.7rem)] max-[620px]:p-4">
            <div className="flex justify-between gap-4 font-mono text-[clamp(0.55rem,1.1vw,0.7rem)] font-semibold tracking-[0.06em] text-(--rolecue-ink-muted) uppercase">
              <span>Practice field / 01</span>
              <span>Role focus</span>
            </div>
            <div className="relative z-2 mt-auto max-w-[10.8ch] max-[620px]:mb-auto">
              <p className="m-0 text-(length:--rolecue-type-section) leading-(--rolecue-leading-display) tracking-(--rolecue-tracking-display) text-(--rolecue-ink) font-(--rolecue-weight-display) max-[620px]:text-(length:--rolecue-type-heading-lg)">
                Tell us about a decision that changed the work.
              </p>
              <i className="mt-4 block w-[min(9rem,45%)] border-t-2 border-(--rolecue-accent)" />
            </div>
            <div className="relative z-3 mt-4 max-w-52 rounded-lg border border-[rgb(var(--rolecue-ink-rgb)/10%)] bg-[color-mix(in_srgb,var(--rolecue-surface)_78%,transparent)] px-[0.8rem] py-[0.72rem] shadow-(--rolecue-shadow-low) max-[620px]:mt-auto">
              <span className="block font-mono text-(length:--rolecue-type-label) tracking-[0.06em] text-(--rolecue-ink-muted) uppercase">
                Keep the detail
              </span>
              <strong className="mt-[0.32rem] block text-(length:--rolecue-type-body-sm) leading-tight font-[650]">
                Start with the constraint, then name the choice.
              </strong>
            </div>
            <div
              aria-hidden="true"
              className="absolute top-[28%] right-[-2.8rem] aspect-square w-[clamp(11rem,26vw,22rem)] rounded-full border border-(--rolecue-border) max-[620px]:top-[19%] max-[620px]:-right-20 max-[620px]:w-64"
            >
              <span className="absolute inset-[22%] rounded-full border border-(--rolecue-border)" />
            </div>
            <span
              aria-hidden="true"
              className="absolute top-[35%] right-[12%] w-[clamp(3.5rem,8vw,7rem)] rotate-45 border-t-[clamp(0.35rem,0.75vw,0.65rem)] border-(--rolecue-accent) max-[620px]:top-[29%] max-[620px]:right-[1%] max-[620px]:w-[4.8rem]"
            />
          </div>
        </div>
        <figcaption className="absolute right-4 bottom-4 z-5 max-w-56 rounded-lg border border-white/58 bg-[color-mix(in_srgb,var(--rolecue-surface-dark)_68%,transparent)] px-[0.7rem] py-[0.55rem] text-right font-mono text-(length:--rolecue-type-label) font-semibold leading-[1.45] tracking-[0.035em] text-(--rolecue-on-dark) backdrop-blur-lg max-[620px]:max-w-40 max-[620px]:text-[0.53rem]">
          ROLECUE / PRACTICE FIELD
          <br />A quiet screen for a considered answer
        </figcaption>
      </figure>
    </Reveal>
  );
}

function ProcessArtifact() {
  return (
    <Reveal className="relative mt-[clamp(5.5rem,10vw,8.5rem)] min-h-136 overflow-hidden rounded-[1.4rem] border border-(--rolecue-border) bg-[radial-gradient(circle_at_20%_70%,var(--rolecue-wash-blue),transparent_30%),radial-gradient(circle_at_82%_25%,var(--rolecue-wash-pink),transparent_28%),var(--rolecue-surface-soft)] shadow-(--rolecue-shadow-media) max-[760px]:mt-16 max-[760px]:min-h-100 max-[620px]:min-h-88 max-[620px]:rounded-2xl">
      <div
        aria-hidden="true"
        className="absolute inset-0 opacity-[0.38] bg-[linear-gradient(rgb(var(--rolecue-ink-rgb)/8%)_1px,transparent_1px),linear-gradient(90deg,rgb(var(--rolecue-ink-rgb)/8%)_1px,transparent_1px)] bg-size-[4.5rem_4.5rem]"
      />
      <div className="absolute top-1/2 left-1/2 z-1 flex min-h-68 w-[min(68%,34rem)] -translate-x-1/2 -translate-y-1/2 -rotate-3 flex-col justify-center rounded-xl border border-(--rolecue-border) bg-[color-mix(in_srgb,var(--rolecue-surface)_82%,transparent)] p-[clamp(1.45rem,4vw,3.3rem)] shadow-(--rolecue-shadow-media) backdrop-blur-[11px] max-[760px]:min-h-56 max-[760px]:w-[min(80%,28rem)] max-[620px]:w-[82%] max-[620px]:p-[1.4rem]">
        <p className="m-0 font-mono text-(length:--rolecue-type-label) font-bold tracking-(--rolecue-tracking-label) text-(--rolecue-ink-muted) uppercase">
          RoleCue note / 03
        </p>
        <strong className="mt-4 max-w-[13ch] text-(length:--rolecue-type-heading-lg) leading-(--rolecue-leading-display) tracking-(--rolecue-tracking-display) font-(--rolecue-weight-display)">
          One clear example is more memorable than a perfect script.
        </strong>
        <span className="mt-[1.3rem] font-mono text-(length:--rolecue-type-label) font-bold tracking-(--rolecue-tracking-label) text-(--rolecue-accent-hover) uppercase">
          Save the cue, not the performance.
        </span>
      </div>
      <span
        aria-hidden="true"
        className="absolute -right-24 -bottom-28 aspect-square w-92 rounded-full border border-(--rolecue-border)"
      >
        <span className="absolute inset-[20%] rounded-full border border-(--rolecue-border)" />
      </span>
      <span
        aria-hidden="true"
        className="absolute top-[26%] left-[16%] w-28 rotate-45 border-t-[0.55rem] border-(--rolecue-accent) max-[620px]:top-[15%] max-[620px]:left-[8%]"
      />
    </Reveal>
  );
}

function ClosingMedia() {
  return (
    <Reveal className="relative mt-[clamp(4.2rem,9vw,7.5rem)] min-h-[clamp(25rem,43vw,37rem)] overflow-hidden rounded-[1.4rem] border border-(--rolecue-border) bg-(--rolecue-wash-neutral) shadow-(--rolecue-shadow-media) max-[760px]:mt-16 max-[760px]:min-h-100 max-[620px]:min-h-104 max-[620px]:rounded-2xl">
      <div
        aria-hidden="true"
        className="absolute inset-0 bg-[radial-gradient(circle_at_72%_26%,var(--rolecue-wash-pink),transparent_26%),radial-gradient(circle_at_22%_76%,var(--rolecue-wash-blue),transparent_36%),linear-gradient(rgb(var(--rolecue-ink-rgb)/8%)_1px,transparent_1px),linear-gradient(90deg,rgb(var(--rolecue-ink-rgb)/8%)_1px,transparent_1px)] bg-size-[auto,auto,4.8rem_4.8rem,4.8rem_4.8rem]"
      />
      <div className="absolute inset-[clamp(1.2rem,4vw,3.2rem)] flex flex-col rounded-xl border border-white/78 bg-[color-mix(in_srgb,var(--rolecue-surface)_75%,transparent)] p-[clamp(1.2rem,3vw,2.4rem)] shadow-[inset_0_1px_rgb(255_255_255/88%),0_1.5rem_3.4rem_rgb(var(--rolecue-ink-rgb)/15%)] backdrop-blur-md">
        <div className="flex justify-between gap-4 font-mono text-[clamp(0.55rem,1.1vw,0.7rem)] font-semibold tracking-[0.06em] text-(--rolecue-ink-muted) uppercase">
          <span>RoleCue</span>
          <span>Practice note</span>
        </div>
        <p className="mt-auto mb-0 max-w-[11ch] text-(length:--rolecue-type-section) leading-(--rolecue-leading-display) tracking-(--rolecue-tracking-display) font-(--rolecue-weight-display) max-[620px]:max-w-[9ch]">
          Make the next response feel like yours.
        </p>
        <div className="mt-[1.6rem] flex min-h-[3.7rem] w-[min(100%,31rem)] items-center justify-between gap-4 rounded-lg border border-(--rolecue-border) bg-[color-mix(in_srgb,var(--rolecue-surface)_75%,transparent)] px-[0.85rem] py-[0.7rem] text-(length:--rolecue-type-body-sm) text-(--rolecue-ink-muted) max-[620px]:items-start max-[620px]:flex-col">
          <span>One thing to carry forward…</span>
          <i aria-hidden="true" className="h-5 w-px bg-(--rolecue-accent)" />
        </div>
      </div>
    </Reveal>
  );
}

export function RoleCueLanding() {
  return (
    <div className="min-w-0 overflow-x-clip bg-(--rolecue-canvas) text-(--rolecue-ink) font-(--rolecue-weight-body) leading-(--rolecue-leading-body)">
      <MarketingHeader />
      <main id="main-content">
        <section
          className="relative mt-[-5.3rem] min-h-232 overflow-hidden pt-[12.7rem] pb-36 max-[760px]:-mt-19 max-[760px]:min-h-0 max-[760px]:pt-39 max-[760px]:pb-23"
          id="top"
        >
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-x-0 top-4 h-208 overflow-hidden max-[760px]:h-168"
          >
            <div className="absolute inset-0 opacity-80 bg-[radial-gradient(circle_at_12%_24%,var(--rolecue-wash-blue),transparent_19rem),radial-gradient(circle_at_88%_30%,var(--rolecue-wash-pink),transparent_24rem),linear-gradient(rgb(var(--rolecue-ink-rgb)/8%)_1px,transparent_1px),linear-gradient(90deg,rgb(var(--rolecue-ink-rgb)/8%)_1px,transparent_1px)] bg-size-[auto,auto,5.5rem_5.5rem,5.5rem_5.5rem] mask-[linear-gradient(90deg,transparent,#000_16%,#000_84%,transparent)] max-[760px]:bg-size-[auto,auto,4rem_4rem,4rem_4rem] max-[760px]:mask-none" />
            <span className="absolute top-46 -right-40 aspect-square w-140 rounded-full border border-(--rolecue-border) max-[760px]:top-76 max-[760px]:-right-72">
              <span className="absolute inset-[21%] rounded-full border border-(--rolecue-border)" />
              <span className="absolute inset-y-0 left-[49%] border-x border-(--rolecue-border)" />
            </span>
            <span className="absolute top-[20.1rem] left-[max(2rem,calc(50%-40rem))] w-52 border-t border-[rgb(var(--rolecue-accent-rgb)/48%)] max-[760px]:top-68 max-[760px]:-left-16 max-[760px]:w-32">
              <i className="absolute -top-1 right-0 size-2 -translate-y-1/2 border border-(--rolecue-accent) bg-(--rolecue-canvas)" />
            </span>
          </div>
          <div className={cn(landingContainer, "relative z-1 text-center")}>
            <Reveal className="relative mx-auto max-w-272 border border-(--rolecue-accent) px-[clamp(1.25rem,3vw,3rem)] py-[clamp(1.45rem,2.5vw,2.3rem)] max-[760px]:px-[1.1rem] max-[760px]:pt-[1.35rem] max-[760px]:pb-6">
              <i className="absolute top-0 left-0 size-[0.58rem] -translate-x-1/2 -translate-y-1/2 bg-(--rolecue-accent)" />
              <i className="absolute top-0 right-0 size-[0.58rem] translate-x-1/2 -translate-y-1/2 bg-(--rolecue-accent)" />
              <i className="absolute bottom-0 left-0 size-[0.58rem] -translate-x-1/2 translate-y-1/2 bg-(--rolecue-accent)" />
              <i className="absolute right-0 bottom-0 size-[0.58rem] translate-x-1/2 translate-y-1/2 bg-(--rolecue-accent)" />
              <p className={eyebrow}>Technical interview practice</p>
              <h1 className="mx-auto max-w-[11.8ch] text-balance text-(length:--rolecue-type-display) leading-(--rolecue-leading-display) tracking-(--rolecue-tracking-display) font-(--rolecue-weight-display) max-[760px]:text-[clamp(2.3rem,11vw,3.2rem)]">
                Practice the role{" "}
                <em className="relative z-1 not-italic font-[720] after:absolute after:right-[-0.05em] after:bottom-[0.02em] after:left-[-0.08em] after:z-[-1] after:h-[0.17em] after:bg-[rgb(var(--rolecue-accent-rgb)/28%)]">
                  before
                </em>{" "}
                the room.
              </h1>
              <p className="mx-auto mt-[1.35rem] max-w-(--rolecue-width-hero-copy) text-balance text-(length:--rolecue-type-display-secondary) leading-[1.13] tracking-[-0.035em] text-(--rolecue-ink-muted) font-[690] max-[760px]:text-[clamp(1.2rem,6vw,1.7rem)]">
                A calmer way to shape the examples, choices, and confidence you
                want to bring to the next technical conversation.
              </p>
            </Reveal>
            <ul className="mx-auto mt-8 flex max-w-184 flex-wrap justify-center gap-2 p-0 max-[760px]:mt-[1.6rem] max-[760px]:max-w-80 max-[760px]:gap-[0.42rem]">
              {["Role context", "Clear examples", "Useful cues"].map((tag) => (
                <li
                  className="list-none rounded-full bg-(--rolecue-accent-soft) px-[0.76rem] py-[0.42rem] text-(length:--rolecue-type-label) font-semibold max-[760px]:text-[0.68rem]"
                  key={tag}
                >
                  {tag}
                </li>
              ))}
            </ul>
            <div className="mt-[2.35rem] flex flex-wrap justify-center gap-3 max-[760px]:mx-auto max-[760px]:w-[min(100%,21rem)] max-[760px]:*:flex-1">
              <Button asChild className={cn(primaryButton, landingFocus)}>
                <Link href={routes.interviews.new.jobDescription}>
                  Start a practice <Arrow />
                </Link>
              </Button>
              <Button
                asChild
                className={cn(secondaryButton, landingFocus)}
                variant="outline"
              >
                <a href="#method">See the method</a>
              </Button>
            </div>
          </div>
          <HeroMedia />
        </section>

        <section className="px-5 py-[clamp(9rem,18vw,16rem)] pt-[clamp(10rem,20vw,18rem)] max-[760px]:py-34">
          <Reveal className="mx-auto w-full max-w-(--rolecue-width-editorial) text-center">
            <p className={eyebrow}>Less performance, more preparation</p>
            <h2 className={sectionTitle}>
              Bring the thinking behind your work into the conversation.
            </h2>
            <p className={cn(sectionIntro, "mx-auto")}>
              The strongest interview answer does not sound rehearsed. It makes
              a decision legible: what you saw, what you chose, and why it
              mattered.
            </p>
          </Reveal>
        </section>

        <PracticeSection />

        <section
          className="scroll-mt-26 border-t border-(--rolecue-border-soft) py-[clamp(8rem,15vw,14rem)] max-[760px]:py-30"
          id="method"
        >
          <div className={landingContainer}>
            <Reveal className="mx-auto max-w-(--rolecue-width-editorial) text-center">
              <p className={eyebrow}>A usable rhythm</p>
              <h2 className={sectionTitle}>
                From first read to a response you can stand behind.
              </h2>
              <p className={cn(sectionIntro, "mx-auto")}>
                A small sequence creates enough structure to prepare without
                turning the work into a script.
              </p>
            </Reveal>
            <ol className="mt-[clamp(4rem,8vw,6.8rem)] grid grid-cols-3 border-t border-(--rolecue-border) p-0 max-[760px]:grid-cols-1">
              {processSteps.map((step) => (
                <li
                  className="min-h-60 list-none border-r border-b border-(--rolecue-border) px-[1.45rem] pt-[1.35rem] pb-[1.7rem] first:border-l max-[760px]:min-h-0 max-[760px]:border-l max-[760px]:px-[1.1rem] max-[760px]:pt-5 max-[760px]:pb-[1.45rem]"
                  key={step.number}
                >
                  <span className="font-mono text-(length:--rolecue-type-label) font-bold tracking-(--rolecue-tracking-label) text-(--rolecue-accent-hover)">
                    {step.number}
                  </span>
                  <h3 className="mt-14 mb-0 max-w-[12ch] text-(length:--rolecue-type-heading-md) leading-(--rolecue-leading-heading) tracking-(--rolecue-tracking-heading) font-(--rolecue-weight-heading) max-[760px]:mt-7">
                    {step.title}
                  </h3>
                  <p className="mt-[0.8rem] mb-0 max-w-84 text-(length:--rolecue-type-body-sm) leading-(--rolecue-leading-copy) text-(--rolecue-ink-muted)">
                    {step.detail}
                  </p>
                </li>
              ))}
            </ol>
            <ProcessArtifact />
          </div>
        </section>

        <section className="bg-(--rolecue-surface-dark) py-[clamp(8rem,14vw,13rem)] text-(--rolecue-on-dark) max-[760px]:py-30">
          <div className={landingContainer}>
            <Reveal className="mx-auto max-w-(--rolecue-width-editorial) text-center">
              <p className={cn(eyebrow, "text-(--rolecue-on-dark-muted)")}>
                One private working field
              </p>
              <h2 className={cn(sectionTitle, "text-(--rolecue-on-dark)")}>
                Give each part of the conversation its own clear place.
              </h2>
              <p
                className={cn(
                  sectionIntro,
                  "mx-auto text-(--rolecue-on-dark-muted)",
                )}
              >
                RoleCue keeps the interface quiet so the useful detail can stay
                in view.
              </p>
            </Reveal>
            <div className="mt-[clamp(3.5rem,7vw,6rem)] grid grid-cols-3 gap-px overflow-hidden rounded-2xl border border-white/20 bg-white/16 max-[760px]:grid-cols-1">
              {workingModes.map((mode) => (
                <article
                  className="flex min-h-76 flex-col justify-between bg-(--rolecue-surface-dark-raised) p-6 transition-colors duration-(--duration-fast) ease-(--ease-smooth-out) hover:bg-(--rolecue-surface-dark-hover) motion-reduce:transition-none! max-[760px]:min-h-56"
                  key={mode.number}
                >
                  {mode.shape === "square" ? (
                    <span className="block size-16 rotate-12 rounded-[0.85rem] bg-(--rolecue-accent) shadow-[3.1rem_3rem_0_var(--rolecue-media-blue),6.1rem_0.8rem_0_var(--rolecue-media-pink)]" />
                  ) : null}
                  {mode.shape === "circle" ? (
                    <span className="block size-16 rounded-full bg-(--rolecue-media-blue) shadow-[3.4rem_1rem_0_var(--rolecue-accent),1.75rem_4.1rem_0_var(--rolecue-media-pink)]" />
                  ) : null}
                  {mode.shape === "line" ? (
                    <span className="my-4 block h-8 w-28 bg-(--rolecue-media-pink) shadow-[0_3rem_0_var(--rolecue-accent)]" />
                  ) : null}
                  <div>
                    <p className="m-0 font-mono text-(length:--rolecue-type-label) font-bold tracking-(--rolecue-tracking-label) text-(--rolecue-on-dark-muted) uppercase">
                      {mode.number}
                    </p>
                    <h3 className="mt-[0.35rem] mb-0 max-w-[13ch] text-(length:--rolecue-type-heading-md) leading-(--rolecue-leading-heading) tracking-(--rolecue-tracking-heading) text-(--rolecue-on-dark) font-(--rolecue-weight-heading)">
                      {mode.title}
                    </h3>
                  </div>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="py-[clamp(10rem,18vw,16rem)] max-[760px]:py-30">
          <div
            className={cn(
              landingContainer,
              "grid grid-cols-[minmax(0,1fr)_minmax(22rem,0.8fr)] items-center gap-[clamp(3rem,9vw,11rem)] max-[1080px]:gap-16 max-[760px]:grid-cols-1 max-[760px]:gap-13",
            )}
          >
            <Reveal className="max-w-2xl">
              <p className={eyebrow}>The cue stays with you</p>
              <h2 className={sectionTitle}>
                Notice the pattern. Then take it into the next room.
              </h2>
              <p className={cn(sectionIntro, "ml-0")}>
                A good review does not turn your experience into a score. It
                gives you a sharper way to recognize what is worth saying.
              </p>
              <Button
                asChild
                className={cn(secondaryButton, landingFocus, "mt-[2.2rem]")}
                variant="outline"
              >
                <Link href={routes.interviews.new.jobDescription}>
                  Prepare a role <Arrow />
                </Link>
              </Button>
            </Reveal>
            <Reveal className="relative mx-auto aspect-square min-h-124 w-full overflow-hidden rounded-full border border-(--rolecue-border) bg-[radial-gradient(circle_at_48%_52%,rgb(var(--rolecue-accent-rgb)/14%),transparent_27%),var(--rolecue-surface-soft)] max-[760px]:min-h-100 max-[760px]:max-w-md max-[620px]:min-h-88">
              <span className="absolute inset-[15%] rounded-full border border-(--rolecue-border)" />
              <span className="absolute inset-[31%] rounded-full border border-[rgb(var(--rolecue-accent-rgb)/72%)]" />
              <span className="absolute top-[18%] left-[11%] grid size-[3.8rem] place-items-center rounded-full border border-(--rolecue-border) bg-(--rolecue-surface) font-mono text-(length:--rolecue-type-label) font-bold tracking-[0.03em] shadow-(--rolecue-shadow-low) max-[620px]:size-[3.15rem] max-[620px]:text-[0.53rem]">
                ROLE
              </span>
              <span className="absolute top-[16%] right-[10%] grid size-[3.8rem] place-items-center rounded-full border border-(--rolecue-border) bg-(--rolecue-surface) font-mono text-(length:--rolecue-type-label) font-bold tracking-[0.03em] shadow-(--rolecue-shadow-low) max-[620px]:size-[3.15rem] max-[620px]:text-[0.53rem]">
                CHOICE
              </span>
              <span className="absolute bottom-[15%] left-[15%] grid size-[3.8rem] place-items-center rounded-full border border-(--rolecue-border) bg-(--rolecue-surface) font-mono text-(length:--rolecue-type-label) font-bold tracking-[0.03em] shadow-(--rolecue-shadow-low) max-[620px]:size-[3.15rem] max-[620px]:text-[0.53rem]">
                DETAIL
              </span>
              <span className="absolute right-[15%] bottom-[13%] grid size-[3.8rem] place-items-center rounded-full border border-(--rolecue-border) bg-(--rolecue-surface) font-mono text-(length:--rolecue-type-label) font-bold tracking-[0.03em] shadow-(--rolecue-shadow-low) max-[620px]:size-[3.15rem] max-[620px]:text-[0.53rem]">
                CUE
              </span>
              <span className="absolute top-[calc(50%-2.7rem)] left-[calc(50%-2.7rem)] grid size-[5.4rem] place-items-center rounded-full border border-(--rolecue-accent) bg-(--rolecue-accent) font-mono text-[0.77rem] font-bold tracking-[0.03em] text-(--rolecue-ink) shadow-(--rolecue-shadow-low) max-[620px]:top-[calc(50%-2.25rem)] max-[620px]:left-[calc(50%-2.25rem)] max-[620px]:size-18">
                YOU
              </span>
            </Reveal>
          </div>
        </section>

        <FaqSection />

        <section className="pt-[clamp(11rem,21vw,19rem)] pb-[clamp(7rem,12vw,11rem)] max-[760px]:pt-38 max-[760px]:pb-24">
          <div className={landingContainer}>
            <Reveal className="mx-auto w-full max-w-(--rolecue-width-editorial) text-center">
              <p className={eyebrow}>The next response</p>
              <h2 className={sectionTitle}>
                Walk in with more of your own work in reach.
              </h2>
              <p className={cn(sectionIntro, "mx-auto")}>
                Start with the role. Make space for the answer. Keep the cue
                that makes the next attempt better.
              </p>
              <Button
                asChild
                className={cn(primaryButton, landingFocus, "mt-9")}
              >
                <Link href={routes.interviews.new.jobDescription}>
                  Start a practice <Arrow />
                </Link>
              </Button>
            </Reveal>
            <ClosingMedia />
          </div>
        </section>
      </main>
      <footer className="border-t border-(--rolecue-border) pt-16 pb-[1.55rem] max-[620px]:pt-12">
        <div className={landingContainer}>
          <div className="grid grid-cols-[1fr_auto] items-end gap-8 max-[760px]:grid-cols-1">
            <div>
              <p className="mb-[0.9rem] font-mono text-(length:--rolecue-type-label) font-bold tracking-(--rolecue-tracking-label) text-(--rolecue-ink-muted) uppercase">
                Role-focused practice
              </p>
              <p className="m-0 text-[clamp(4.25rem,10.5vw,9rem)] leading-[0.78] tracking-[-0.095em] [font-variation-settings:'wght'_850]">
                Role<span className="text-(--rolecue-accent)">Cue.</span>
              </p>
            </div>
            <nav
              aria-label="Footer navigation"
              className="flex flex-wrap justify-end gap-5 text-(length:--rolecue-type-body-sm) text-(--rolecue-ink-muted) max-[760px]:justify-start"
            >
              <a
                className={cn(
                  landingFocus,
                  "no-underline transition-colors duration-(--duration-quick) hover:text-(--rolecue-accent-hover) motion-reduce:transition-none!",
                )}
                href="#practice"
              >
                Practice
              </a>
              <a
                className={cn(
                  landingFocus,
                  "no-underline transition-colors duration-(--duration-quick) hover:text-(--rolecue-accent-hover) motion-reduce:transition-none!",
                )}
                href="#method"
              >
                Method
              </a>
              <a
                className={cn(
                  landingFocus,
                  "no-underline transition-colors duration-(--duration-quick) hover:text-(--rolecue-accent-hover) motion-reduce:transition-none!",
                )}
                href="#questions"
              >
                Questions
              </a>
              <Link
                className={cn(
                  landingFocus,
                  "no-underline transition-colors duration-(--duration-quick) hover:text-(--rolecue-accent-hover) motion-reduce:transition-none!",
                )}
                href={routes.interviews.new.jobDescription}
              >
                Start
              </Link>
            </nav>
          </div>
          <div className="mt-[4.2rem] flex justify-between gap-4 font-mono text-(length:--rolecue-type-label) font-semibold tracking-wide text-(--rolecue-ink-muted) max-[760px]:mt-12 max-[760px]:flex-col">
            <span>© 2026 RoleCue</span>
            <span>Made for clearer technical conversations</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
