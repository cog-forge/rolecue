import type { Formatters } from "@/lib/i18n/formatters";
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

export function formatInterviewTime(iso: string, fmt: Formatters) {
  return fmt.date(iso, "localDateTime");
}

export function scoreText(score: number | null) {
  return score === null ? "No score yet" : `${score} / 100`;
}
