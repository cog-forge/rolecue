"use client";

import type {
  RevenueSummaryItem,
  BillingDailyItem,
} from "../schemas/billing-schema";
import { RevenueCharts } from "./revenue-charts";

export { RevenueCharts };

export function RevenueSummaryChart({
  summary,
  dailyData,
  isLoading,
  showComparison = false,
  periodLabel,
}: {
  summary?: RevenueSummaryItem[];
  dailyData?: BillingDailyItem[];
  isLoading: boolean;
  showComparison?: boolean;
  periodLabel?: string;
}) {
  // If modern dailyData is provided, use it
  if (dailyData && dailyData.length > 0) {
    return (
      <RevenueCharts
        dailyData={dailyData}
        isLoading={isLoading}
        showComparison={showComparison}
        periodLabel={periodLabel}
      />
    );
  }

  // Convert legacy summary to dailyData shape
  const adapted: BillingDailyItem[] = (summary || []).map((s) => ({
    date: s.period.split("T")[0],
    revenue: s.revenue,
    gross_revenue: s.revenue,
    refunds_vnd: 0,
    deposit_count: s.successful_deposits,
    unique_payers: s.successful_deposits,
    coins_sold: s.revenue / 500,
    coins_spent: 0,
    packages_breakdown: {},
    features_breakdown: {},
  }));

  return (
    <RevenueCharts
      dailyData={adapted}
      isLoading={isLoading}
      showComparison={showComparison}
      periodLabel={periodLabel}
    />
  );
}
