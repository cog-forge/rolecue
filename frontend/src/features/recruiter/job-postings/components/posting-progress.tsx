import { cn } from "@/lib/utils";
import type { PostingStatus } from "../schemas/job-posting-schema";
import type { Formatters } from "@/lib/i18n/formatters";
import { useFormatters } from "@/lib/i18n/use-formatters";

type StepState = "done" | "current" | "upcoming" | "blocked";
type Step = { label: string; detail: string; state: StepState };

// The part of a posting the path needs; null while a new posting has not been saved yet.
export type ProgressPosting = {
  status: PostingStatus;
  review_submitted: boolean;
  core_question_count: number;
  interview_slot?: number | null;
} | null;

function steps(
  posting: ProgressPosting,
  generating: boolean,
  fmt: Formatters,
): Step[] {
  const count = posting?.core_question_count ?? 0;
  const live =
    posting != null &&
    posting.status !== "pending" &&
    posting.status !== "rejected";
  const inReview = posting?.status === "pending" && posting.review_submitted;
  const rejected = posting?.status === "rejected";
  const hasBank = count > 0;

  const requirements: Step = posting
    ? { label: "Requirements", detail: "Reviewed and saved", state: "done" }
    : {
        label: "Requirements",
        detail: "Review what RoleCue extracted",
        state: "current",
      };
  const bank: Step = generating
    ? {
        label: "Questions",
        detail: "Generating questions…",
        state: "current",
      }
    : hasBank
      ? {
          label: "Questions",
          detail: `${fmt.number(count)} ${count === 1 ? "question" : "questions"}`,
          state: "done",
        }
      : {
          label: "Questions",
          detail: posting ? "Not generated yet" : "Generated after you confirm",
          state: posting ? "current" : "upcoming",
        };
  const review: Step = live
    ? { label: "Admin review", detail: "Approved", state: "done" }
    : rejected
      ? { label: "Admin review", detail: "Changes requested", state: "blocked" }
      : inReview
        ? {
            label: "Admin review",
            detail: "Waiting for an administrator",
            state: "current",
          }
        : {
            label: "Admin review",
            detail: hasBank ? "Ready to submit" : "Not submitted",
            state: "upcoming",
          };
  const funded = posting?.interview_slot != null;
  const evaluation: Step = {
    label: "Evaluation",
    detail:
      live || inReview
        ? "Criteria confirmed"
        : "Optional: defaults apply unless you customize",
    state:
      live || inReview
        ? "done"
        : hasBank && !generating
          ? "current"
          : "upcoming",
  };
  const funding: Step = {
    label: "Fund slots",
    detail: funded
      ? "Slots funded"
      : posting?.status === "open"
        ? "Fund slots to publish"
        : live
          ? "Never funded"
          : "After admin approval",
    state: funded
      ? "done"
      : posting?.status === "open"
        ? "current"
        : "upcoming",
  };
  // Screening is the recruiter's work once candidates can apply: it runs while the posting takes
  // CVs and while approved candidates still interview, and it ends with the posting.
  const screening: Step = !posting
    ? { label: "Screening", detail: "After funding", state: "upcoming" }
    : posting.status === "closed"
      ? { label: "Screening", detail: "Closed", state: "done" }
      : posting.status === "intake_closed"
        ? {
            label: "Screening",
            detail: "Intake closed, finishing interviews",
            state: "current",
          }
        : posting.status === "open"
          ? funded
            ? {
                label: "Screening",
                detail: "Taking applications",
                state: "current",
              }
            : { label: "Screening", detail: "After funding", state: "upcoming" }
          : {
              label: "Screening",
              detail: "After approval and funding",
              state: "upcoming",
            };
  return [requirements, bank, evaluation, review, funding, screening];
}

const barClass: Record<StepState, string> = {
  done: "bg-foreground",
  current: "bg-rolecue-brand",
  upcoming: "bg-border",
  blocked: "bg-destructive",
};

// Where the posting is on its way to candidates. Each step says what it holds, so the strip
// replaces the separate "ready", "waiting" and "rejected" notices.
export function PostingProgress({
  posting,
  generating = false,
  className,
}: {
  posting: ProgressPosting;
  generating?: boolean;
  className?: string;
}) {
  const fmt = useFormatters();
  return (
    <ol
      aria-label="Posting progress"
      className={cn(
        "grid grid-cols-2 gap-x-4 gap-y-5 md:grid-cols-3 xl:grid-cols-6",
        className,
      )}
    >
      {steps(posting, generating, fmt).map((step) => (
        <li
          key={step.label}
          aria-current={step.state === "current" ? "step" : undefined}
          className="min-w-0 space-y-2"
        >
          <span
            aria-hidden
            className={cn(
              "block h-1 rounded-full transition-colors duration-[var(--duration-fast)] ease-[var(--ease-smooth-out)]",
              barClass[step.state],
            )}
          />
          <span
            className={cn(
              "block text-sm font-medium",
              step.state === "upcoming" && "text-muted-foreground",
            )}
          >
            {step.label}
            <span className="sr-only">
              {step.state === "done"
                ? " (done)"
                : step.state === "current"
                  ? " (current step)"
                  : step.state === "blocked"
                    ? " (needs changes)"
                    : ""}
            </span>
          </span>
          <span
            className={cn(
              "block text-[13px] leading-snug [overflow-wrap:anywhere]",
              step.state === "blocked"
                ? "text-foreground"
                : "text-muted-foreground",
            )}
          >
            {generating && step.label === "Questions" ? (
              <span className="t-shimmer" data-text={step.detail}>
                {step.detail}
              </span>
            ) : (
              step.detail
            )}
          </span>
        </li>
      ))}
    </ol>
  );
}
