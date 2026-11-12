import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { PostingStatus } from "../schemas/job-posting-schema";

type StepState = "done" | "current" | "upcoming" | "blocked";
type StepId =
  "requirements" | "bank" | "evaluation" | "review" | "funding" | "screening";
type Step = { id: StepId; detail: string; state: StepState };
type ProgressTranslator = ReturnType<
  typeof useTranslations<"recruiter.postings.progress">
>;

// The part of a posting the path needs; null while a new posting has not been saved yet.
export type ProgressPosting = {
  status: PostingStatus;
  review_submitted: boolean;
  core_question_count: number;
  grading_ready?: boolean;
  interview_slot?: number | null;
} | null;

function steps(
  posting: ProgressPosting,
  generating: boolean,
  t: ProgressTranslator,
): Step[] {
  const count = posting?.core_question_count ?? 0;
  const live =
    posting != null &&
    posting.status !== "pending" &&
    posting.status !== "rejected";
  const inReview = posting?.status === "pending" && posting.review_submitted;
  const rejected = posting?.status === "rejected";
  const hasBank = count > 0;
  const gradingReady = posting?.grading_ready ?? hasBank;

  const requirements: Step = posting
    ? {
        id: "requirements",
        detail: t("detail.requirementsSaved"),
        state: "done",
      }
    : {
        id: "requirements",
        detail: t("detail.requirementsReview"),
        state: "current",
      };
  const bank: Step = generating
    ? { id: "bank", detail: t("detail.generating"), state: "current" }
    : hasBank
      ? {
          id: "bank",
          detail: gradingReady
            ? t("detail.questionCount", { count })
            : t("detail.gradingIncomplete"),
          state: gradingReady ? "done" : "current",
        }
      : {
          id: "bank",
          detail: posting
            ? t("detail.notGenerated")
            : t("detail.generatedAfterConfirm"),
          state: posting ? "current" : "upcoming",
        };
  const review: Step = live
    ? { id: "review", detail: t("detail.approved"), state: "done" }
    : rejected
      ? { id: "review", detail: t("detail.changesRequested"), state: "blocked" }
      : inReview
        ? {
            id: "review",
            detail: t("detail.waitingForAdmin"),
            state: "current",
          }
        : {
            id: "review",
            detail: gradingReady
              ? t("detail.readyToSubmit")
              : t("detail.notSubmitted"),
            state: "upcoming",
          };
  const funded = posting?.interview_slot != null;
  const evaluation: Step = {
    id: "evaluation",
    detail:
      live || inReview
        ? t("detail.criteriaConfirmed")
        : t("detail.criteriaOptional"),
    state:
      live || inReview
        ? "done"
        : hasBank && gradingReady && !generating
          ? "current"
          : "upcoming",
  };
  const funding: Step = {
    id: "funding",
    detail: funded
      ? t("detail.slotsFunded")
      : posting?.status === "open"
        ? t("detail.fundToPublish")
        : live
          ? t("detail.neverFunded")
          : t("detail.afterApproval"),
    state: funded
      ? "done"
      : posting?.status === "open"
        ? "current"
        : "upcoming",
  };
  // Screening is the recruiter's work once candidates can apply: it runs while the posting takes
  // CVs and while approved candidates still interview, and it ends with the posting.
  const screening: Step = !posting
    ? { id: "screening", detail: t("detail.afterFunding"), state: "upcoming" }
    : posting.status === "closed"
      ? { id: "screening", detail: t("detail.closed"), state: "done" }
      : posting.status === "intake_closed"
        ? {
            id: "screening",
            detail: t("detail.intakeClosed"),
            state: "current",
          }
        : posting.status === "open"
          ? funded
            ? {
                id: "screening",
                detail: t("detail.takingApplications"),
                state: "current",
              }
            : {
                id: "screening",
                detail: t("detail.afterFunding"),
                state: "upcoming",
              }
          : {
              id: "screening",
              detail: t("detail.afterApprovalAndFunding"),
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
  const t = useTranslations("recruiter.postings.progress");
  return (
    <ol
      aria-label={t("label")}
      className={cn(
        "grid grid-cols-2 gap-x-4 gap-y-5 md:grid-cols-3 xl:grid-cols-6",
        className,
      )}
    >
      {steps(posting, generating, t).map((step) => (
        <li
          key={step.id}
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
            {t(`step.${step.id}`)}
            <span className="sr-only">
              {step.state === "upcoming" ? "" : ` ${t(`state.${step.state}`)}`}
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
            {generating && step.id === "bank" ? (
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
