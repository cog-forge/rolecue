"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Download, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { adminBillingKeys, getBillingSummary } from "../api/billing";
import { BillingStatCards } from "./revenue-stat-cards";
import { RevenueCharts } from "./revenue-charts";
import { formatVND, formatCoins, formatPlural } from "../lib/format";

function sanitizeCSVCell(val: string): string {
  if (!val) return "";
  if (/^[=+\-@\t\r]/.test(val)) {
    return `'${val}`;
  }
  return val;
}

export function AdminRevenueView() {
  // Uses the exact same shared summary endpoint as the Payment Transactions page.
  const summaryQuery = useQuery({
    queryKey: adminBillingKeys.summary({}),
    queryFn: ({ signal }) => getBillingSummary({}, signal),
  });

  const summary = summaryQuery.data;

  const handleExportSummaryCSV = () => {
    if (!summary) return;
    const rows: string[][] = [
      ["Package Name", "Revenue (VND)", "Successful Deposits", "Coins Sold"],
      ...(summary.package_breakdown ?? []).map((p) => [
        sanitizeCSVCell(p.package_name),
        String(p.revenue_vnd),
        String(p.deposit_count),
        String(p.coins_sold),
      ]),
      [
        "TOTAL",
        String(summary.total_revenue),
        String(summary.successful_deposits),
        String(summary.coins_sold),
      ],
    ];
    const csvContent = rows
      .map((r) =>
        r.map((cell) => `"${String(cell).replace(/"/g, '""')}"`).join(","),
      )
      .join("\n");

    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "rolecue-revenue-report.csv";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  return (
    <div className="space-y-6">
      {/* Header & Controls */}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Revenue Reports
          </h1>
          <p className="text-sm text-muted-foreground">
            Verified PayOS deposit revenue and coin economy breakdown. Revenue
            includes only{" "}
            <code className="rounded bg-muted px-1 py-0.5 text-xs">
              status = &apos;success&apos;
            </code>{" "}
            transactions with a PayOS order code (KAN-192).
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={!summary}
            onClick={handleExportSummaryCSV}
            className="h-8 gap-1.5 text-xs"
          >
            <Download className="size-3.5" />
            Export Report CSV
          </Button>
          <Link
            href="/admin/billing"
            className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-background px-3 text-xs font-medium text-foreground shadow-xs hover:bg-muted"
          >
            View All Transactions
            <ExternalLink className="size-3.5" />
          </Link>
        </div>
      </div>

      {summaryQuery.isError ? (
        <div
          role="alert"
          className="flex flex-col items-center justify-center gap-3 rounded-xl border border-destructive/40 bg-destructive/5 p-8 text-center"
        >
          <AlertCircle className="size-8 text-destructive" />
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-foreground">
              Unable to load revenue report
            </h2>
            <p className="text-xs text-muted-foreground">
              Please check your connection and try again.
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void summaryQuery.refetch()}
          >
            Retry
          </Button>
        </div>
      ) : (
        <>
          {/* KPI Summary Cards */}
          <BillingStatCards
            summary={summary}
            isLoading={summaryQuery.isLoading}
            periodLabel="All-time ledger"
            variant="revenue"
          />

          {/* Real Package & Feature Charts (Honest Empty States if None) */}
          <RevenueCharts summary={summary} isLoading={summaryQuery.isLoading} />

          {/* Package Revenue Breakdown Table */}
          <Card className="border-border/80 shadow-xs">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-semibold">
                  Package Revenue Breakdown
                </CardTitle>
                <span className="text-xs text-muted-foreground">
                  Source of truth: successful PayOS top-up transactions
                </span>
              </div>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="text-xs font-semibold">
                      Package
                    </TableHead>
                    <TableHead className="text-right text-xs font-semibold">
                      Successful Deposits
                    </TableHead>
                    <TableHead className="text-right text-xs font-semibold">
                      Coins Sold
                    </TableHead>
                    <TableHead className="text-right text-xs font-semibold">
                      Revenue (VND)
                    </TableHead>
                    <TableHead className="text-right text-xs font-semibold">
                      Drill-down
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {!summary || summary.package_breakdown.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={5}
                        className="h-32 text-center text-sm text-muted-foreground"
                      >
                        No successful PayOS deposit transactions recorded.
                      </TableCell>
                    </TableRow>
                  ) : (
                    <>
                      {summary.package_breakdown.map((pkg) => (
                        <TableRow key={pkg.package_name}>
                          <TableCell className="text-xs font-medium text-foreground">
                            {pkg.package_name}
                          </TableCell>
                          <TableCell className="text-right font-mono text-xs tabular-nums">
                            {formatPlural(
                              pkg.deposit_count,
                              "deposit",
                              "deposits",
                            )}
                          </TableCell>
                          <TableCell className="text-right font-mono text-xs tabular-nums">
                            {formatCoins(pkg.coins_sold)}
                          </TableCell>
                          <TableCell className="text-right font-mono text-xs font-semibold text-foreground tabular-nums">
                            {formatVND(pkg.revenue_vnd)}
                          </TableCell>
                          <TableCell className="text-right">
                            <Link
                              href="/admin/billing"
                              className="inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"
                            >
                              Inspect
                              <ExternalLink className="size-3" />
                            </Link>
                          </TableCell>
                        </TableRow>
                      ))}
                      {/* Totals Footer Row */}
                      <TableRow className="bg-muted/40 font-semibold">
                        <TableCell className="text-xs">Total</TableCell>
                        <TableCell className="text-right font-mono text-xs tabular-nums">
                          {formatPlural(
                            summary.successful_deposits,
                            "deposit",
                            "deposits",
                          )}
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs tabular-nums">
                          {formatCoins(summary.coins_sold)}
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs tabular-nums">
                          {formatVND(summary.total_revenue)}
                        </TableCell>
                        <TableCell />
                      </TableRow>
                    </>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
