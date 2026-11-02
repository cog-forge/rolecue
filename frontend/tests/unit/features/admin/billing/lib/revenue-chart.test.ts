import { describe, expect, it } from "vitest";
import { buildTrendRows } from "@/features/admin/billing/lib/revenue-chart";
import { octoberReport } from "../fixtures/revenue-report";
import { datesFor } from "../fixtures/translators";

const monthly = {
  ...octoberReport,
  granularity: "month" as const,
  series: [
    {
      start: "2025-11-01T00:00:00+07:00",
      comparison_start: "2024-11-01T00:00:00+07:00",
      revenue_vnd: 200000,
      comparison_revenue_vnd: 0,
    },
  ],
};

describe("buildTrendRows", () => {
  it("pairs each day with the same day last year", () => {
    const rows = buildTrendRows(octoberReport, datesFor("en"));
    expect(rows).toHaveLength(27);
    expect(rows[4]).toEqual({
      key: "2026-10-05T00:00:00+07:00",
      tick: "5",
      label: "5 Oct 2026",
      comparisonLabel: "5 Oct 2025",
      current: 100000,
      comparison: 0,
    });
    expect(rows[2]).toMatchObject({ current: 0, comparison: 50000 });
  });

  it("labels month buckets by month", () => {
    const rows = buildTrendRows(monthly, datesFor("en"));
    expect(rows[0]).toMatchObject({
      tick: "Nov",
      label: "Nov 2025",
      comparisonLabel: "Nov 2024",
    });
  });

  it("names days and months in Vietnamese", () => {
    const vi = datesFor("vi");
    expect(buildTrendRows(octoberReport, vi)[4]).toMatchObject({
      tick: "5",
      label: "5 thg 10, 2026",
      comparisonLabel: "5 thg 10, 2025",
    });
    expect(buildTrendRows(monthly, vi)[0]).toMatchObject({
      tick: "thg 11",
      label: "thg 11, 2025",
      comparisonLabel: "thg 11, 2024",
    });
  });
});
