import { cn } from "@/lib/utils";
import type { PostingStatus } from "../schemas/job-posting-schema";

type StepState = "done" | "current" | "upcoming" | "blocked";
type Step = { label: string; detail: string; state: StepState };

// The part of a posting the path needs; null while a new posting has not been saved yet.
export type ProgressPosting = {
  status: PostingStatus;
  review_submitted: boolean;
  core_question_count: number;
  interview_slot?: number | null;
} | null;

const numberFormat = new Intl.NumberFormat("en");

function steps(posting: ProgressPosting, generating: boolean): Step[] {
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
        label: "Question bank",
        detail: "Generating questions…",
        state: "current",
      }
    : hasBank
      ? {
          label: "Question bank",
          detail: `${numberFormat.format(count)} ${count === 1 ? "question" : "questions"}`,
          state: "done",
        }
      : {
          label: "Question bank",
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
  const liveDetail: Record<PostingStatus, string> = {
    open: "Open to candidates",
    intake_closed: "Intake closed",
    closed: "Closed",
    pending: "After approval",
    rejected: "After approval",
  };
  const funded = posting?.interview_slot != null;
  const evaluation: Step = {
    label: "Evaluation",
    detail:
      live || inReview ? "Criteria confirmed" : "Use defaults or customize",
    state:
      live || inReview
        ? "done"
        : hasBank && !generating
          ? "current"
          : "upcoming",
  };
  const funding: Step = {
    label: "Fund & publish",
    detail: funded ? "Slots funded" : "After admin approval",
    state: funded
      ? "done"
      : live && posting?.status === "open"
        ? "current"
        : "upcoming",
  };
  const goLive: Step = {
    label: "Live",
    detail:
      posting?.status === "open" && !funded
        ? "After funding"
        : posting
          ? liveDetail[posting.status]
          : "After funding",
    state: live && funded ? "done" : "upcoming",
  };
  return [requirements, bank, evaluation, review, funding, goLive];
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
  return (
    <ol
      aria-label="Posting progress"
      className={cn(
        "grid grid-cols-2 gap-x-4 gap-y-5 md:grid-cols-3 xl:grid-cols-6",
        className,
      )}
    >
      {steps(posting, generating).map((step) => (
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
            {generating && step.label === "Question bank" ? (
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
