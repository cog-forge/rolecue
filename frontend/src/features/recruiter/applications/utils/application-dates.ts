import type { Formatters } from "@/lib/i18n/formatters";

// One place for how the Applications list and the Application detail print dates.
export function formatSubmitted(iso: string, fmt: Formatters) {
  return fmt.date(iso, "calendarDay");
}

// The deadline is a moment, so it is shown in the viewer's own time zone, named to avoid misreading.
export function formatDeadline(iso: string, fmt: Formatters) {
  return fmt.date(iso, "moment");
}

// An eligible Application the server no longer clears to interview has passed its deadline. The
// server's expiry sweep records that as rejected shortly after; the detail shows it as rejected now.
export function isDeadlineMissed(application: {
  status: string;
  interview_eligible: boolean;
}) {
  return (
    application.status === "interview_eligible" &&
    !application.interview_eligible
  );
}
