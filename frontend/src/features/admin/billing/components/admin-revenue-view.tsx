"use client";

import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { adminBillingKeys, getRevenueReport } from "../api/billing";
import { compareValues, formatWindowRange } from "../lib/compare";
import { encodeCSVRow, formatVND } from "../lib/format";
import { buildTrendRows } from "../lib/revenue-chart";
import { revenuePresets, timeWindowFileSuffix } from "../lib/time-window";
import type { RevenueCompare, RevenueReport } from "../schemas/billing-schema";
import { CoinFlow, OutstandingCoins } from "./coin-flow";
import { RevenueKpis } from "./revenue-kpis";
import { RevenueMix } from "./revenue-mix";
import { RevenueTrendChart } from "./revenue-trend-chart";
import { TimeWindowFilter, useTimeWindow } from "./time-window-filter";

const compareOptions: { value: RevenueCompare; label: string }[] = [
  { value: "previous_year", label: "Same period last year" },
  { value: "previous_period", label: "Previous period" },
];

// The one sentence the screen answers: how much came in, and how that sits against the baseline.
function finding(
  report: RevenueReport,
  range: string,
  comparisonRange: string,
) {
  const now = report.current.revenue_vnd;
  const before = report.comparison.revenue_vnd;
  if (now === 0 && before === 0) {
    return `No paid top-ups in ${range} or ${comparisonRange}`;
  }
  const revenue = `${range} · ${formatVND(now)}`;
  const change = compareValues(now, before, "money");
  if (change.state === "new") {
    return `${revenue}; no paid top-ups in ${comparisonRange}`;
  }
  if (change.state === "same") {
    return `${revenue}, the same as ${comparisonRange}`;
  }
  return `${revenue}, ${change.direction} ${change.percent} on ${comparisonRange} (${change.absolute})`;
}

function downloadReport(
  report: RevenueReport,
  range: string,
  comparisonRange: string,
  suffix: string,
) {
  const metric = (label: string, key: keyof RevenueReport["current"]) => [
    label,
    report.current[key],
    report.comparison[key],
  ];
  const rows: (string | number)[][] = [
    ["Period (Vietnam time)", range, comparisonRange],
    ["Metric", "This period", "Comparison"],
    metric("Revenue (VND)", "revenue_vnd"),
    metric("Paid top-ups", "successful_topups"),
    metric("PayOS attempts", "topup_attempts"),
    metric("Payment success (%)", "payment_success_rate"),
    metric("Paying users", "paying_users"),
    metric("New payers", "new_payers"),
    metric("Average order (VND)", "average_order_vnd"),
    metric("Coins sold", "coins_sold"),
    metric("Coins spent", "coins_spent"),
    metric("Coins refunded", "coins_refunded"),
    [],
    [
      "Bucket",
      "Revenue (VND)",
      "Comparison bucket",
      "Comparison revenue (VND)",
    ],
    ...buildTrendRows(report).map((row) => [
      row.label,
      row.current,
      row.comparisonLabel,
      row.comparison,
    ]),
    [],
    ["Coin pack", "Revenue (VND)", "Comparison revenue (VND)", "Paid top-ups"],
    ...report.by_package.map((item) => [
      item.key,
      item.revenue_vnd,
      item.comparison_revenue_vnd,
      item.topups,
    ]),
  ];
  const blob = new Blob([rows.map((row) => encodeCSVRow(row)).join("\n")], {
    type: "text/csv;charset=utf-8;",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `rolecue-revenue${suffix}.csv`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

function LoadingReport() {
  return (
    <div className="space-y-4" aria-hidden>
      <Skeleton className="h-36 w-full rounded-xl" />
      <div className="grid gap-4 lg:grid-cols-3">
        <Skeleton className="h-80 rounded-xl lg:col-span-2" />
        <Skeleton className="h-80 rounded-xl" />
      </div>
    </div>
  );
}

export function AdminRevenueView() {
  const timeWindow = useTimeWindow({
    initial: { preset: "this_month" },
    requireBothEnds: true,
  });
  const [compare, setCompare] = useState<RevenueCompare>("previous_year");
  const { dateFrom, dateTo } = timeWindow.applied;
  const query = {
    dateFrom: dateFrom ?? "",
    dateTo: dateTo ?? "",
    compare,
  };

  const reportQuery = useQuery({
    queryKey: adminBillingKeys.revenueReport(query),
    queryFn: ({ signal }) => getRevenueReport(query, signal),
    enabled: Boolean(dateFrom && dateTo),
    placeholderData: keepPreviousData,
  });
  const report = reportQuery.data;
  const range = report
    ? formatWindowRange(report.window.from, report.window.to)
    : "";
  const comparisonRange = report
    ? formatWindowRange(
        report.comparison_window.from,
        report.comparison_window.to,
      )
    : "";

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Revenue
          </h1>
          {report ? (
            <p className="text-sm text-muted-foreground">
              {finding(report, range, comparisonRange)}
            </p>
          ) : reportQuery.isError ? null : (
            <Skeleton className="h-5 w-80 max-w-full" />
          )}
          <p className="text-xs text-muted-foreground">
            Paid PayOS top-ups only; coins moving inside RoleCue are not
            revenue.
          </p>
        </div>

        <div className="grid grid-cols-1 items-start gap-2 sm:flex sm:flex-wrap">
          <TimeWindowFilter
            value={timeWindow.draft}
            onChange={timeWindow.change}
            error={timeWindow.error}
            presets={revenuePresets}
          />
          <Select
            value={compare}
            onValueChange={(value) => setCompare(value as RevenueCompare)}
          >
            <SelectTrigger
              className="h-9 w-full text-xs sm:w-[190px]"
              aria-label="Compare with"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {compareOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            type="button"
            variant="outline"
            className="h-9 gap-1.5 text-xs"
            disabled={!report}
            onClick={() =>
              report &&
              downloadReport(
                report,
                range,
                comparisonRange,
                timeWindowFileSuffix(timeWindow.applied.window, new Date()),
              )
            }
          >
            <Download className="size-3.5" aria-hidden />
            Export CSV
          </Button>
        </div>
      </div>

      {reportQuery.isError ? (
        <Card className="items-start gap-2 p-6">
          <p className="text-sm font-medium text-foreground">
            Couldn&apos;t load the revenue report.
          </p>
          <p className="text-sm text-muted-foreground">
            Check the connection and try again.
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void reportQuery.refetch()}
          >
            Retry
          </Button>
        </Card>
      ) : !report ? (
        <LoadingReport />
      ) : (
        <div className="space-y-4">
          <RevenueKpis report={report} comparisonRange={comparisonRange} />
          <div className="grid gap-4 lg:grid-cols-3">
            <RevenueTrendChart
              report={report}
              range={range}
              comparisonRange={comparisonRange}
            />
            <RevenueMix
              byRole={report.by_role}
              byPackage={report.by_package}
              total={report.current.revenue_vnd}
            />
          </div>
          <div className="grid gap-4 lg:grid-cols-3">
            <CoinFlow report={report} />
            <OutstandingCoins report={report} />
          </div>
        </div>
      )}
    </div>
  );
}
