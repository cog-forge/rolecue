"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { Reveal } from "./reveal";
import {
  eyebrow,
  landingContainer,
  landingFocus,
  sectionTitle,
} from "./rolecue-landing-styles";

const moments = [
  {
    title: "Start with the role",
    description:
      "Bring a target job description into a private practice space, then review the technical context that matters.",
    shortLabel: "Role context",
  },
  {
    title: "Speak your answer",
    description:
      "Rehearse your decisions aloud with an AI interviewer, at the pace of a real technical conversation.",
    shortLabel: "Spoken rehearsal",
  },
  {
    title: "Work through the next question",
    description:
      "The conversation responds to your answer and asks you to explain the trade-offs behind your choices.",
    shortLabel: "Follow-up questions",
  },
  {
    title: "Review what to carry forward",
    description:
      "See where your explanation landed and what to strengthen in the next attempt.",
    shortLabel: "Useful feedback",
  },
] as const;

function PracticeStack({ active }: { active: number }) {
  return (
    <div className="relative mx-auto flex min-h-112 w-full max-w-128 flex-col items-center justify-center max-[760px]:min-h-92">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-[8%] rounded-full border border-(--rolecue-border-soft)"
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-[19%] rounded-full border border-(--rolecue-border-soft)"
      />
      <svg
        aria-hidden="true"
        className="relative h-auto w-[min(100%,28rem)] overflow-visible drop-shadow-[0_30px_35px_rgb(38_38_38/10%)]"
        viewBox="0 0 480 590"
      >
        <defs>
          <linearGradient id="rolecue-stack-left" x1="0" x2="1" y1="0" y2="1">
            <stop offset="0" stopColor="#343b48" />
            <stop offset="1" stopColor="#1f2530" />
          </linearGradient>
          <linearGradient id="rolecue-stack-right" x1="0" x2="1" y1="0" y2="1">
            <stop offset="0" stopColor="#202936" />
            <stop offset="1" stopColor="#111923" />
          </linearGradient>
          <linearGradient id="rolecue-stack-top" x1="0" x2="1" y1="0" y2="1">
            <stop offset="0" stopColor="#414a56" />
            <stop offset="1" stopColor="#242d39" />
          </linearGradient>
        </defs>

        {[3, 2, 1, 0].map((index) => {
          const y = 76 + index * 106;
          const selected = active === index;

          return (
            <g key={index}>
              <path
                d={`M144 ${y + 47} 240 ${y + 94} 240 ${y + 164} 144 ${y + 117}Z`}
                fill="url(#rolecue-stack-left)"
                stroke="#697381"
                strokeOpacity="0.28"
              />
              <path
                d={`M240 ${y + 94} 336 ${y + 47} 336 ${y + 117} 240 ${y + 164}Z`}
                fill="url(#rolecue-stack-right)"
                stroke="#697381"
                strokeOpacity="0.24"
              />
              <path
                d={`M144 ${y + 47} 240 ${y} 336 ${y + 47} 240 ${y + 94}Z`}
                fill="url(#rolecue-stack-top)"
                stroke="#697381"
                strokeOpacity="0.36"
              />
              <path
                d={`M175 ${y + 47} 240 ${y + 16} 305 ${y + 47} 240 ${y + 79}Z`}
                fill="#17212d"
                stroke="#697381"
                strokeOpacity="0.36"
              />
              <path
                d={`M160 ${y + 65} 160 ${y + 109} M176 ${y + 73} 176 ${y + 117} M192 ${y + 81} 192 ${y + 125} M208 ${y + 89} 208 ${y + 133} M224 ${y + 97} 224 ${y + 141} M256 ${y + 101} 256 ${y + 145} M272 ${y + 93} 272 ${y + 137} M288 ${y + 85} 288 ${y + 129} M304 ${y + 77} 304 ${y + 121} M320 ${y + 69} 320 ${y + 113}`}
                fill="none"
                stroke="#9babbd"
                strokeOpacity="0.14"
              />
              <g
                className="transition-opacity duration-500 motion-reduce:transition-none"
                opacity={selected ? 1 : 0}
              >
                <path
                  d={`M164 ${y + 47} 240 ${y + 10} 316 ${y + 47} 240 ${y + 84}Z`}
                  fill="none"
                  stroke="var(--rolecue-brand-blue)"
                  strokeWidth="2.5"
                />
                <path
                  d={`M144 ${y + 47} 240 ${y + 94} 240 ${y + 164} 144 ${y + 117}Z`}
                  fill="var(--rolecue-brand-blue)"
                  fillOpacity="0.12"
                  stroke="var(--rolecue-brand-blue)"
                  strokeWidth="2.5"
                />
                <path
                  d={`M240 ${y + 94} 336 ${y + 47} 336 ${y + 117} 240 ${y + 164}Z`}
                  fill="var(--rolecue-brand-blue)"
                  fillOpacity="0.08"
                  stroke="var(--rolecue-brand-blue)"
                  strokeWidth="2.5"
                />
              </g>
            </g>
          );
        })}
      </svg>
      <p className="relative -mt-6 mb-0 flex items-center gap-3 font-mono text-xs font-semibold tracking-wider text-(--rolecue-ink-muted) uppercase">
        <span className="size-1.5 rounded-full bg-(--rolecue-brand-blue)" />
        {String(active + 1).padStart(2, "0")} / 04 ·{" "}
        {moments[active].shortLabel}
      </p>
    </div>
  );
}

