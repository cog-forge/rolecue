import type { RevenueReport } from "../schemas/billing-schema";
import { monthName, vietnamParts, type DatesTranslator } from "./dates";

// One chart row per bucket: this period's revenue beside the same bucket of the comparison window.
export type TrendRow = {
  key: string;
  tick: string;
  label: string;
  comparisonLabel: string;
  current: number;
  comparison: number;
};

function bucketLabel(
  iso: string,
  granularity: RevenueReport["granularity"],
  t: DatesTranslator,
) {
  const { day, month, year } = vietnamParts(Date.parse(iso));
  const name = monthName(month, t);
  if (granularity === "month") {
    return { tick: name, label: t("monthYear", { month: name, year }) };
  }
  return {
    tick: String(day),
    label: t("dayMonthYear", { day, month: name, year }),
  };
}

export function buildTrendRows(
  report: RevenueReport,
  t: DatesTranslator,
): TrendRow[] {
  return report.series.map((point) => {
    const current = bucketLabel(point.start, report.granularity, t);
    return {
      key: point.start,
      tick: current.tick,
      label: current.label,
      comparisonLabel: bucketLabel(
        point.comparison_start,
        report.granularity,
        t,
      ).label,
      current: point.revenue_vnd,
      comparison: point.comparison_revenue_vnd,
    };
  });
}
