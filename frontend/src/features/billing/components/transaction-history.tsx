"use client";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { TableSkeleton } from "@/components/ui/skeleton-presets";

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
  const t = useTranslations("candidate.billing.transactions");
  const describeT = useTranslations("candidate.billing.describe");
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
            {t("status.success")}
          </Badge>
        );
      case "pending":
        return (
          <Badge
            variant="outline"
            className="gap-1 border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400 text-[11px] font-medium"
          >
            <Clock className="h-3 w-3" />
            {t("status.pending")}
          </Badge>
        );
      case "cancelled":
        return (
          <Badge
            variant="outline"
            className="gap-1 border-muted bg-muted/50 text-muted-foreground text-[11px] font-medium"
          >
            <AlertCircle className="h-3 w-3" />
            {t("status.cancelled")}
          </Badge>
        );
      case "failed":
        return (
          <Badge
            variant="outline"
            className="gap-1 border-destructive/30 bg-destructive/10 text-destructive text-[11px] font-medium"
          >
            <XCircle className="h-3 w-3" />
            {t("status.failed")}
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
              {t("title")}
            </h3>
            <p className="text-xs text-muted-foreground">{t("subtitle")}</p>
          </div>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onRefresh}
              disabled={isLoading || isFetching}
              aria-label={t("refreshAria")}
              className="h-8 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
              />
              <span>{isFetching ? t("refreshing") : t("refresh")}</span>
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
            {t("filterScope")}
          </p>
          <TabsList
            aria-label={t("filterLabel")}
            aria-describedby="transaction-filter-scope"
            className="grid w-full grid-cols-2 gap-1 group-data-horizontal/tabs:h-auto sm:grid-cols-4"
          >
            {(["all", "topup", "spend", "refund"] as const).map((tab) => (
              <TabsTrigger
                key={tab}
                value={tab}
                className="text-xs min-h-9 whitespace-normal"
              >
                {t(`tabs.${tab}`)}
              </TabsTrigger>
            ))}
          </TabsList>

          {/* List Content */}
          <TabsContent key={activeTab} value={activeTab} className="mt-4">
            {isLoading ? (
              <TableSkeleton columns={3} label={t("tableLabel")} />
            ) : error ? (
              <ErrorState
                title={t("loadError")}
                description={apiErrorMessage(
                  errorsT,
                  error,
                  t("loadErrorFallback"),
                )}
                onRetry={onRefresh}
                isRetrying={isFetching}
                retryLabel={t("retry")}
              />
            ) : filteredItems.length === 0 ? (
              <EmptyState
                icons={[Receipt]}
                title={t("emptyTitle")}
                description={
                  activeTab === "all"
                    ? total === 0
                      ? t("emptyNone")
                      : t("emptyPage")
                    : t(`emptyFiltered.${activeTab}`)
                }
              />
            ) : (
              <Table aria-label={t("tableLabel")} className="table-fixed">
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-[50%]">
                      {t("columns.transaction")}
                    </TableHead>
                    <TableHead className="w-[25%]">
                      {t("columns.status")}
                    </TableHead>
                    <TableHead className="w-[25%] text-right">
                      {t("columns.coins")}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredItems.map((tx) => {
                    const refund = isRefund(tx);
                    const deposit = isDeposit(tx) && !refund;
                    const positive = deposit || refund;
                    const described = describeTransaction(
                      tx.description,
                      describeT,
                      fmt,
                    );
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
                                    ? t("fallbackDeposit")
                                    : refund
                                      ? t("fallbackRefund")
                                      : t("fallbackSpend")}
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
                                  {t("order", { code: tx.payos_order_code })}
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
                            aria-label={t("noCoins")}
                            className="text-right text-xs text-muted-foreground whitespace-normal"
                          >
                            {t("noCoins")}
                          </TableCell>
                        ) : (
                          <TableCell
                            aria-label={
                              unpaid
                                ? t("coinsAriaWhenPaid", { coins })
                                : t("coinsAria", { coins })
                            }
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
                              <span className="sr-only">{t("coinsSr")}</span>
                            </span>
                            {unpaid && (
                              <span className="block text-xs font-normal text-muted-foreground">
                                {t("whenPaid")}
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
            {t("showing", { shown: filteredItems.length, total: items.length })}
          </p>
        )}

        {/* Pagination Bar */}
        {total > pageSize && (
          <div className="mt-6 flex flex-wrap gap-3 items-center justify-between border-t border-border/60 pt-4 text-xs text-muted-foreground">
            <div>{t("pagination", { page, totalPages, total })}</div>

            <div className="flex items-center gap-1.5">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => onPageChange(page - 1)}
                disabled={page <= 1 || isLoading || isFetching}
                className="h-8 w-8 p-0"
                aria-label={t("previous")}
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
                aria-label={t("next")}
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
