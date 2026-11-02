"use client";

import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useLocale, useTranslations } from "next-intl";
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
import {
  compareValues,
  formatWindowRange,
  type CompareTranslator,
} from "../lib/compare";
import type { DatesTranslator } from "../lib/dates";
import { encodeCSVRow, formatVND } from "../lib/format";
import { buildTrendRows } from "../lib/revenue-chart";
import { revenuePresets, timeWindowFileSuffix } from "../lib/time-window";
import {
  revenueCompareSchema,
  type RevenueCompare,
  type RevenueReport,
} from "../schemas/billing-schema";
import type { AppLocale } from "@/lib/i18n/config";
import { CoinFlow, OutstandingCoins } from "./coin-flow";
import { RevenueKpis } from "./revenue-kpis";
import { RevenueMix } from "./revenue-mix";
import { RevenueTrendChart } from "./revenue-trend-chart";
import { TimeWindowFilter, useTimeWindow } from "./time-window-filter";

const compareOptions: readonly RevenueCompare[] = [
  "previous_year",
  "previous_period",
];

type RevenueTranslator = ReturnType<
  typeof useTranslations<"admin.billing.revenue">
>;
type CsvTranslator = ReturnType<
  typeof useTranslations<"admin.billing.revenue.csv">
>;

// The one sentence the screen answers: how much came in, and how that sits against the baseline.
function finding(
  report: RevenueReport,
  range: string,
  comparisonRange: string,
  locale: AppLocale,
  t: RevenueTranslator,
  compareT: CompareTranslator,
) {
  const now = report.current.revenue_vnd;
  const before = report.comparison.revenue_vnd;
  if (now === 0 && before === 0) {
    return t("finding.none", { range, comparisonRange });
  }
  const revenue = t("finding.revenue", { range, amount: formatVND(now) });
  const change = compareValues(now, before, "money", locale, compareT);
  if (change.state === "new") {
    return t("finding.newBaseline", { revenue, comparisonRange });
  }
  if (change.state === "same") {
    return t("finding.same", { revenue, comparisonRange });
  }
  return t("finding.change", {
    revenue,
    direction: change.direction,
    percent: change.percent,
    absolute: change.absolute,
    comparisonRange,
  });
}

function downloadReport(
  report: RevenueReport,
  range: string,
  comparisonRange: string,
  suffix: string,
  t: CsvTranslator,
  datesT: DatesTranslator,
) {
  const metric = (label: string, key: keyof RevenueReport["current"]) => [
    label,
    report.current[key],
    report.comparison[key],
  ];
  const rows: (string | number)[][] = [
    [t("period"), range, comparisonRange],
    [t("metric"), t("thisPeriod"), t("comparison")],
    metric(t("revenueVnd"), "revenue_vnd"),
    metric(t("paidTopups"), "successful_topups"),
    metric(t("payosAttempts"), "topup_attempts"),
    metric(t("paymentSuccess"), "payment_success_rate"),
    metric(t("payingUsers"), "paying_users"),
    metric(t("newPayers"), "new_payers"),
    metric(t("averageOrderVnd"), "average_order_vnd"),
    metric(t("coinsSold"), "coins_sold"),
    metric(t("coinsSpent"), "coins_spent"),
    metric(t("coinsRefunded"), "coins_refunded"),
    [],
    [
      t("bucket"),
      t("revenueVnd"),
      t("comparisonBucket"),
      t("comparisonRevenueVnd"),
    ],
    ...buildTrendRows(report, datesT).map((row) => [
      row.label,
      row.current,
      row.comparisonLabel,
      row.comparison,
    ]),
    [],
    [
      t("coinPack"),
      t("revenueVnd"),
      t("comparisonRevenueVnd"),
      t("paidTopups"),
    ],
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
  const locale = useLocale();
  const t = useTranslations("admin.billing.revenue");
  const tCommon = useTranslations("common");
  const csvT = useTranslations("admin.billing.revenue.csv");
  const compareT = useTranslations("admin.billing.compare");
  const datesT = useTranslations("admin.billing.dates");
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
    ? formatWindowRange(report.window.from, report.window.to, datesT)
    : "";
  const comparisonRange = report
    ? formatWindowRange(
        report.comparison_window.from,
        report.comparison_window.to,
        datesT,
      )
    : "";

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            {t("title")}
          </h1>
          {report ? (
            <p className="text-sm text-muted-foreground">
              {finding(report, range, comparisonRange, locale, t, compareT)}
            </p>
          ) : reportQuery.isError ? null : (
            <Skeleton className="h-5 w-80 max-w-full" />
          )}
          <p className="text-xs text-muted-foreground">{t("subtitle")}</p>
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
            onValueChange={(value) =>
              setCompare(
                revenueCompareSchema.catch("previous_year").parse(value),
              )
            }
          >
            <SelectTrigger
              className="h-9 w-full text-xs sm:w-[190px]"
              aria-label={t("compareLabel")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {compareOptions.map((option) => (
                <SelectItem key={option} value={option}>
                  {t(`compareOptions.${option}`)}
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
                csvT,
                datesT,
              )
            }
          >
            <Download className="size-3.5" aria-hidden />
            {t("export")}
          </Button>
        </div>
      </div>

      {reportQuery.isError ? (
        <Card className="items-start gap-2 p-6">
          <p className="text-sm font-medium text-foreground">
            {t("loadError.title")}
          </p>
          <p className="text-sm text-muted-foreground">{t("loadError.hint")}</p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void reportQuery.refetch()}
          >
            {tCommon("actions.retry")}
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
