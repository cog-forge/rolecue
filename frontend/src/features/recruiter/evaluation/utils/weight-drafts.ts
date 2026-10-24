import type {
  Metric,
  PostingWeights,
  WeightsBody,
} from "../schemas/evaluation-schema";

// Go caps a single weight at the total, or at 1000 when there is no sum rule.
export const NO_RULE_LIMIT = 1000;
// Go compares the sum with the total to within this much (the percentages are doubles).
const TOLERANCE = 1e-9;

// One text value per metric, keyed by metric id. A metric the saved set leaves out starts empty.
export type WeightDrafts = Record<string, string>;

export function weightDrafts(
  metrics: readonly Metric[],
  saved: PostingWeights,
): WeightDrafts {
  const byMetric = new Map(saved.weights.map((w) => [w.metric_id, w]));
  return Object.fromEntries(
    metrics.map((metric) => {
      const weight = byMetric.get(metric.id);
      return [metric.id, weight ? String(weight.percentage) : ""];
    }),
  );
}

export function formatPercent(value: number) {
  return String(Math.round(value * 100) / 100);
}

// Shows the sum next to the total it must reach. Two-decimal rounding would turn a sum of 99.996
// into "100" beside a total of 100, so it shows more digits when the rounded values would match.
export function formatSum(sum: number, total: number) {
  return formatPercent(sum) === formatPercent(total) && sum !== total
    ? String(Number(sum.toFixed(6)))
    : formatPercent(sum);
}

export type DraftEvaluation = {
  sum: number;
  // A message per metric whose value cannot be sent.
  fieldErrors: Record<string, string>;
  // Set when every field is fine but the set as a whole cannot be saved.
  formError: string | null;
  valid: boolean;
  // Only metrics above 0% are sent: a metric left out is simply not weighted.
  body: WeightsBody | null;
};

// Checks the drafts the way the Go service will, so the Recruiter sees the problem next to the
// field instead of after a round trip. Go stays the authority; this never replaces its answer.
export function evaluateDrafts(
  metrics: readonly Metric[],
  drafts: WeightDrafts,
  total: number,
): DraftEvaluation {
  const limit = total > 0 ? total : NO_RULE_LIMIT;
  const fieldErrors: Record<string, string> = {};
  const weights: WeightsBody["weights"] = [];
  let sum = 0;
  for (const metric of metrics) {
    const raw = (drafts[metric.id] ?? "").trim();
    if (raw === "") continue;
    const value = Number(raw);
    if (!Number.isFinite(value)) {
      fieldErrors[metric.id] = "Enter a number.";
    } else if (value < 0) {
      fieldErrors[metric.id] = "A weight can't be negative.";
    } else if (value > limit) {
      fieldErrors[metric.id] =
        `A weight can be at most ${formatPercent(limit)}.`;
    } else {
      sum += value;
      if (value > 0) weights.push({ metric_id: metric.id, percentage: value });
    }
  }
  let formError: string | null = null;
  if (Object.keys(fieldErrors).length === 0) {
    if (weights.length === 0) {
      formError = "Give at least one metric a weight.";
    } else if (total > 0 && Math.abs(sum - total) > TOLERANCE) {
      formError = `The weights add up to ${formatSum(sum, total)}, but they must add up to ${formatPercent(total)}.`;
    }
  }
  const valid = Object.keys(fieldErrors).length === 0 && formError === null;
  return {
    sum,
    fieldErrors,
    formError,
    valid,
    body: valid ? { weights } : null,
  };
}
