import { Plus } from "lucide-react";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { cn } from "@/lib/utils";
import { Reveal } from "./reveal";
import {
  eyebrow,
  landingContainer,
  sectionIntro,
  sectionTitle,
} from "./rolecue-landing-styles";

const questions = [
  {
    question: "What does a practice start with?",
    answer:
      "A role and the context you want to prepare for. The landing points to the existing interview setup flow; it does not claim a backend workflow that is not connected yet.",
  },
  {
    question: "Is RoleCue trying to script an interview?",
    answer:
      "No. The visual and content direction centers reflection, examples, and clearer choices—not a synthetic answer generator or a promise of an outcome.",
  },
  {
    question: "What should feel different after a session?",
    answer:
      "You should leave with a more legible version of your own experience: what matters, which example proves it, and how to bring it into the next conversation.",
  },
  {
    question: "Can I return to a specific part of the page?",
    answer:
      "Yes. The navigation and disclosure controls work with keyboard focus, named anchors, and reduced-motion preferences as well as pointer input.",
  },
] as const;

export function FaqSection() {
  return (
    <section
      className="scroll-mt-26 border-t border-(--rolecue-border-soft) py-[clamp(8rem,15vw,14rem)] max-[760px]:py-30"
      id="questions"
    >
      <div className={landingContainer}>
        <div className="grid grid-cols-[minmax(0,0.72fr)_minmax(0,1fr)] items-start gap-[clamp(3rem,9vw,11rem)] max-[1080px]:gap-16 max-[760px]:grid-cols-1 max-[760px]:gap-14">
          <Reveal className="max-w-140">
            <p className={eyebrow}>A few useful details</p>
            <h2 className={sectionTitle}>
              Preparation should make the next question feel less strange.
            </h2>
            <p className={cn(sectionIntro, "ml-0")}>
              A quiet, practical layer for the things people usually need to
              know before they begin.
            </p>
          </Reveal>

          <Reveal>
            <Accordion
              collapsible
              className="border-t border-(--rolecue-border)"
              type="single"
            >
              {questions.map((item, index) => (
                <AccordionItem
                  className="border-b border-(--rolecue-border)"
                  key={item.question}
                  value={`question-${index + 1}`}
                >
                  <AccordionTrigger
                    className="group grid min-h-[5.15rem] grid-cols-[2.6rem_1fr_auto] items-center gap-3 rounded-none border-0 px-0 py-4 text-left text-(--rolecue-ink) no-underline hover:no-underline max-[620px]:grid-cols-[2rem_1fr_auto]"
                    showIndicator={false}
                  >
                    <span className="font-mono text-(length:--rolecue-type-label) font-bold text-(--rolecue-ink-subtle)">
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <strong className="text-(length:--rolecue-type-heading-sm) tracking-(--rolecue-tracking-heading) font-[690]">
                      {item.question}
                    </strong>
                    <span
                      aria-hidden="true"
                      className="grid size-8 place-items-center rounded-full border border-(--rolecue-border) text-(--rolecue-ink) transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-data-[state=open]:rotate-45 motion-reduce:transform-none! motion-reduce:transition-none!"
                    >
                      <Plus size={18} strokeWidth={1.5} />
                    </span>
                  </AccordionTrigger>
                  <AccordionContent className="pb-[1.65rem] pl-[3.35rem] text-(length:--rolecue-type-body-sm) leading-(--rolecue-leading-copy) text-(--rolecue-ink-muted) max-[620px]:pl-8">
                    <p className="m-0 max-w-148">{item.answer}</p>
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
