import type { useTranslations } from "next-intl";
import type { Formatters } from "@/lib/i18n/formatters";
import { routes } from "@/config/routes";
import type { CapacityState } from "@/features/recruiter/capacity/schemas/capacity-schema";
import {
  applicationSearch,
  defaultApplicationFilters,
  type ApplicationStatusFilter,
} from "@/features/recruiter/applications/utils/application-filters";
import type { JobPosting } from "../schemas/job-posting-schema";

// The review inbox narrowed to one posting, optionally to the CVs still awaiting a decision.
export function postingInboxHref(
  postingId: string,
  status: ApplicationStatusFilter = "all",
) {
  return `${routes.recruiter.applications}${applicationSearch({
    ...defaultApplicationFilters,
    status,
    posting: postingId,
  })}`;
}

// Pending and rejected postings take no applications, so there is nothing to review yet.
export function takesApplications(status: JobPosting["status"]) {
  return status !== "pending" && status !== "rejected";
}

export type GuidanceTranslator = ReturnType<
  typeof useTranslations<"recruiter.postings.guidance">
>;

type Action = { label: string; href: string };

// The one thing the Recruiter most likely does next, as a link into the screen that does it. It
// only points; the rules stay with the screens and the server.
export function primaryAction(
  posting: JobPosting,
  t: GuidanceTranslator,
): Action | null {
  const hasBank = posting.core_question_count > 0;
  switch (posting.status) {
    case "pending":
      if (posting.review_submitted) return null;
      return hasBank
        ? {
            label: t("action.reviewAndSubmit"),
            href: routes.recruiter.jobPostingQuestions(posting.id),
          }
        : {
            label: t("action.reviewRequirements"),
            href: "#posting-requirements",
          };
    case "rejected":
      return hasBank
        ? {
            label: t("action.editAndResubmit"),
            href: routes.recruiter.jobPostingQuestions(posting.id),
          }
        : {
            label: t("action.reviewRequirements"),
            href: "#posting-requirements",
          };
    case "open":
      return posting.interview_slot === null
        ? { label: t("action.fundAndPublish"), href: "#interview-capacity" }
        : {
            label: t("action.reviewApplications"),
            href: postingInboxHref(posting.id),
          };
    default:
      return null;
  }
}

// One line of state for the header, for the statuses where a number or a next step helps. The
// slot figures come from the capacity state; without it the line stays out rather than guess.
export function statusLine(
  posting: JobPosting,
  fmt: Formatters,
  t: GuidanceTranslator,
  capacity?: CapacityState,
) {
  switch (posting.status) {
    case "pending":
      return posting.review_submitted ? t("line.waitingForReview") : null;
    case "rejected":
      return t("line.rejected");
    case "open":
      if (posting.interview_slot === null) return t("line.approvedUnfunded");
      return capacity?.interview_slot != null
        ? t("line.slotsApproved", {
            approved: capacity.cv_approvals,
            slots: capacity.interview_slot,
          })
        : null;
    case "intake_closed":
      if (!capacity) return null;
      if (capacity.interview_slot === null) return t("line.intakeNeverFunded");
      return capacity.cv_approvals >= capacity.interview_slot
        ? t("line.intakeAllApproved", { slots: capacity.interview_slot })
        : t("line.intakePartlyApproved", {
            approved: capacity.cv_approvals,
            slots: capacity.interview_slot,
          });
    case "closed":
      if (!capacity) return null;
      if (capacity.interview_slot === null) return t("line.closedNeverFunded");
      if (capacity.refunded === null) return null;
      return capacity.refunded > 0
        ? t("line.closedRefunded", { coins: fmt.coins(capacity.refunded) })
        : t("line.closedNoRefund");
  }
}

// The one sentence under the title: what happens next for this posting.
export function nextStep(posting: JobPosting, t: GuidanceTranslator) {
  switch (posting.status) {
    case "pending":
      if (posting.review_submitted) return t("next.waitingForReview");
      return posting.core_question_count > 0
        ? t("next.bankReady")
        : t("next.confirmRequirements");
    case "rejected":
      return t("next.rejected");
    default:
      return posting.status === "open" && posting.interview_slot === null
        ? t("next.approvedUnfunded")
        : t("next.approved");
  }
}
