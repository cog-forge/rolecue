import type { Formatters } from "@/lib/i18n/formatters";
import type { CandidateApplicationStatus } from "../schemas/candidate-application-schema";

export const statusLabel: Record<CandidateApplicationStatus, string> = {
  pending: "Awaiting CV review",
  interview_eligible: "Interview eligible",
  interviewed: "Interview completed",
  approved: "Approved",
  rejected: "Not selected",
};

export function formatSubmitted(iso: string, fmt: Formatters) {
  return fmt.date(iso, "calendarDay");
}

// The deadline is a moment, so it is shown in the viewer's own time zone, named to avoid misreading.
export function formatDeadline(iso: string, fmt: Formatters) {
  return fmt.date(iso, "moment");
}

// "3 h 20 min left", "12 min left"; the countdown never reads "0 min" while time remains.
export function timeLeftText(deadline: string, now: number, fmt: Formatters) {
  return fmt.timeLeft(deadline, now);
}

export function isDeadlinePassed(deadline: string, now: number) {
  return new Date(deadline).getTime() <= now;
}
