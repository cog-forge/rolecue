import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { TextGenerateEffect } from "@/components/ui/text-generate-effect";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { AboutRolecueSection } from "./about-rolecue-section";
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
        <div className="relative isolate overflow-hidden">
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 -z-10 overflow-hidden"
          >
            <div className="absolute inset-0 bg-[linear-gradient(rgb(var(--rolecue-ink-rgb)/5%)_1px,transparent_1px),linear-gradient(90deg,rgb(var(--rolecue-ink-rgb)/5%)_1px,transparent_1px)] bg-size-[5.5rem_5.5rem] opacity-75 mask-[linear-gradient(180deg,#000_0%,#000_78%,transparent_100%)] max-[760px]:bg-size-[4rem_4rem]" />
            <div className="absolute -top-40 left-[2%] size-[min(70vw,48rem)] rounded-full bg-(--rolecue-wash-blue) blur-[110px]" />
            <div className="absolute -top-24 right-[2%] size-[min(65vw,42rem)] rounded-full bg-(--rolecue-wash-pink) blur-[110px]" />
            <div className="absolute top-[50%] right-[8%] size-[min(72vw,46rem)] rounded-full bg-(--rolecue-wash-blue) opacity-80 blur-[130px]" />
          </div>
          <section
            className="relative mt-[-5.3rem] flex min-h-[min(58rem,100svh)] items-end overflow-hidden pt-[9rem] pb-[clamp(7rem,12vh,10rem)] max-[760px]:-mt-19 max-[760px]:min-h-[88svh] max-[760px]:pt-32 max-[760px]:pb-28"
            id="top"
          >
            <video
              aria-label="RoleCue technical interview practice preview"
              autoPlay
              className="absolute inset-0 size-full object-cover object-center motion-reduce:hidden max-[760px]:object-[42%_center]"
              loop
              muted
              playsInline
              poster="/images/hero-poster.webp"
              preload="metadata"
            >
              <source src="/hero.mp4" type="video/mp4" />
            </video>
            <div
              aria-label="RoleCue technical interview practice preview"
              className="absolute inset-0 hidden bg-[url('/images/hero-poster.webp')] bg-cover bg-center motion-reduce:block max-[760px]:bg-[position:42%_center]"
              role="img"
            />
            <div
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,transparent_0%,transparent_88%,rgb(var(--rolecue-canvas-rgb)/24%)_95%,var(--rolecue-canvas)_100%)] max-[760px]:bg-[linear-gradient(180deg,transparent_0%,transparent_86%,rgb(var(--rolecue-canvas-rgb)/30%)_95%,var(--rolecue-canvas)_100%)]"
            />

            <div className={cn(landingContainer, "relative")}>
              <Reveal className="max-w-[31rem] translate-x-[-1rem] translate-y-3 max-[760px]:translate-x-0 max-[760px]:translate-y-2">
                <h1 className="mt-3 max-w-[11ch] text-balance text-[clamp(2.4rem,4.6vw,4.1rem)] font-(--rolecue-weight-display) leading-[1.02] tracking-(--rolecue-tracking-display) text-(--rolecue-ink) max-[760px]:text-[clamp(2.2rem,9vw,3rem)]">
                  <span className="sr-only">Practice the role before the room.</span>
                  <TextGenerateEffect
                    className="text-inherit"
                    duration={0.32}
                    filter={false}
                    words="Practice the role before the room."
                  />
                </h1>
                <p className="mt-6 max-w-[28rem] text-pretty text-sm leading-(--rolecue-leading-copy) font-medium text-(--rolecue-ink) [text-shadow:0_1px_8px_var(--rolecue-copy-halo)] sm:text-base">
                  A calmer way to prepare for the next technical conversation.
                </p>
                <div className="group/trial relative mt-5 inline-flex">
                  <Button
                    asChild
                    className={cn(primaryButton, landingFocus, "relative min-h-12 overflow-hidden pr-12 pl-2 font-[family-name:var(--font-cta)] text-sm font-semibold shadow-(--rolecue-shadow-low)")}
                  >
                    <Link href={routes.interviews.new.jobDescription}>
                      <span className="grid size-8 shrink-0 place-items-center rounded-full bg-[#65f328] text-[#2f3139]">
                        <Arrow />
                      </span>
                      Start a practice
                      <span
                        aria-hidden="true"
                        className="pointer-events-none absolute top-0 right-0 h-14 w-14 overflow-hidden rounded-tr-[inherit]"
                      >
                        <span className="absolute top-[0.55rem] right-[-1.35rem] flex h-4 w-24 rotate-45 items-center justify-center bg-[#65f328] pt-0.5 text-[0.5rem] leading-none font-black tracking-[0.03em] text-[#2f3139] shadow-sm">
                          FREE TRIAL
                        </span>
                      </span>
                    </Link>
                  </Button>
                </div>
              </Reveal>
            </div>
          </section>

          <AboutRolecueSection />
        </div>

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