export function AboutRolecueSection() {
  const [active, setActive] = useState(0);

  return (
    <section
      aria-labelledby="about-rolecue-title"
      className="relative scroll-mt-24 pb-[clamp(6rem,11vw,10rem)] pt-[clamp(4rem,7vw,7rem)]"
      id="about"
    >
      <div className={landingContainer}>
        <div className="grid items-center gap-12 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.95fr)] lg:gap-16">
          <Reveal className="relative max-w-144">
            <p className={eyebrow}>Meet RoleCue</p>
            <h2
              className={cn(sectionTitle, "max-w-[13ch]")}
              id="about-rolecue-title"
            >
              A practice room for the role ahead.
            </h2>
            <p className="mt-6 max-w-130 text-pretty text-(length:--rolecue-type-body-lg) leading-(--rolecue-leading-copy) text-(--rolecue-ink-muted)">
              RoleCue turns a target job description into a private technical
              interview rehearsal. Talk through your decisions with an AI
              interviewer, then review what to strengthen before the real
              conversation.
            </p>

            <ul className="mt-10 space-y-2 p-0">
              {moments.map((moment, index) => {
                const selected = active === index;

                return (
                  <li className="list-none" key={moment.title}>
                    <div
                      className={cn(
                        "rounded-xl border px-5 transition-[background-color,border-color,box-shadow] duration-300 motion-reduce:transition-none",
                        selected
                          ? "border-(--rolecue-border) bg-(--rolecue-surface) shadow-(--rolecue-shadow-low)"
                          : "border-transparent hover:border-(--rolecue-border-soft) hover:bg-(--rolecue-surface)",
                      )}
                      onPointerEnter={() => setActive(index)}
                    >
                      <button
                        aria-controls={`about-rolecue-detail-${index}`}
                        aria-expanded={selected}
                        className={cn(
                          landingFocus,
                          "flex min-h-15 w-full items-center gap-4 py-3 text-left text-base font-semibold transition-colors duration-300 motion-reduce:transition-none",
                          selected
                            ? "text-(--rolecue-ink)"
                            : "text-(--rolecue-ink-muted) hover:text-(--rolecue-ink)",
                        )}
                        onClick={() => setActive(index)}
                        onFocus={() => setActive(index)}
                        type="button"
                      >
                        <span className="font-mono text-xs text-(--rolecue-brand-blue)">
                          {String(index + 1).padStart(2, "0")}
                        </span>
                        <span>{moment.title}</span>
                      </button>
                      <div
                        aria-hidden={!selected}
                        className={cn(
                          "grid transition-[grid-template-rows,opacity] duration-300 ease-(--ease-smooth-out) motion-reduce:transition-none",
                          selected
                            ? "grid-rows-[1fr] opacity-100"
                            : "grid-rows-[0fr] opacity-0",
                        )}
                        id={`about-rolecue-detail-${index}`}
                      >
                        <div className="overflow-hidden">
                          <p className="mb-5 pl-9 text-sm leading-relaxed text-(--rolecue-ink-muted) sm:text-base">
                            {moment.description}
                          </p>
                        </div>
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          </Reveal>

          <Reveal delay={0.12}>
            <PracticeStack active={active} />
          </Reveal>
        </div>
      </div>
    </section>
  );
}
