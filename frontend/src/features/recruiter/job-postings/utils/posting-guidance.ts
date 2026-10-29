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

type Action = { label: string; href: string };

// The one thing the Recruiter most likely does next, as a link into the screen that does it. It
// only points; the rules stay with the screens and the server.
export function primaryAction(posting: JobPosting): Action | null {
  const hasBank = posting.core_question_count > 0;
  switch (posting.status) {
    case "pending":
      if (posting.review_submitted) return null;
      return hasBank
        ? {
            label: "Review & submit",
            href: routes.recruiter.jobPostingQuestions(posting.id),
          }
        : { label: "Review requirements", href: "#posting-requirements" };
    case "rejected":
      return hasBank
        ? {
            label: "Edit and resubmit",
            href: routes.recruiter.jobPostingQuestions(posting.id),
          }
        : { label: "Review requirements", href: "#posting-requirements" };
    case "open":
      return posting.interview_slot === null
        ? { label: "Fund & publish", href: "#interview-capacity" }
        : {
            label: "Review applications",
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
  capacity?: CapacityState,
) {
  switch (posting.status) {
    case "pending":
      return posting.review_submitted ? "Waiting for admin review" : null;
    case "rejected":
      return "Rejected: see the reason below";
    case "open":
      if (posting.interview_slot === null)
        return "Approved: fund interview slots to publish";
      return capacity?.interview_slot != null
        ? `${capacity.cv_approvals} of ${capacity.interview_slot} interview slots approved`
        : null;
    case "intake_closed":
      if (!capacity) return null;
      if (capacity.interview_slot === null)
        return "Intake closed: slots were never funded";
      return capacity.cv_approvals >= capacity.interview_slot
        ? `Intake closed: all ${capacity.interview_slot} slots approved`
        : `Intake closed: ${capacity.cv_approvals} of ${capacity.interview_slot} slots approved`;
    case "closed":
      if (!capacity) return null;
      if (capacity.interview_slot === null)
        return "Closed without funded slots";
      if (capacity.refunded === null) return null;
      return capacity.refunded > 0
        ? `Closed: ${fmt.coins(capacity.refunded)} from unused slots returned to your wallet`
        : "Closed: no unused slots to return";
  }
}
