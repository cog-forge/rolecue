"use client";

import { useState, useMemo } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import {
  ChartNoAxesCombined,
  CreditCard,
  RefreshCw,
  TrendingUp,
  Calendar,
  AlertCircle,
  Coins,
  Download,
  ExternalLink,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { routes } from "@/config/routes";
import { adminBillingKeys, getAdminRevenueReport } from "../api/billing";
import type { DatePreset, BillingDailyItem } from "../schemas/billing-schema";
import { RevenueStatCards } from "./revenue-stat-cards";
import { RevenueCharts } from "./revenue-charts";
import { DatePresetPicker, getPresetDates } from "./date-preset-picker";
import { formatVND, formatDateOnly, plural } from "../lib/format";

export function AdminRevenueView() {
  const [preset, setPreset] = useState<DatePreset>("30d");
  const initialDates = getPresetDates("30d");
  const [dateFrom, setDateFrom] = useState(initialDates.dateFrom);
  const [dateTo, setDateTo] = useState(initialDates.dateTo);
  const [showComparison, setShowComparison] = useState(false);
  const [sortField, setSortField] = useState<
    "date" | "revenue" | "deposit_count"
  >("date");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("desc");

  // Primary authoritative revenue report query (matches /admin/revenue)
  const revenueQuery = useQuery({
    queryKey: adminBillingKeys.revenue(dateFrom, dateTo),
    queryFn: ({ signal }) => getAdminRevenueReport(dateFrom, dateTo, signal),
    staleTime: 30_000,
  });

  const handlePresetChange = (newPreset: DatePreset) => {
    setPreset(newPreset);
    if (newPreset !== "custom") {
      const dates = getPresetDates(newPreset);
      setDateFrom(dates.dateFrom);
      setDateTo(dates.dateTo);
    }
  };

  const handleCustomDateChange = (from: string, to: string) => {
    setPreset("custom");
    setDateFrom(from ? `${from}T00:00:00+07:00` : "");
    setDateTo(to ? `${to}T23:59:59+07:00` : "");
  };

  const report = revenueQuery.data;
  const stats = report?.stats;
  // Adapt to BillingDailyItem shape for recharts and breakdown table
  const rawDailyList: BillingDailyItem[] = useMemo(() => {
    const summaryList = report?.summary ?? [];
    return summaryList.map((s) => ({
      date: s.period.split("T")[0],
      revenue: s.revenue,
      gross_revenue: s.revenue,
      refunds_vnd: 0,
      deposit_count: s.successful_deposits,
      unique_payers: s.successful_deposits,
      coins_sold: Math.round(s.revenue / 500),
      coins_spent: 0,
      packages_breakdown: {},
      features_breakdown: {},
    }));
  }, [report?.summary]);

  // Sorting daily items
  const sortedDaily = useMemo(() => {
    return [...rawDailyList].sort((a, b) => {
      let cmp = 0;
      if (sortField === "date") cmp = a.date.localeCompare(b.date);
      else if (sortField === "revenue") cmp = a.revenue - b.revenue;
      else if (sortField === "deposit_count")
        cmp = a.deposit_count - b.deposit_count;
      return sortDirection === "asc" ? cmp : -cmp;
    });
  }, [rawDailyList, sortField, sortDirection]);

  // Derived metrics
  const dailyTotalDeposits = rawDailyList.reduce(
    (acc, d) => acc + d.deposit_count,
    0,
  );
  const dailyTotalNet = rawDailyList.reduce((acc, d) => acc + d.revenue, 0);
  const activeDays = rawDailyList.filter((d) => d.revenue > 0).length;
  const avgDailyRevenue =
    activeDays > 0 ? Math.round(dailyTotalNet / activeDays) : 0;

  const peakDay = useMemo(() => {
    return rawDailyList.reduce<BillingDailyItem | null>((peak, current) => {
      if (!peak || current.revenue > peak.revenue) return current;
      return peak;
    }, null);
  }, [rawDailyList]);

  // CSV Export for Daily Breakdown
  function exportDailyCSV() {
    const headers = [
      "Date (UTC+7)",
      "Successful Deposits",
      "Gross Revenue (VND)",
      "Refunds (VND)",
      "Net Revenue (VND)",
    ];

    const rows = sortedDaily.map((d) => [
      formatDateOnly(d.date),
      d.deposit_count,
      d.gross_revenue || d.revenue,
      d.refunds_vnd || 0,
      d.revenue,
    ]);

    const csvContent =
      "\uFEFF" +
      [headers.join(","), ...rows.map((r) => r.join(","))].join("\r\n");

    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.setAttribute(
      "download",
      `revenue-daily-${(dateFrom || "all").slice(0, 10)}-to-${(dateTo || "all").slice(0, 10)}.csv`,
    );
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }

  const periodDaysCount = rawDailyList.length;

  return (
    <div className="mx-auto max-w-7xl space-y-6">
      {/* Header */}
      <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <p className="text-xs font-semibold tracking-wider text-workspace-accent uppercase">
            Finance
          </p>
          <div className="flex items-center gap-2.5">
            <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#185cc0]/10 text-[#185cc0] dark:text-[#8ab4f8] border border-[#185cc0]/20">
              <ChartNoAxesCombined className="h-5 w-5" aria-hidden />
            </span>
            <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              Revenue Reports
            </h1>
          </div>
          <p className="max-w-2xl text-xs sm:text-sm text-muted-foreground">
            Track PayOS top-up revenue, coin circulation liability, deposit
            conversion rates, and daily financial performance.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            asChild
            variant="outline"
            size="sm"
            className="h-9 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
          >
            <Link href={routes.admin.billing}>
              <CreditCard className="h-3.5 w-3.5" aria-hidden />
              <span>Payment Transactions</span>
            </Link>
          </Button>

          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={exportDailyCSV}
            className="h-9 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            title="Download Daily Breakdown CSV"
          >
            <Download className="h-3.5 w-3.5" aria-hidden />
            <span>Export CSV</span>
          </Button>

          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void revenueQuery.refetch()}
            disabled={revenueQuery.isFetching}
            className="h-9 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            aria-label="Refresh revenue data"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${revenueQuery.isFetching ? "animate-spin" : ""}`}
              aria-hidden
            />
            <span>{revenueQuery.isFetching ? "Refreshing…" : "Refresh"}</span>
          </Button>
        </div>
      </header>

      {/* Period Controls & Comparison Toggle */}
      <section
        aria-labelledby="period-controls-heading"
        className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-xs space-y-3"
      >
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-2.5">
            <div className="flex items-center gap-1.5 text-muted-foreground">
              <Calendar className="h-4 w-4" aria-hidden />
              <h2
                id="period-controls-heading"
                className="text-xs font-semibold tracking-wider uppercase"
              >
                Period
              </h2>
            </div>
            <DatePresetPicker value={preset} onChange={handlePresetChange} />
          </div>

          {/* Comparison toggle */}
          <div className="flex items-center gap-2 text-xs text-muted-foreground bg-muted/40 px-3 py-1.5 rounded-lg border border-border/50 self-start lg:self-auto">
            <Switch
              id="compare-prev"
              checked={showComparison}
              onCheckedChange={setShowComparison}
            />
            <label
              htmlFor="compare-prev"
              className="cursor-pointer font-medium select-none text-xs"
            >
              Compare to previous period
            </label>
          </div>
        </div>

        {/* Custom date range inputs when Custom selected */}
        {preset === "custom" && (
          <div className="grid gap-3 pt-3 sm:grid-cols-2 border-t border-border/40">
            <div className="flex flex-col gap-1">
              <label
                htmlFor="rev-date-from"
                className="text-xs font-medium text-muted-foreground"
              >
                From Date
              </label>
              <Input
                id="rev-date-from"
                type="date"
                className="h-9 bg-background text-xs"
                value={dateFrom ? dateFrom.split("T")[0] : ""}
                onChange={(e) =>
                  handleCustomDateChange(
                    e.target.value,
                    dateTo ? dateTo.split("T")[0] : "",
                  )
                }
              />
            </div>
            <div className="flex flex-col gap-1">
              <label
                htmlFor="rev-date-to"
                className="text-xs font-medium text-muted-foreground"
              >
                To Date
              </label>
              <Input
                id="rev-date-to"
                type="date"
                className="h-9 bg-background text-xs"
                value={dateTo ? dateTo.split("T")[0] : ""}
                onChange={(e) =>
                  handleCustomDateChange(
                    dateFrom ? dateFrom.split("T")[0] : "",
                    e.target.value,
                  )
                }
              />
            </div>
          </div>
        )}
      </section>

      {/* Query Error State */}
      {revenueQuery.error ? (
        <div
          role="alert"
          className="flex flex-col items-center justify-center rounded-xl border border-destructive/20 bg-destructive/5 p-8 text-center"
        >
          <AlertCircle className="h-8 w-8 text-destructive" aria-hidden />
          <h3 className="mt-3 text-sm font-semibold text-foreground">
            Unable to load revenue report
          </h3>
          <p className="mt-1 max-w-sm text-xs text-muted-foreground">
            {revenueQuery.error instanceof Error
              ? revenueQuery.error.message
              : "Failed to fetch revenue statistics. Please retry."}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void revenueQuery.refetch()}
            className="mt-4 gap-1.5 text-xs"
          >
            <RefreshCw className="h-3.5 w-3.5" aria-hidden />
            Retry
          </Button>
        </div>
      ) : (
        <>
          {/* Primary Financial Metric Cards */}
          <section aria-labelledby="key-metrics-heading" className="space-y-3">
            <h2 id="key-metrics-heading" className="sr-only">
              Key Financial Metrics
            </h2>
            <RevenueStatCards
              stats={stats}
              isLoading={revenueQuery.isLoading}
              showComparison={showComparison}
              periodLabel={`${periodDaysCount} days in period`}
            />
          </section>

          {/* Secondary Insight Metric Cards */}
          <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {/* Average Active Day Revenue */}
            <Card className="border-border/70 shadow-xs">
              <CardHeader className="p-4.5 pb-2">
                <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                  Avg Active Day Revenue
                </p>
              </CardHeader>
              <CardContent className="p-4.5 pt-0">
                <div className="flex items-center gap-2.5">
                  <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                    <Coins className="h-4.5 w-4.5" />
                  </span>
                  <div>
                    <p className="text-xl font-bold text-foreground tabular-nums">
                      {formatVND(avgDailyRevenue)}
                    </p>
                    <p className="text-[11px] text-muted-foreground">
                      Across {activeDays} active day
                      {activeDays === 1 ? "" : "s"} in this period
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Peak Revenue Day */}
            <Card className="border-border/70 shadow-xs">
              <CardHeader className="p-4.5 pb-2">
                <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                  Peak Revenue Day
                </p>
              </CardHeader>
              <CardContent className="p-4.5 pt-0">
                <div className="flex items-center gap-2.5">
                  <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">
                    <TrendingUp className="h-4.5 w-4.5" />
                  </span>
                  <div>
                    <p className="text-xl font-bold text-foreground tabular-nums">
                      {peakDay && peakDay.revenue > 0
                        ? formatVND(peakDay.revenue)
                        : "—"}
                    </p>
                    <p className="text-[11px] text-muted-foreground truncate max-w-[150px]">
                      {peakDay && peakDay.revenue > 0
                        ? formatDateOnly(peakDay.date)
                        : "No deposits recorded in period"}
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Audit & Verification Note */}
            <Card className="border-border/70 shadow-xs sm:col-span-2 lg:col-span-1">
              <CardHeader className="p-4.5 pb-2">
                <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                  Audit & Verification
                </p>
              </CardHeader>
              <CardContent className="p-4.5 pt-0 space-y-2">
                <p className="text-xs text-muted-foreground leading-relaxed">
                  Verified against PayOS payment records. All deposits and
                  refunds are authenticated with PayOS gateway signatures.
                </p>
                <Button
                  asChild
                  variant="link"
                  className="h-auto p-0 text-xs font-medium text-workspace-accent gap-1"
                >
                  <Link href={routes.admin.billing}>
                    View raw transactions ledger
                    <ExternalLink className="h-3 w-3" />
                  </Link>
                </Button>
              </CardContent>
            </Card>
          </section>

          {/* Visual Recharts Charts */}
          <section aria-labelledby="visual-charts-heading">
            <h2 id="visual-charts-heading" className="sr-only">
              Visual Financial Analytics
            </h2>
            <RevenueCharts
              dailyData={rawDailyList}
              isLoading={revenueQuery.isLoading}
              showComparison={showComparison}
              periodLabel={`${periodDaysCount} calendar days reported`}
            />
          </section>

          {/* Daily Breakdown Table */}
          <section aria-labelledby="breakdown-heading" className="space-y-3">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h2
                  id="breakdown-heading"
                  className="text-base font-bold text-foreground"
                >
                  Daily Breakdown
                </h2>
                <p className="text-xs text-muted-foreground">
                  All UTC+7 calendar days in range including zero-revenue days
                </p>
              </div>
              <span className="text-xs text-muted-foreground">
                {periodDaysCount} day{periodDaysCount === 1 ? "" : "s"} reported
              </span>
            </div>

            <div className="rounded-xl border border-border/70 bg-card overflow-hidden shadow-xs">
              <div className="overflow-x-auto">
                <table
                  className="min-w-full text-left text-sm border-collapse"
                  aria-label="Daily revenue breakdown"
                >
                  <thead className="bg-muted/40 border-b border-border/70 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    <tr>
                      <th
                        scope="col"
                        className="px-5 py-3.5 cursor-pointer hover:text-foreground select-none"
                        onClick={() => {
                          if (sortField === "date")
                            setSortDirection((d) =>
                              d === "asc" ? "desc" : "asc",
                            );
                          else {
                            setSortField("date");
                            setSortDirection("desc");
                          }
                        }}
                      >
                        Date{" "}
                        {sortField === "date" &&
                          (sortDirection === "asc" ? "↑" : "↓")}
                      </th>
                      <th
                        scope="col"
                        className="px-5 py-3.5 text-right cursor-pointer hover:text-foreground select-none"
                        onClick={() => {
                          if (sortField === "deposit_count")
                            setSortDirection((d) =>
                              d === "asc" ? "desc" : "asc",
                            );
                          else {
                            setSortField("deposit_count");
                            setSortDirection("desc");
                          }
                        }}
                      >
                        Successful Deposits{" "}
                        {sortField === "deposit_count" &&
                          (sortDirection === "asc" ? "↑" : "↓")}
                      </th>
                      <th
                        scope="col"
                        className="px-5 py-3.5 text-right cursor-pointer hover:text-foreground select-none"
                        onClick={() => {
                          if (sortField === "revenue")
                            setSortDirection((d) =>
                              d === "asc" ? "desc" : "asc",
                            );
                          else {
                            setSortField("revenue");
                            setSortDirection("desc");
                          }
                        }}
                      >
                        Revenue (VND){" "}
                        {sortField === "revenue" &&
                          (sortDirection === "asc" ? "↑" : "↓")}
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border/40 text-xs">
                    {sortedDaily.length === 0 ? (
                      <tr>
                        <td
                          colSpan={3}
                          className="px-5 py-8 text-center text-muted-foreground"
                        >
                          No daily breakdown records in this period.
                        </td>
                      </tr>
                    ) : (
                      <>
                        {/* Summary Total Row */}
                        {periodDaysCount > 2 && (
                          <tr className="bg-muted/20 font-bold border-b border-border/60">
                            <td className="px-5 py-3 text-foreground">
                              Total ({periodDaysCount} days)
                            </td>
                            <td className="px-5 py-3 text-right text-foreground tabular-nums">
                              {plural(dailyTotalDeposits, "deposit")}
                            </td>
                            <td className="px-5 py-3 text-right font-bold text-foreground tabular-nums text-sm">
                              {formatVND(dailyTotalNet)}
                            </td>
                          </tr>
                        )}

                        {sortedDaily.map((row) => (
                          <tr
                            key={row.date}
                            className="hover:bg-muted/20 transition-colors"
                          >
                            <td className="px-5 py-3 font-medium text-foreground">
                              {formatDateOnly(row.date)}
                            </td>
                            <td className="px-5 py-3 text-right text-muted-foreground">
                              {row.deposit_count > 0 ? (
                                <span>
                                  {plural(row.deposit_count, "deposit")}
                                </span>
                              ) : (
                                <span className="text-muted-foreground/50">
                                  No deposits
                                </span>
                              )}
                            </td>
                            <td className="px-5 py-3 text-right font-semibold tabular-nums">
                              {row.revenue > 0 ? (
                                <span className="text-emerald-700 dark:text-emerald-400">
                                  +{formatVND(row.revenue)}
                                </span>
                              ) : (
                                <span className="text-muted-foreground/60">
                                  —
                                </span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  );
}
