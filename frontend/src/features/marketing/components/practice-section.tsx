"use client";

import { ArrowUpRight } from "lucide-react";
import { useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { Reveal } from "./reveal";
import {
  eyebrow,
  landingContainer,
  sectionIntro,
  sectionTitle,
} from "./rolecue-landing-styles";

const practiceStages = [
  {
    key: "role",
    number: "01",
    label: "Read the role",
    title: "Find the signal beneath the job description.",
    detail:
      "Start with the decisions, technical context, and point of view the conversation is really trying to surface.",
    visualLabel: "Role signal",
  },
  {
    key: "response",
    number: "02",
    label: "Try the response",
    title: "Give the answer room to take shape.",
    detail:
      "Work through one focused prompt at a time, with enough quiet to notice what is true, vague, or missing.",
    visualLabel: "Response loop",
  },
  {
    key: "review",
    number: "03",
    label: "Keep the cue",
    title: "Turn each attempt into a more useful next one.",
    detail:
      "Save the patterns worth carrying forward: clearer examples, sharper structure, and the question behind the question.",
    visualLabel: "Review note",
  },
] as const;

type PracticeStage = (typeof practiceStages)[number];
type PracticeStageKey = PracticeStage["key"];

function stageDescriptionId(stageKey: PracticeStageKey) {
  return `practice-stage-${stageKey}-description`;
}

const stageTheme: Record<PracticeStageKey, { tone: string; wash: string }> = {
  role: {
    tone: "[--stage-accent:var(--rolecue-accent)] [--stage-ink:var(--rolecue-on-dark-muted)]",
    wash: "bg-[radial-gradient(circle_at_78%_15%,rgb(var(--rolecue-accent-rgb)/24%),transparent_25%)]",
  },
  response: {
    tone: "[--stage-accent:var(--rolecue-media-blue)] [--stage-ink:color-mix(in_srgb,var(--rolecue-media-blue)_30%,var(--rolecue-on-dark))]",
    wash: "bg-[radial-gradient(circle_at_80%_20%,var(--rolecue-wash-blue),transparent_26%)]",
  },
  review: {
    tone: "[--stage-accent:var(--rolecue-media-pink)] [--stage-ink:color-mix(in_srgb,var(--rolecue-media-pink)_40%,var(--rolecue-on-dark))]",
    wash: "bg-[radial-gradient(circle_at_80%_20%,var(--rolecue-wash-pink),transparent_25%)]",
  },
};

function PracticeStagePanel({ stage }: { stage: PracticeStage }) {
  const theme = stageTheme[stage.key];

  return (
    <article
      className={cn(
        theme.tone,
        "relative min-h-136 overflow-hidden rounded-[1.4rem] border border-(--rolecue-border) bg-[linear-gradient(140deg,var(--rolecue-surface-dark-raised),var(--rolecue-surface-dark)_72%)] text-(--rolecue-on-dark) shadow-(--rolecue-shadow-media) max-[760px]:min-h-100 max-[620px]:min-h-92 max-[620px]:rounded-2xl",
      )}
    >
      <div aria-hidden="true" className={cn("absolute inset-0", theme.wash)} />
      <div
        aria-hidden="true"
        className="absolute inset-0 opacity-40 bg-[linear-gradient(rgb(255_255_255/11%)_1px,transparent_1px),linear-gradient(90deg,rgb(255_255_255/11%)_1px,transparent_1px)] bg-size-[3.65rem_3.65rem]"
      />
      <div
        aria-hidden="true"
        className="absolute top-1/2 left-1/2 aspect-square w-104 -translate-x-1/2 -translate-y-1/2 rounded-full border border-white/25 max-[760px]:w-84"
      >
        <span className="absolute inset-[18%] rounded-full border border-white/22" />
        <span className="absolute inset-y-0 left-[49.8%] border-l border-white/18" />
      </div>
      <span
        aria-hidden="true"
        className="absolute top-[39%] left-[54%] w-32 rotate-45 border-t-[0.62rem] border-(--stage-accent)"
      />
      <div className="absolute inset-[clamp(1.3rem,4vw,2.6rem)] flex flex-col rounded-xl border border-white/27 bg-[color-mix(in_srgb,var(--rolecue-surface-dark)_58%,transparent)] p-[clamp(1rem,2.4vw,1.8rem)] shadow-[inset_0_1px_rgb(255_255_255/12%)] backdrop-blur-[7px] max-[620px]:inset-4">
        <div className="flex items-center gap-[0.28rem]">
          <span className="size-[0.32rem] rounded-full bg-white/48" />
          <span className="size-[0.32rem] rounded-full bg-white/48" />
          <span className="size-[0.32rem] rounded-full bg-white/48" />
          <p className="mb-0 ml-[0.55rem] font-mono text-(length:--rolecue-type-label) tracking-wider text-white/68 uppercase">
            {stage.visualLabel}
          </p>
        </div>
        <p className="mt-auto mb-0 font-mono text-[0.75rem] font-bold tracking-[0.08em] text-(--stage-accent)">
          {stage.number}
        </p>
        <p className="mt-[0.65rem] mb-0 max-w-[10ch] text-(length:--rolecue-type-heading-lg) leading-(--rolecue-leading-display) tracking-(--rolecue-tracking-display) text-(--rolecue-on-dark) font-[760] max-[620px]:text-(length:--rolecue-type-heading-lg)">
          What do you want the interviewer to remember?
        </p>
        <span className="mt-[1.15rem] w-28 border-t-2 border-(--stage-accent)" />
        <p className="mt-4 mb-0 font-mono text-(length:--rolecue-type-label) tracking-[0.06em] text-(--stage-ink) uppercase">
          A cue for the next response
        </p>
      </div>
    </article>
  );
}

export function PracticeSection() {
  const [activeStageKey, setActiveStageKey] =
    useState<PracticeStageKey>("role");
  const activeStage =
    practiceStages.find((stage) => stage.key === activeStageKey) ??
    practiceStages[0];

  return (
    <section
      className="scroll-mt-26 pb-[clamp(10rem,19vw,17rem)] max-[760px]:pb-36"
      id="practice"
    >
      <div className={landingContainer}>
        <Reveal className="mx-auto w-full max-w-(--rolecue-width-editorial) text-center">
          <p className={eyebrow}>A calmer preparation loop</p>
          <h2 className={sectionTitle}>
            The best answer starts before you start speaking.
          </h2>
          <p className={cn(sectionIntro, "mx-auto")}>
            RoleCue makes a private space for the kind of preparation that feels
            more like thinking than performing.
          </p>
        </Reveal>

        <Tabs
          className="block"
          onValueChange={(value) => {
            const stage = practiceStages.find((item) => item.key === value);
            if (stage) {
              setActiveStageKey(stage.key);
            }
          }}
          value={activeStageKey}
        >
          <div className="grid grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)] items-center gap-[clamp(2.5rem,7vw,8rem)] pt-[clamp(5.5rem,12vw,10rem)] max-[1080px]:gap-14 max-[760px]:grid-cols-1 max-[760px]:gap-13 max-[760px]:pt-18">
            <div className="max-w-136 max-[760px]:max-w-none">
              <p className={eyebrow}>The practice</p>
              <TabsList
                aria-label="Practice stages"
                className="mt-9 grid h-auto w-full rounded-none border-t border-(--rolecue-border) bg-transparent p-0"
                variant="line"
              >
                {practiceStages.map((stage) => (
                  <TabsTrigger
                    className="group grid h-auto min-h-19 w-full grid-cols-[2.65rem_1fr_auto] items-center gap-3 rounded-none border-0 border-b border-(--rolecue-border) px-0 py-3 text-left text-(length:--rolecue-type-body) text-(--rolecue-ink) shadow-none after:hidden hover:text-(--rolecue-accent-hover) data-[state=active]:bg-transparent data-[state=active]:text-(--rolecue-accent-hover) data-[state=active]:shadow-none"
                    key={stage.key}
                    value={stage.key}
                  >
                    <span className="font-mono text-(length:--rolecue-type-label) font-bold text-(--rolecue-ink-subtle)">
                      {stage.number}
                    </span>
                    <strong className="text-left font-[690]">
                      {stage.label}
                    </strong>
                    <ArrowUpRight
                      aria-hidden="true"
                      className="size-[1.1rem] transition-transform duration-(--duration-quick) ease-(--ease-smooth-out) group-data-[state=active]:translate-x-[0.12rem] group-data-[state=active]:translate-y-[-0.08rem] motion-reduce:transform-none! motion-reduce:transition-none!"
                      strokeWidth={1.8}
                    />
                  </TabsTrigger>
                ))}
              </TabsList>
              <div
                className="min-h-44 pt-[2.35rem] max-[620px]:min-h-[12.3rem]"
                id={stageDescriptionId(activeStage.key)}
              >
                <h3 className="m-0 max-w-[17ch] text-(length:--rolecue-type-heading-lg) leading-(--rolecue-leading-heading) tracking-(--rolecue-tracking-heading) font-(--rolecue-weight-heading)">
                  {activeStage.title}
                </h3>
                <p className="mt-[0.9rem] mb-0 max-w-124 text-(length:--rolecue-type-body) leading-(--rolecue-leading-copy) text-(--rolecue-ink-muted)">
                  {activeStage.detail}
                </p>
              </div>
            </div>

            <div>
              {practiceStages.map((stage) => (
                <TabsContent
                  aria-describedby={stageDescriptionId(stage.key)}
                  className="m-0 flex-none outline-none"
                  data-stage={stage.key}
                  key={stage.key}
                  value={stage.key}
                >
                  <PracticeStagePanel stage={stage} />
                </TabsContent>
              ))}
            </div>
          </div>
        </Tabs>
      </div>
    </section>
  );
}
