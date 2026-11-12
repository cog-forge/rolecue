import type { Report, ReportMetric } from "../schemas/report-schema";

// Display helpers for the rubric score explanation (KAN-337). Nothing here recomputes a score: every
// number comes from the server, which owns the calculation. These only arrange it for reading.

/** Formats a stored score with its two decimals in the reader's locale: 62 -> "62.00" / "62,00". */
export function formatScore(
  value: number,
  number: (value: number, options?: Intl.NumberFormatOptions) => string,
) {
  return number(value, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

/** The metrics the overall is weighted over: a positive weight. */
export function weightedMetrics(breakdown: readonly ReportMetric[]) {
  return breakdown.filter((metric) => (metric.weight ?? 0) > 0);
}

/** Weighted metrics no graded question assessed: the reason an overall is withheld. */
export function uncoveredMetrics(breakdown: readonly ReportMetric[]) {
  return weightedMetrics(breakdown).filter((metric) => metric.score === null);
}

/** A metric's share of the total weight, in percent (weights are relative). */
export function weightShare(weight: number, total: number) {
  return total > 0 ? (weight / total) * 100 : 0;
}

/**
 * The overall's calculation written out with the stored values, for example
 * "(70.00 × 60 + 50.00 × 40) ÷ 100 = 62.00". Null when there is no overall to explain.
 */
export function overallFormula(
  report: Pick<Report, "overall_score" | "breakdown" | "scoring">,
  number: (value: number, options?: Intl.NumberFormatOptions) => string,
) {
  const { overall_score: overall, breakdown, scoring } = report;
  if (overall === null || !breakdown || !scoring || scoring.method !== "rubric")
    return null;
  const terms = weightedMetrics(breakdown).filter(
    (metric) => metric.score !== null,
  );
  if (terms.length === 0) return null;
  const total = terms.reduce((sum, metric) => sum + (metric.weight ?? 0), 0);
  const sum = terms
    .map(
      (metric) =>
        `${formatScore(metric.score ?? 0, number)} × ${number(metric.weight ?? 0)}`,
    )
    .join(" + ");
  return `(${sum}) ÷ ${number(total)} = ${formatScore(overall, number)}`;
}
