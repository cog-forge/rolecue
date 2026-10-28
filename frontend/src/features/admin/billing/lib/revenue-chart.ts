import type { RevenueReport } from "../schemas/billing-schema";

// One chart row per bucket: this period's revenue beside the same bucket of the comparison window.
export type TrendRow = {
  key: string;
  tick: string;
  label: string;
  comparisonLabel: string;
  current: number;
  comparison: number;
};

const VIETNAM_OFFSET_MS = 7 * 60 * 60 * 1000;
const months = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

function bucketLabel(iso: string, granularity: RevenueReport["granularity"]) {
  const date = new Date(Date.parse(iso) + VIETNAM_OFFSET_MS);
  const month = months[date.getUTCMonth()];
  const year = date.getUTCFullYear();
  if (granularity === "month") {
    return { tick: month, label: `${month} ${year}` };
  }
  const day = date.getUTCDate();
  return { tick: String(day), label: `${day} ${month} ${year}` };
}

export function buildTrendRows(report: RevenueReport): TrendRow[] {
  return report.series.map((point) => {
    const current = bucketLabel(point.start, report.granularity);
    return {
      key: point.start,
      tick: current.tick,
      label: current.label,
      comparisonLabel: bucketLabel(point.comparison_start, report.granularity)
        .label,
      current: point.revenue_vnd,
      comparison: point.comparison_revenue_vnd,
    };
  });
}
