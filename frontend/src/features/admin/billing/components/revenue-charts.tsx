"use client";

import { Coins, Package } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVND, formatCoins, formatPlural } from "../lib/format";
import type { BillingSummary } from "../schemas/billing-schema";

type RevenueChartsProps = {
  summary?: BillingSummary;
  isLoading: boolean;
};

export function RevenueCharts({ summary, isLoading }: RevenueChartsProps) {
  if (isLoading || !summary) {
    return (
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Skeleton className="h-72 w-full rounded-xl" />
        <Skeleton className="h-72 w-full rounded-xl" />
      </div>
    );
  }

  const packages = summary.package_breakdown ?? [];
  const features = summary.feature_breakdown ?? [];

  const totalPkgRevenue = packages.reduce((acc, p) => acc + p.revenue_vnd, 0);
  const totalFeatureCoins = features.reduce((acc, f) => acc + f.coins_spent, 0);

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      {/* Chart 1: Revenue by Coin Package (Real PayOS Data Only) */}
      <Card className="border-border/80 shadow-xs">
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2 text-sm font-semibold">
              <Package className="size-4 text-emerald-600 dark:text-emerald-400" />
              Revenue by Coin Package
            </CardTitle>
            <span className="text-xs text-muted-foreground">
              Verified PayOS deposits
            </span>
          </div>
        </CardHeader>
        <CardContent className="pt-2">
          {packages.length === 0 ? (
            <div className="flex h-48 flex-col items-center justify-center rounded-lg border border-dashed border-border/70 p-6 text-center">
              <p className="text-sm font-medium text-foreground">
                No package revenue data
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                No successful PayOS top-up transactions match the current
                filters.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {packages.map((pkg) => {
                const sharePct =
                  totalPkgRevenue > 0
                    ? Math.round((pkg.revenue_vnd / totalPkgRevenue) * 1000) /
                      10
                    : 0;
                return (
                  <div key={pkg.package_name} className="space-y-1.5">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-medium text-foreground">
                        {pkg.package_name}
                      </span>
                      <span className="font-mono font-semibold text-foreground tabular-nums">
                        {formatVND(pkg.revenue_vnd)} ({sharePct}%)
                      </span>
                    </div>
                    <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full rounded-full bg-emerald-500 transition-all"
                        style={{ width: `${Math.min(100, sharePct)}%` }}
                      />
                    </div>
                    <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                      <span>
                        {formatPlural(pkg.deposit_count, "deposit", "deposits")}
                      </span>
                      <span>{formatCoins(pkg.coins_sold)} issued</span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Chart 2: Coin Economy & Feature Consumption (Real Ledger Data Only) */}
      <Card className="border-border/80 shadow-xs">
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2 text-sm font-semibold">
              <Coins className="size-4 text-amber-600 dark:text-amber-400" />
              Coin Consumption by Feature
            </CardTitle>
            <span className="text-xs text-muted-foreground">
              Sold: {formatCoins(summary.coins_sold)} · Spent:{" "}
              {formatCoins(summary.coins_spent)}
            </span>
          </div>
        </CardHeader>
        <CardContent className="pt-2">
          {features.length === 0 ? (
            <div className="flex h-48 flex-col items-center justify-center rounded-lg border border-dashed border-border/70 p-6 text-center">
              <p className="text-sm font-medium text-foreground">
                No coin spending data
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                No internal coin spending transactions match the current
                filters.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {features.map((feat) => {
                const sharePct =
                  totalFeatureCoins > 0
                    ? Math.round(
                        (feat.coins_spent / totalFeatureCoins) * 1000,
                      ) / 10
                    : 0;
                return (
                  <div key={feat.feature_name} className="space-y-1.5">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-medium text-foreground">
                        {feat.feature_name}
                      </span>
                      <span className="font-mono font-semibold text-foreground tabular-nums">
                        {formatCoins(feat.coins_spent)} ({sharePct}%)
                      </span>
                    </div>
                    <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full rounded-full bg-amber-500 transition-all"
                        style={{ width: `${Math.min(100, sharePct)}%` }}
                      />
                    </div>
                    <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                      <span>
                        {formatPlural(feat.spend_count, "charge", "charges")}
                      </span>
                      <span>Internal ledger debit</span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
