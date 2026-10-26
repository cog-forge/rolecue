"use client";

import {
  CheckCircle2,
  Clock,
  Coins,
  DollarSign,
  Layers,
  Users,
  XCircle,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVND, formatCoins, formatPlural } from "../lib/format";
import type { BillingSummary } from "../schemas/billing-schema";

type BillingStatCardsProps = {
  summary?: BillingSummary;
  isLoading: boolean;
  periodLabel?: string;
  variant?: "transactions" | "revenue";
};

export function BillingStatCards({
  summary,
  isLoading,
  periodLabel = "Filtered view",
  variant = "transactions",
}: BillingStatCardsProps) {
  if (isLoading || !summary) {
    return (
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <Card key={i} className="border-border/80 shadow-xs">
            <CardContent className="p-5">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="mt-3 h-7 w-36" />
              <Skeleton className="mt-2 h-3.5 w-24" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  if (variant === "revenue") {
    return (
      <div className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {/* Card 1: Total Revenue (Success PayOS Deposits Only per KAN-192) */}
          <Card className="border-border/80 shadow-xs">
            <CardContent className="p-5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
                  Total Revenue (PayOS)
                </span>
                <div className="rounded-lg bg-emerald-500/10 p-2 text-emerald-600 dark:text-emerald-400">
                  <DollarSign className="size-4" />
                </div>
              </div>
              <p className="mt-2 text-2xl font-bold tracking-tight text-foreground tabular-nums">
                {formatVND(summary.total_revenue)}
              </p>
              <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                <span>
                  {formatPlural(
                    summary.successful_deposits,
                    "successful deposit",
                    "successful deposits",
                  )}
                </span>
                <span className="font-medium text-foreground/70">
                  {periodLabel}
                </span>
              </div>
            </CardContent>
          </Card>

          {/* Card 2: Successful Deposits & Average Deposit */}
          <Card className="border-border/80 shadow-xs">
            <CardContent className="p-5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
                  Successful Deposits
                </span>
                <div className="rounded-lg bg-blue-500/10 p-2 text-blue-600 dark:text-blue-400">
                  <CheckCircle2 className="size-4" />
                </div>
              </div>
              <p className="mt-2 text-2xl font-bold tracking-tight text-foreground tabular-nums">
                {formatPlural(
                  summary.successful_deposits,
                  "deposit",
                  "deposits",
                )}
              </p>
              <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                <span>Avg: {formatVND(summary.average_deposit)}</span>
                <span>{summary.success_rate.toFixed(1)}% success rate</span>
              </div>
            </CardContent>
          </Card>

          {/* Card 3: Unique Paying Users */}
          <Card className="border-border/80 shadow-xs">
            <CardContent className="p-5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
                  Unique Payers
                </span>
                <div className="rounded-lg bg-violet-500/10 p-2 text-violet-600 dark:text-violet-400">
                  <Users className="size-4" />
                </div>
              </div>
              <p className="mt-2 text-2xl font-bold tracking-tight text-foreground tabular-nums">
                {formatPlural(summary.unique_payers, "payer", "payers")}
              </p>
              <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                <span>Verified PayOS top-up users</span>
                <span>{periodLabel}</span>
              </div>
            </CardContent>
          </Card>

          {/* Card 4: Coin Economy Summary */}
          <Card className="border-border/80 shadow-xs">
            <CardContent className="p-5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
                  Coins Sold / Spent
                </span>
                <div className="rounded-lg bg-amber-500/10 p-2 text-amber-600 dark:text-amber-400">
                  <Coins className="size-4" />
                </div>
              </div>
              <p className="mt-2 text-2xl font-bold tracking-tight text-foreground tabular-nums">
                {formatCoins(summary.coins_sold)}
              </p>
              <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                <span>Spent: {formatCoins(summary.coins_spent)}</span>
                <span>Refunded: {formatCoins(summary.coins_refunded)}</span>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Secondary Status Breakdown Row (Sums to Total Transactions) */}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
          <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
            <span className="text-[11px] font-medium text-muted-foreground uppercase">
              Total Ledger Rows
            </span>
            <p className="mt-1 text-lg font-semibold tabular-nums">
              {formatPlural(
                summary.total_transactions,
                "transaction",
                "transactions",
              )}
            </p>
          </div>
          <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
            <span className="text-[11px] font-medium text-emerald-600 uppercase dark:text-emerald-400">
              Success (All Types)
            </span>
            <p className="mt-1 text-lg font-semibold tabular-nums">
              {summary.success_count.toLocaleString("vi-VN")}
            </p>
          </div>
          <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
            <span className="text-[11px] font-medium text-amber-600 uppercase dark:text-amber-400">
              Pending
            </span>
            <p className="mt-1 text-lg font-semibold tabular-nums">
              {summary.pending_count.toLocaleString("vi-VN")}
            </p>
          </div>
          <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
            <span className="text-[11px] font-medium text-rose-600 uppercase dark:text-rose-400">
              Failed
            </span>
            <p className="mt-1 text-lg font-semibold tabular-nums">
              {summary.failed_count.toLocaleString("vi-VN")}
            </p>
          </div>
          <div className="rounded-xl border border-border/70 bg-card px-4 py-3">
            <span className="text-[11px] font-medium text-slate-600 uppercase dark:text-slate-400">
              Cancelled
            </span>
            <p className="mt-1 text-lg font-semibold tabular-nums">
              {summary.cancelled_count.toLocaleString("vi-VN")}
            </p>
          </div>
        </div>
      </div>
    );
  }

  // Default variant: "transactions" (5 cards whose statuses sum to total_transactions)
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
      {/* Total Revenue */}
      <Card className="border-border/80 shadow-xs">
        <CardContent className="p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
              Total Revenue
            </span>
            <div className="rounded-lg bg-emerald-500/10 p-2 text-emerald-600 dark:text-emerald-400">
              <DollarSign className="size-4" />
            </div>
          </div>
          <p className="mt-2 text-xl font-bold tracking-tight text-foreground tabular-nums">
            {formatVND(summary.total_revenue)}
          </p>
          <p className="mt-1.5 text-xs text-muted-foreground">
            {formatPlural(
              summary.successful_deposits,
              "successful deposit",
              "successful deposits",
            )}{" "}
            · {periodLabel}
          </p>
        </CardContent>
      </Card>

      {/* Successful */}
      <Card className="border-border/80 shadow-xs">
        <CardContent className="p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
              Successful
            </span>
            <div className="rounded-lg bg-emerald-500/10 p-2 text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="size-4" />
            </div>
          </div>
          <p className="mt-2 text-xl font-bold tracking-tight text-foreground tabular-nums">
            {formatPlural(summary.successful_deposits, "deposit", "deposits")}
          </p>
          <p className="mt-1.5 text-xs text-muted-foreground">
            {summary.success_count.toLocaleString("vi-VN")} total success rows ·
            Avg {formatVND(summary.average_deposit)}
          </p>
        </CardContent>
      </Card>

      {/* Pending */}
      <Card className="border-border/80 shadow-xs">
        <CardContent className="p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
              Pending
            </span>
            <div className="rounded-lg bg-amber-500/10 p-2 text-amber-600 dark:text-amber-400">
              <Clock className="size-4" />
            </div>
          </div>
          <p className="mt-2 text-xl font-bold tracking-tight text-foreground tabular-nums">
            {summary.pending_count.toLocaleString("vi-VN")}
          </p>
          <p className="mt-1.5 text-xs text-muted-foreground">
            Awaiting PayOS webhook / user sync
          </p>
        </CardContent>
      </Card>

      {/* Failed & Cancelled */}
      <Card className="border-border/80 shadow-xs">
        <CardContent className="p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
              Failed / Cancelled
            </span>
            <div className="rounded-lg bg-rose-500/10 p-2 text-rose-600 dark:text-rose-400">
              <XCircle className="size-4" />
            </div>
          </div>
          <p className="mt-2 text-xl font-bold tracking-tight text-foreground tabular-nums">
            {summary.failed_count.toLocaleString("vi-VN")} failed
          </p>
          <p className="mt-1.5 text-xs text-muted-foreground">
            + {summary.cancelled_count.toLocaleString("vi-VN")} cancelled
          </p>
        </CardContent>
      </Card>

      {/* Total Transactions */}
      <Card className="border-border/80 shadow-xs">
        <CardContent className="p-5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
              Total Transactions
            </span>
            <div className="rounded-lg bg-slate-500/10 p-2 text-slate-600 dark:text-slate-400">
              <Layers className="size-4" />
            </div>
          </div>
          <p className="mt-2 text-xl font-bold tracking-tight text-foreground tabular-nums">
            {summary.total_transactions.toLocaleString("vi-VN")}
          </p>
          <p className="mt-1.5 text-xs text-muted-foreground">
            {summary.success_count} + {summary.pending_count} +{" "}
            {summary.failed_count} + {summary.cancelled_count} ={" "}
            {summary.total_transactions}
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

// Keep alias for compatibility with existing imports
export const RevenueStatCards = BillingStatCards;
