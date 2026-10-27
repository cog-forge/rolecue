import type { ReportStatus, ReportType } from "../schemas/report-schema";

export const reportTypeLabel: Record<ReportType, string> = {
  practice: "Practice interview",
  recruitment: "Recruitment interview",
};

export const reportStatusLabel: Record<ReportStatus, string> = {
  in_progress: "In progress",
  pending: "Evaluating",
  completed: "Completed",
  failed: "Evaluation failed",
};

const dateTimeFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
});

export function formatInterviewTime(iso: string) {
  return dateTimeFormat.format(new Date(iso));
}

export function scoreText(score: number | null) {
  return score === null ? "No score yet" : `${score} / 100`;
}
