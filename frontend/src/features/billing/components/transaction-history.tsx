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
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { describeTransaction } from "../lib/describe-transaction";
import type { Transaction, TransactionPage } from "../schemas/wallet-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { apiErrorMessage } from "@/lib/i18n/api-errors";
import { useTranslations } from "next-intl";

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
  const fmt = useFormatters();
  const errorsT = useTranslations("errors");
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
    if (!dateStr || dateStr.startsWith("0001-01-01")) return "—";
    return fmt.date(dateStr, "transaction") || "—";
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
            className="gap-1 border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400 text-[11px] font-medium"
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
      <CardContent className="p-4 sm:p-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 className="text-lg font-semibold tracking-tight text-foreground">
              Your transactions
            </h3>
            <p className="text-xs text-muted-foreground">
              Purchases, spending, and refunds in one place.
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

        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as FilterTab)}
          className="mt-4"
        >
          <p
            id="transaction-filter-scope"
            className="text-xs text-muted-foreground"
          >
            Filters apply to the transactions on this page.
          </p>
          <TabsList
            aria-label="Filter transactions on this page"
            aria-describedby="transaction-filter-scope"
            className="grid w-full grid-cols-2 gap-1 group-data-horizontal/tabs:h-auto sm:grid-cols-4"
          >
            {(
              [
                { id: "all", label: "All Transactions" },
                { id: "topup", label: "Top-ups (PayOS)" },
                { id: "spend", label: "Coin Spending" },
                { id: "refund", label: "Refunds" },
              ] as const
            ).map((tab) => (
              <TabsTrigger
                key={tab.id}
                value={tab.id}
                className="text-xs min-h-9 whitespace-normal"
              >
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>

          {/* List Content */}
          <TabsContent key={activeTab} value={activeTab} className="mt-4">
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
                  {apiErrorMessage(
                    errorsT,
                    error,
                    "Please try refreshing the page.",
                  )}
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
                    ? total === 0
                      ? "You haven't made any coin transactions yet. Top up coins above to get started."
                      : "No transactions on this page. Try another page or refresh."
                    : `No ${activeTab} transactions on this page. Other pages may contain matching transactions.`}
                </p>
              </div>
            ) : (
              <Table aria-label="Coin transactions" className="table-fixed">
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-[50%]">Transaction</TableHead>
                    <TableHead className="w-[25%]">Status</TableHead>
                    <TableHead className="w-[25%] text-right">Coins</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredItems.map((tx) => {
                    const refund = isRefund(tx);
                    const deposit = isDeposit(tx) && !refund;
                    const positive = deposit || refund;
                    const described = describeTransaction(tx.description);
                    // A PayOS top-up adds coins only once it is paid.
                    const unpaid =
                      tx.payos_order_code != null && tx.status !== "success";
                    const coins = `${positive ? "+" : "-"}${fmt.coinAmount(Math.abs(tx.amount))}`;
                    const Icon = refund
                      ? RotateCcw
                      : positive
                        ? ArrowDownLeft
                        : ArrowUpRight;
                    return (
                      <TableRow key={tx.id}>
                        <TableCell className="whitespace-normal py-4">
                          <div className="flex items-start gap-3">
                            <span className="hidden size-9 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground sm:flex">
                              <Icon className="size-4" aria-hidden />
                            </span>
                            <div className="min-w-0 space-y-1">
                              <p className="break-words text-sm font-medium">
                                {tx.description
                                  ? described.title
                                  : deposit
                                    ? "Coin Deposit"
                                    : refund
                                      ? "Coin Refund"
                                      : "Coin Spending"}
                              </p>
                              <p className="text-xs text-muted-foreground">
                                <time dateTime={tx.created_at}>
                                  {formatDate(tx.created_at)}
                                </time>
                                {described.detail && (
                                  <span> · {described.detail}</span>
                                )}
                              </p>
                              {tx.payos_order_code != null && (
                                <p className="text-xs break-all text-muted-foreground">
                                  Order #{tx.payos_order_code}
                                </p>
                              )}
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="px-1">
                          {renderStatusBadge(tx.status)}
                        </TableCell>
                        {unpaid && tx.status !== "pending" ? (
                          <TableCell
                            aria-label="No coins added"
                            className="text-right text-xs text-muted-foreground whitespace-normal"
                          >
                            No coins added
                          </TableCell>
                        ) : (
                          <TableCell
                            aria-label={`${coins} Coins${unpaid ? " when paid" : ""}`}
                            className="text-right font-semibold tabular-nums whitespace-normal"
                          >
                            <span
                              className={
                                unpaid
                                  ? "text-muted-foreground"
                                  : positive && tx.status === "success"
                                    ? "text-emerald-700 dark:text-emerald-400"
                                    : "text-foreground"
                              }
                            >
                              {coins}
                              <span className="sr-only"> Coins</span>
                            </span>
                            {unpaid && (
                              <span className="block text-xs font-normal text-muted-foreground">
                                when paid
                              </span>
                            )}
                          </TableCell>
                        )}
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            )}
          </TabsContent>
        </Tabs>
        {!isLoading && !error && (
          <p className="mt-3 text-xs text-muted-foreground">
            Showing {filteredItems.length} of {items.length} transactions on
            this page.
          </p>
        )}

        {/* Pagination Bar */}
        {total > pageSize && (
          <div className="mt-6 flex flex-wrap gap-3 items-center justify-between border-t border-border/60 pt-4 text-xs text-muted-foreground">
            <div>
              Showing page {page} of {totalPages} ({total} transactions across
              all types)
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
