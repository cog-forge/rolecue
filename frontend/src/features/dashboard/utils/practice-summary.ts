import type { InterviewHistoryItem } from "../schemas/dashboard-schema";

export function recentPractice(items: readonly InterviewHistoryItem[]) {
  return [...items]
    .filter((item) => item.type === "practice")
    .sort((a, b) => Date.parse(b.started_at) - Date.parse(a.started_at));
}

// Partial evaluations have a different denominator; never mix them into the complete-score trend.
// A null outcome is the existing API's legacy evaluation, not a missing score.
export function hasCompleteScore(
  item: InterviewHistoryItem,
): item is InterviewHistoryItem & { score: number } {
  return (
    item.status === "completed" &&
    item.score !== null &&
    item.score_outcome !== "partial" &&
    item.score_outcome !== "withheld"
  );
}
