"use client";

import { useState } from "react";
import {
  ArrowDownLeft,
  ArrowUpRight,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle,
  RotateCcw,
  Receipt,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { Transaction, TransactionPage } from "../schemas/wallet-schema";

type FilterTab = "all" | "topup" | "spend" | "refund";

interface TransactionHistoryProps {
  currentUserId: string;
  data?: TransactionPage;
  isLoading: boolean;
  isFetching: boolean;
  error: Error | null;
  page: number;
  onPageChange: (newPage: number) => void;
  onRefresh: () => void;
}

export function TransactionHistory({
  currentUserId,
  data,
  isLoading,
  isFetching,
  error,
  page,
  onPageChange,
  onRefresh,
}: TransactionHistoryProps) {
  const [activeTab, setActiveTab] = useState<FilterTab>("all");

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pageSize = data?.page_size ?? 20;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  const isDeposit = (tx: Transaction) => {
    return (
      tx.payos_order_code != null ||
      tx.to === currentUserId ||
      (tx.description?.toLowerCase().includes("deposit") ?? false)
    );
  };

  const isRefund = (tx: Transaction) => {
    return (
      (tx.description?.toLowerCase().includes("refund") ?? false) ||
      (tx.to === currentUserId && tx.from == null && !tx.payos_order_code)
    );
  };

  const isSpend = (tx: Transaction) => {
    return (
      tx.from === currentUserId ||
      (!isDeposit(tx) && !isRefund(tx)) ||
      (tx.description?.toLowerCase().includes("spend") ?? false)
    );
  };

  const filteredItems = items.filter((tx) => {
    if (activeTab === "all") return true;
    if (activeTab === "topup") return isDeposit(tx) && !isRefund(tx);
    if (activeTab === "refund") return isRefund(tx);
    if (activeTab === "spend") return isSpend(tx);
    return true;
  });

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "—";
    try {
      const d = new Date(dateStr);
      return new Intl.DateTimeFormat("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      }).format(d);
    } catch {
      return dateStr;
    }
  };

  const renderStatusBadge = (status: Transaction["status"]) => {
    switch (status) {
      case "success":
        return (
          <Badge
            variant="outline"
            className="gap-1 border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 text-[11px] font-medium"
          >
            <CheckCircle2 className="h-3 w-3" />
            Success
          </Badge>
        );
      case "pending":
        return (
          <Badge
            variant="outline"
            className="gap-1 border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400 text-[11px] font-medium animate-pulse"
          >
            <Clock className="h-3 w-3" />
            Pending
          </Badge>
        );
      case "cancelled":
        return (
          <Badge
            variant="outline"
            className="gap-1 border-muted bg-muted/50 text-muted-foreground text-[11px] font-medium"
          >
            <AlertCircle className="h-3 w-3" />
            Cancelled
          </Badge>
        );
      case "failed":
        return (
          <Badge
            variant="outline"
            className="gap-1 border-destructive/30 bg-destructive/10 text-destructive text-[11px] font-medium"
          >
            <XCircle className="h-3 w-3" />
            Failed
          </Badge>
        );
      default:
        return (
          <Badge variant="outline" className="text-[11px]">
            {status}
          </Badge>
        );
    }
  };

  return (
    <Card className="border-border/80 shadow-sm">
      <CardContent className="p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 className="text-lg font-semibold tracking-tight text-foreground">
              Unified Transaction Ledger
            </h3>
            <p className="text-xs text-muted-foreground">
              Authoritative transaction history from backend operations
              (deposits, spending, and refunds).
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onRefresh}
              disabled={isLoading || isFetching}
              aria-label="Refresh transaction history"
              className="h-8 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
              />
              <span>{isFetching ? "Refreshing…" : "Refresh"}</span>
            </Button>
          </div>
        </div>

        <p className="mt-4 text-xs text-muted-foreground">Filter this page only. Use pagination to browse other transactions.</p>
        {/* Sliding Tabs Filter */}
        <div className="mt-4 flex gap-1 rounded-lg bg-muted/60 p-1 text-xs">
          {(
            [
              { id: "all", label: "All Transactions" },
              { id: "topup", label: "Top-ups (PayOS)" },
              { id: "spend", label: "Coin Spending" },
              { id: "refund", label: "Refunds" },
            ] as const
          ).map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`flex-1 rounded-md py-1.5 font-medium transition-all duration-150 ${
                activeTab === tab.id
                  ? "bg-card text-foreground shadow-xs font-semibold"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* List Content */}
        <div className="mt-4">
          {isLoading ? (
            <div className="space-y-3">
              {[1, 2, 3, 4].map((i) => (
                <div
                  key={i}
                  className="flex items-center justify-between border-b border-border/50 py-3"
                >
                  <div className="flex items-center gap-3">
                    <Skeleton className="h-9 w-9 rounded-full" />
                    <div className="space-y-1.5">
                      <Skeleton className="h-4 w-48" />
                      <Skeleton className="h-3 w-28" />
                    </div>
                  </div>
                  <div className="flex items-center gap-4">
                    <Skeleton className="h-6 w-16" />
                    <Skeleton className="h-4 w-20" />
                  </div>
                </div>
              ))}
            </div>
          ) : error ? (
            <div className="flex flex-col items-center justify-center p-8 text-center">
              <p className="text-sm font-medium text-destructive">
                Unable to load transaction history
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                {error.message || "Please try refreshing the page."}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onRefresh}
                disabled={isFetching}
                className="mt-4 gap-1.5 text-xs"
              >
                <RefreshCw
                  className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
                />
                Retry
              </Button>
            </div>
          ) : filteredItems.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground">
              <Receipt className="h-10 w-10 stroke-1 opacity-40" />
              <p className="mt-2 text-sm font-medium text-foreground">
                No transactions found
              </p>
              <p className="max-w-xs text-xs text-muted-foreground">
                {activeTab === "all"
                  ? (total === 0 ? "You haven't made any coin transactions yet. Top up coins above to get started." : "No transactions on this page. Try another page or refresh.")
                  : `No ${activeTab} transactions on this page. Other pages may contain matching transactions.`}
              </p>
            </div>
          ) : (
            <div className="divide-y divide-border/60">
              {filteredItems.map((tx) => {
                const deposit = isDeposit(tx) && !isRefund(tx);
                const refund = isRefund(tx);
                const positive = deposit || refund;

                return (
                  <div
                    key={tx.id}
                    className="flex flex-col gap-2 py-3.5 sm:flex-row sm:items-center sm:justify-between transition-colors hover:bg-muted/20 px-2 rounded-md"
                  >
                    <div className="flex items-start gap-3">
                      <div
                        className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full ${
                          positive
                            ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                            : "bg-muted text-muted-foreground"
                        }`}
                      >
                        {refund ? (
                          <RotateCcw className="h-4 w-4" />
                        ) : positive ? (
                          <ArrowDownLeft className="h-4 w-4" />
                        ) : (
                          <ArrowUpRight className="h-4 w-4" />
                        )}
                      </div>

                      <div className="space-y-0.5">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-semibold text-foreground">
                            {tx.description ||
                              (deposit
                                ? "Coin Deposit"
                                : refund
                                  ? "Coin Refund"
                                  : "Coin Spending")}
                          </span>
                          {tx.payos_order_code && (
                            <Badge
                              variant="secondary"
                              className="text-[10px] font-mono px-1.5 py-0"
                            >
                              Order #{tx.payos_order_code}
                            </Badge>
                          )}
                        </div>
                        <p className="text-xs text-muted-foreground">
                          {formatDate(tx.created_at)}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center justify-between sm:justify-end gap-3 pl-12 sm:pl-0">
                      {renderStatusBadge(tx.status)}
                      <span
                        className={`text-sm font-bold tracking-tight ${
                          positive
                            ? "text-emerald-600 dark:text-emerald-400"
                            : "text-foreground"
                        }`}
                      >
                        {positive ? "+" : "-"}
                        {Math.abs(tx.amount).toLocaleString("en-US")} Coins
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {!isLoading && !error && <p className="mt-3 text-xs text-muted-foreground">Showing {filteredItems.length} of {items.length} transactions on this page.</p>}
        {/* Pagination Bar */}
        {total > pageSize && (
          <div className="mt-6 flex items-center justify-between border-t border-border/60 pt-4 text-xs text-muted-foreground">
            <div>
              Showing page {page} of {totalPages} ({total} transactions across all types)
            </div>

            <div className="flex items-center gap-1.5">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => onPageChange(page - 1)}
                disabled={page <= 1 || isLoading || isFetching}
                className="h-8 w-8 p-0"
                aria-label="Previous page"
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="px-2 font-medium text-foreground">{page}</span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => onPageChange(page + 1)}
                disabled={page >= totalPages || isLoading || isFetching}
                className="h-8 w-8 p-0"
                aria-label="Next page"
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
