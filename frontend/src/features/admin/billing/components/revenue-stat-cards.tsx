"use client";

import {
  TrendingUp,
  CheckCircle2,
  Clock,
  XCircle,
  ReceiptText,
  ArrowUpRight,
  ArrowDownRight,
  Minus,
  HelpCircle,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { BillingSummary, RevenueStats } from "../schemas/billing-schema";
import { formatVND, plural, formatPercentChange } from "../lib/format";

export { formatVND };

interface StatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  sub?: string;
  tooltip?: string;
  accent?: "emerald" | "amber" | "destructive" | "muted" | "blue" | "purple";
  changePct?: number;
  showComparison?: boolean;
}

function StatCard({
  icon,
  label,
  value,
  sub,
  tooltip,
  accent = "muted",
  changePct,
  showComparison = false,
}: StatCardProps) {
  const accentMap = {
    emerald:
      "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
    amber:
      "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
    destructive: "bg-destructive/10 text-destructive border-destructive/20",
    muted: "bg-muted text-muted-foreground border-border/60",
    blue: "bg-[#185cc0]/10 text-[#185cc0] dark:text-[#8ab4f8] border-[#185cc0]/20",
    purple:
      "bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20",
  };

  const pct = formatPercentChange(changePct);

  return (
    <Card className="border-border/70 shadow-xs transition-all duration-200 hover:shadow-md hover:border-border">
      <CardContent className="p-4.5">
        <div className="flex items-start justify-between gap-2.5">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <p className="text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                {label}
              </p>
              {tooltip && (
                <TooltipProvider delayDuration={200}>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        className="text-muted-foreground/60 hover:text-muted-foreground focus:outline-hidden"
                        aria-label={`About ${label}`}
                      >
                        <HelpCircle className="h-3 w-3" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="top" className="text-xs max-w-xs">
                      {tooltip}
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              )}
            </div>

            <div className="mt-1.5 flex items-baseline gap-2">
              <span className="text-2xl font-bold tracking-tight text-foreground tabular-nums">
                {value}
              </span>
              {showComparison && changePct != null && (
                <span
                  className={`inline-flex items-center text-xs font-medium px-1.5 py-0.5 rounded-full ${
                    pct.isNeutral
                      ? "bg-muted text-muted-foreground"
                      : pct.isPositive
                        ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                        : "bg-destructive/10 text-destructive"
                  }`}
                  title={`${pct.text} compared to previous period`}
                >
                  {pct.isNeutral ? (
                    <Minus className="h-3 w-3 mr-0.5" />
                  ) : pct.isPositive ? (
                    <ArrowUpRight className="h-3 w-3 mr-0.5" />
                  ) : (
                    <ArrowDownRight className="h-3 w-3 mr-0.5" />
                  )}
                  {pct.text}
                </span>
              )}
            </div>

            {sub && (
              <p className="mt-1 text-xs text-muted-foreground truncate">
                {sub}
              </p>
            )}
          </div>

          <span
            className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border ${accentMap[accent]}`}
          >
            {icon}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}

export function StatCardSkeleton() {
  return (
    <Card className="border-border/70 shadow-xs">
      <CardContent className="p-4.5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-3.5 w-24" />
            <Skeleton className="h-7 w-32" />
            <Skeleton className="h-3 w-20" />
          </div>
          <Skeleton className="h-9 w-9 shrink-0 rounded-xl" />
        </div>
      </CardContent>
    </Card>
  );
}

export function RevenueStatCards({
  stats,
  summary,
  isLoading,
  showComparison = false,
  periodLabel,
}: {
  stats?: RevenueStats;
  summary?: BillingSummary;
  isLoading: boolean;
  showComparison?: boolean;
  periodLabel?: string;
}) {
  if (isLoading) {
    return (
      <div className="grid gap-3.5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <StatCardSkeleton key={i} />
        ))}
      </div>
    );
  }

  // Support both unified BillingSummary and legacy RevenueStats
  if (summary) {
    const netRevenue = summary.net_revenue;
    const successfulCount = summary.successful_deposits;
    const pendingCount = summary.pending_count;
    const failedCount = summary.failed_count;
    const otherCount = summary.other_count; // Cancelled + Refunds
    const totalCount = summary.total_transactions;
    const comparison = summary.comparison;

    const cards: StatCardProps[] = [
      {
        icon: <TrendingUp className="h-4 w-4" />,
        label: "Total Revenue",
        value: formatVND(netRevenue),
        sub:
          summary.refund_count > 0
            ? `${formatVND(summary.gross_revenue)} gross - ${formatVND(summary.total_refunds_vnd)} refunds`
            : (periodLabel ?? "Net successful top-ups"),
        tooltip: "Net revenue = successful PayOS deposits minus refunds",
        accent: "blue",
        changePct: comparison?.net_revenue_change_pct,
        showComparison,
      },
      {
        icon: <CheckCircle2 className="h-4 w-4" />,
        label: "Successful",
        value: successfulCount.toLocaleString(),
        sub: `${plural(successfulCount, "deposit")} (${summary.success_rate.toFixed(1)}% rate)`,
        tooltip:
          "Successful top-up deposits only (coin spending and refunds excluded)",
        accent: "emerald",
        changePct: comparison?.successful_deposits_change_pct,
        showComparison,
      },
      {
        icon: <Clock className="h-4 w-4" />,
        label: "Pending",
        value: pendingCount.toLocaleString(),
        sub:
          pendingCount > 0
            ? "Awaiting PayOS confirmation"
            : "No pending deposits",
        tooltip:
          "Pending orders awaiting payment gateway or webhook confirmation",
        accent: "amber",
        changePct: comparison?.pending_count_change_pct,
        showComparison,
      },
      {
        icon: <XCircle className="h-4 w-4" />,
        label: "Failed",
        value: failedCount.toLocaleString(),
        sub: "Declined or system errors",
        tooltip: "Failed deposit attempts",
        accent: "destructive",
        changePct: comparison?.failed_count_change_pct,
        showComparison,
      },
      {
        icon: <ReceiptText className="h-4 w-4" />,
        label: "Total Transactions",
        value: totalCount.toLocaleString(),
        sub: `${otherCount > 0 ? `${otherCount} other · ` : ""}${periodLabel ?? "All filtered"}`,
        tooltip: `Sum of all statuses: ${successfulCount} successful + ${pendingCount} pending + ${failedCount} failed + ${otherCount} other = ${totalCount} total`,
        accent: "muted",
        showComparison: false,
      },
    ];

    return (
      <div className="grid gap-3.5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        {cards.map((card) => (
          <StatCard key={card.label} {...card} />
        ))}
      </div>
    );
  }

  if (stats) {
    const cards: StatCardProps[] = [
      {
        icon: <TrendingUp className="h-4 w-4" />,
        label: "Total Revenue",
        value: formatVND(stats.total_revenue),
        sub: `${plural(stats.successful_count, "deposit")}`,
        accent: "blue",
      },
      {
        icon: <CheckCircle2 className="h-4 w-4" />,
        label: "Successful",
        value: stats.successful_count.toLocaleString(),
        sub: "Completed top-ups",
        accent: "emerald",
      },
      {
        icon: <Clock className="h-4 w-4" />,
        label: "Pending",
        value: stats.pending_count.toLocaleString(),
        sub: "Awaiting confirmation",
        accent: "amber",
      },
      {
        icon: <XCircle className="h-4 w-4" />,
        label: "Failed",
        value: stats.failed_count.toLocaleString(),
        sub: "Declined or failed",
        accent: "destructive",
      },
      {
        icon: <ReceiptText className="h-4 w-4" />,
        label: "Total Transactions",
        value: stats.total_count.toLocaleString(),
        sub: periodLabel ?? "All time",
        accent: "muted",
      },
    ];

    return (
      <div className="grid gap-3.5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        {cards.map((card) => (
          <StatCard key={card.label} {...card} />
        ))}
      </div>
    );
  }

  return null;
}
