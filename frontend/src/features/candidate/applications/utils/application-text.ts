import type { CandidateApplicationStatus } from "../schemas/candidate-application-schema";

export const statusLabel: Record<CandidateApplicationStatus, string> = {
  pending: "Awaiting CV review",
  interview_eligible: "Interview eligible",
  interviewed: "Interview completed",
  approved: "Approved",
  rejected: "Not selected",
};

const submittedFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});
// The deadline is a moment, so it is shown in the viewer's own time zone, named to avoid misreading.
const deadlineFormat = new Intl.DateTimeFormat("en", {
  year: "numeric",
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  timeZoneName: "short",
});

export function formatSubmitted(iso: string) {
  return submittedFormat.format(new Date(iso));
}

export function formatDeadline(iso: string) {
  return deadlineFormat.format(new Date(iso));
}

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;

// "3 h 20 min left", "12 min left"; the countdown never reads "0 min" while time remains.
export function timeLeftText(deadline: string, now: number) {
  const remaining = new Date(deadline).getTime() - now;
  if (remaining <= 0) return "Deadline passed";
  if (remaining < MINUTE) return "Less than a minute left";
  const hours = Math.floor(remaining / HOUR);
  const minutes = Math.floor((remaining % HOUR) / MINUTE);
  if (hours === 0) return `${minutes} min left`;
  return `${hours} h ${minutes} min left`;
}

export function isDeadlinePassed(deadline: string, now: number) {
  return new Date(deadline).getTime() <= now;
}
