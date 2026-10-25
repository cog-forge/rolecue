import type { RoleProfileStatus } from "../schemas/dashboard-schema";

export const roleProfileStatusLabel: Record<RoleProfileStatus, string> = {
  draft: "Draft",
  analyzing: "Analyzing the job description",
  review: "Waiting for your review",
  confirmed: "Ready to practice",
};

// The status shown on practice rows comes from the server's derivation; this only names it.
export const interviewStatusLabel = {
  in_progress: "In progress",
  pending: "Feedback being prepared",
  completed: "Feedback ready",
  failed: "Feedback unavailable",
} as const;

export const applicationStatusLabel = {
  pending: "Awaiting CV review",
  interview_eligible: "Interview eligible",
  interviewed: "Interview completed",
  approved: "Approved",
  rejected: "Not selected",
} as const;

const dateFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});

export function formatDate(iso: string) {
  return dateFormat.format(new Date(iso));
}
