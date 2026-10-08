import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { cn } from "@/lib/utils";
import { MarketingBackdrop } from "./marketing-backdrop";
import { Reveal } from "./reveal";
import {
  eyebrow,
  landingContainer,
  landingFocus,
  sectionTitle,
} from "./rolecue-landing-styles";

const questions = [
  {
    question: "Who is RoleCue for?",
    answer:
      "RoleCue is for people preparing for technical interviews, whether you’re starting out or moving to a new role.",
    sticker: "💙",
  },
  {
    question: "What do I need to get started?",
    answer:
      "Create an account, bring a target job description, and have a microphone ready for the spoken interview.",
  },
  {
    question: "Are the questions based on my job description?",
    answer:
      "Yes. RoleCue uses the technical context in your job description to guide the interview. Later questions also respond to your answers.",
  },
  {
    question: "Can I choose my interviewer?",
    answer:
      "Yes. For your own practice sessions, choose an available system or personal 3D avatar and a voice profile.",
  },
  {
    question: "What do I get after a session?",
    answer:
      "You get an interview report with an evaluation of your technical responses, so you can review your attempt and see what to improve.",
  },
  {
    question: "Can I start for free?",
    answer: "Yes. You can create an account and start practicing for free.",
    sticker: "⭐",
  },
] as const;

export function FaqSection() {
  return (
    <section
      aria-labelledby="faq-title"
      className="relative isolate border-t border-(--rolecue-border-soft) pt-[clamp(5rem,8vw,8rem)] pb-12 max-[760px]:pb-8"
    >
      <MarketingBackdrop layout="practice" />
      <div
        className={cn(landingContainer, "relative scroll-mt-28")}
        id="questions"
      >
        <div className="mx-auto max-w-[56rem]">
          <Reveal className="mb-10 sm:mb-12">
            <p className={eyebrow}>Before you begin</p>
            <h2 className={cn(sectionTitle, "max-w-[20ch]")} id="faq-title">
              A few things to know.
            </h2>
          </Reveal>

          <Reveal delay={0.1}>
            <Accordion
              type="multiple"
              defaultValue={["question-1"]}
              className="gap-4 sm:gap-5"
              aria-label="Questions about RoleCue"
            >
              {questions.map((item, index) => (
                <AccordionItem
                  className="group/faq border-0! [&>[data-slot=accordion-content][data-state=open]]:animate-accordion-down [&>[data-slot=accordion-content][data-state=closed]]:animate-accordion-up [&>[data-slot=accordion-content]]:[--tw-animation-duration:var(--duration-slow)] [&>[data-slot=accordion-content]]:[--tw-ease:var(--ease-smooth-out)] motion-reduce:[&>[data-slot=accordion-content]]:animate-none!"
                  key={item.question}
                  value={`question-${index + 1}`}
                >
                  <AccordionTrigger
                    className={cn(
                      "group w-fit max-w-full flex-none items-center gap-3 rounded-2xl border-0 p-0 text-left no-underline hover:no-underline sm:gap-5",
                      landingFocus,
                    )}
                    showIndicator={false}
                  >
                    <span className="relative block min-w-0 rounded-[1.25rem] bg-[#e7edf5] px-5 py-4 text-[clamp(1.05rem,1.65vw,1.45rem)] leading-snug font-medium tracking-[-0.02em] text-(--rolecue-ink) transition-colors duration-(--duration-fast) group-hover:bg-[#dde7f3] group-data-[state=open]:bg-[#d5e2f2] sm:rounded-[1.5rem] sm:px-7 sm:py-5 motion-reduce:transition-none!">
                      {item.question}
                      {"sticker" in item && (
                        <span
                          aria-hidden="true"
                          className={cn(
                            "pointer-events-none absolute -top-3 text-[1.4rem] sm:text-[1.7rem]",
                            index === 0
                              ? "right-2 rotate-12"
                              : "-left-1 -rotate-12",
                          )}
                        >
                          {item.sticker}
                        </span>
                      )}
                    </span>
                    <span
                      aria-hidden="true"
                      className="relative grid size-8 shrink-0 place-items-center text-(--rolecue-ink-muted) group-data-[state=open]:text-(--rolecue-ink) sm:size-10"
                    >
                      <span className="absolute h-0.5 w-4 rounded-full bg-current sm:w-5" />
                      <span className="absolute h-0.5 w-4 rotate-90 rounded-full bg-current transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-data-[state=open]:rotate-0 sm:w-5 motion-reduce:transition-none!" />
                    </span>
                  </AccordionTrigger>
                  <AccordionContent className="pt-2 pb-0 pl-5 sm:pt-3 sm:pl-[clamp(3rem,8vw,7rem)]">
                    <p className="ml-auto max-w-[46rem] rounded-[1.4rem] bg-(--rolecue-ink) px-5 py-5 text-[clamp(1rem,1.55vw,1.3rem)] leading-relaxed font-normal tracking-[-0.01em] text-white sm:rounded-[1.75rem] sm:px-7 sm:py-6">
                      {item.answer}
                    </p>
                  </AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </Reveal>
        </div>
      </div>
    </section>
  );
}
