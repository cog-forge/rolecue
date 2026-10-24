"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import {
  CreditCard,
  RefreshCw,
  ChartNoAxesCombined,
  Receipt,
  AlertCircle,
  Coins,
  ArrowDownLeft,
  RotateCcw,
  ListFilter,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { routes } from "@/config/routes";
import {
  adminBillingKeys,
  getBillingSummary,
  listAdminTransactions,
  getAdminRevenueReport,
} from "../api/billing";
import type {
  AdminTransactionQuery,
  AdminTransactionItem,
  DatePreset,
} from "../schemas/billing-schema";
import { RevenueStatCards } from "./revenue-stat-cards";
import { TransactionToolbar } from "./transaction-toolbar";
import { TransactionTable } from "./transaction-table";

export function AdminBillingView() {
  const [search, setSearch] = useState("");
  const [activeTab, setActiveTab] = useState<
    "all" | "topup" | "spend" | "refund"
  >("all");
  const [datePreset, setDatePreset] = useState<DatePreset>("custom");
  const [selectedTx, setSelectedTx] = useState<AdminTransactionItem | null>(
    null,
  );

  const [query, setQuery] = useState<AdminTransactionQuery>({
    tab: "all",
    page: 1,
    pageSize: 20,
    search: "",
    status: "all",
    type: "all",
    package: "all",
    dateFrom: "",
    dateTo: "",
    includeTest: false,
    sortBy: "created_at",
    sortOrder: "desc",
  });

  // Authoritative transactions query
  const transactionsQuery = useQuery({
    queryKey: adminBillingKeys.transactions(query),
    queryFn: ({ signal }) => listAdminTransactions(query, signal),
    staleTime: 15_000,
  });

  // Unified Billing Summary query (respects all active filters)
  const summaryQuery = useQuery({
    queryKey: adminBillingKeys.summary(query as Record<string, unknown>),
    queryFn: ({ signal }) => getBillingSummary(query, signal),
    staleTime: 15_000,
  });

  // Fallback / legacy revenue query (for backward compatibility in unit test environments)
  const revenueLegacyQuery = useQuery({
    queryKey: adminBillingKeys.revenue(
      query.dateFrom || "",
      query.dateTo || "",
    ),
    queryFn: ({ signal }) =>
      getAdminRevenueReport(query.dateFrom || "", query.dateTo || "", signal),
    staleTime: 30_000,
  });

  const isRefreshing = transactionsQuery.isFetching || summaryQuery.isFetching;

  const handleRefresh = async () => {
    await Promise.all([
      transactionsQuery.refetch(),
      summaryQuery.refetch(),
      revenueLegacyQuery.refetch(),
    ]);
  };

  const handleTabChange = (val: string) => {
    const tabVal = val as "all" | "topup" | "spend" | "refund";
    setActiveTab(tabVal);
    setQuery((prev) => ({
      ...prev,
      page: 1,
      tab: tabVal,
      type: tabVal === "all" ? "all" : tabVal,
    }));
  };

  const handleSearchChange = (value: string) => {
    setSearch(value);
    setQuery((prev) => ({
      ...prev,
      page: 1,
      search: value.trim(),
    }));
  };

  const total = transactionsQuery.data?.total ?? 0;
  const items = transactionsQuery.data?.items ?? [];
  const pageSize = query.pageSize || 20;
  const offset = (query.page - 1) * pageSize;

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
              <CreditCard className="h-5 w-5" aria-hidden />
            </span>
            <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              Payment Transactions
            </h1>
          </div>
          <p className="max-w-2xl text-xs sm:text-sm text-muted-foreground">
            Search, filter, inspect, and reconcile PayOS bank top-ups, coin
            spending ledger, and refunds.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            asChild
            variant="outline"
            size="sm"
            className="h-9 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
          >
            <Link href={routes.admin.revenue}>
              <ChartNoAxesCombined className="h-3.5 w-3.5" aria-hidden />
              <span>Revenue Reports</span>
            </Link>
          </Button>

          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void handleRefresh()}
            disabled={isRefreshing}
            className="h-9 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            aria-label="Refresh transactions and metrics"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isRefreshing ? "animate-spin" : ""}`}
              aria-hidden
            />
            <span>{isRefreshing ? "Refreshing…" : "Refresh"}</span>
          </Button>
        </div>
      </header>

      {/* Metric Cards (Period & Filter Aware) */}
      <section
        aria-labelledby="billing-stat-cards-heading"
        className="space-y-2"
      >
        <h2 id="billing-stat-cards-heading" className="sr-only">
          Transaction Metrics
        </h2>
        <RevenueStatCards
          summary={summaryQuery.data}
          stats={revenueLegacyQuery.data?.stats}
          isLoading={summaryQuery.isLoading && revenueLegacyQuery.isLoading}
          periodLabel={
            query.dateFrom || query.dateTo ? "Active period" : "All time"
          }
        />
      </section>

      {/* Tabs: All | Payments (top-ups) | Coin ledger (spending) | Refunds */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-border/60 pb-1">
        <Tabs
          value={activeTab}
          onValueChange={handleTabChange}
          className="w-full sm:w-auto"
        >
          <TabsList className="h-9 p-1 bg-muted/60">
            <TabsTrigger value="all" className="text-xs px-3 gap-1.5">
              <ListFilter className="h-3.5 w-3.5" />
              All
            </TabsTrigger>
            <TabsTrigger value="topup" className="text-xs px-3 gap-1.5">
              <ArrowDownLeft className="h-3.5 w-3.5" />
              Payments (PayOS Top-ups)
            </TabsTrigger>
            <TabsTrigger value="spend" className="text-xs px-3 gap-1.5">
              <Coins className="h-3.5 w-3.5" />
              Coin Ledger (Spending)
            </TabsTrigger>
            <TabsTrigger value="refund" className="text-xs px-3 gap-1.5">
              <RotateCcw className="h-3.5 w-3.5" />
              Refunds
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      {/* Filter Toolbar */}
      <section aria-labelledby="transaction-filters-heading">
        <h2 id="transaction-filters-heading" className="sr-only">
          Transaction Filters
        </h2>
        <TransactionToolbar
          query={query}
          search={search}
          onSearchChange={handleSearchChange}
          onChange={setQuery}
          preset={datePreset}
          onPresetChange={setDatePreset}
          disabled={transactionsQuery.isFetching}
        />
      </section>

      {/* Status Bar */}
      <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground px-1">
        <p role="status" aria-live="polite">
          {transactionsQuery.isLoading || transactionsQuery.isFetching
            ? "Loading transactions…"
            : total > 0
              ? `Showing ${offset + 1}–${Math.min(offset + pageSize, total)} of ${total.toLocaleString("en-US")} transactions`
              : "0 transactions found"}
        </p>

        {query.includeTest && (
          <span className="text-[11px] font-medium text-amber-600 dark:text-amber-400 bg-amber-500/10 px-2 py-0.5 rounded-sm border border-amber-500/20">
            Showing test & seed data
          </span>
        )}
      </div>

      {/* Ledger Table Section */}
      <section aria-labelledby="transactions-table-heading">
        <h2 id="transactions-table-heading" className="sr-only">
          Transactions Ledger
        </h2>

        <div className="rounded-xl border border-border/70 bg-card overflow-hidden shadow-xs">
          {transactionsQuery.isLoading ? (
            <div
              role="status"
              aria-label="Loading transactions table"
              className="space-y-3 p-5"
            >
              <Skeleton className="h-10 w-full" />
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full" />
              ))}
            </div>
          ) : transactionsQuery.error ? (
            <div
              role="alert"
              className="flex flex-col items-center justify-center p-12 text-center"
            >
              <AlertCircle className="h-9 w-9 text-destructive" aria-hidden />
              <h3 className="mt-3 text-sm font-semibold text-foreground">
                Unable to load transactions
              </h3>
              <p className="mt-1 max-w-sm text-xs text-muted-foreground">
                {transactionsQuery.error instanceof Error
                  ? transactionsQuery.error.message
                  : "An error occurred while fetching the transaction ledger. Please check your connection and retry."}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void transactionsQuery.refetch()}
                className="mt-4 gap-1.5 text-xs"
              >
                <RefreshCw className="h-3.5 w-3.5" aria-hidden />
                Retry
              </Button>
            </div>
          ) : items.length === 0 ? (
            <div
              role="status"
              className="flex flex-col items-center justify-center py-16 text-center"
            >
              <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
                <Receipt className="h-6 w-6 stroke-1" aria-hidden />
              </span>
              <h3 className="mt-4 text-base font-semibold text-foreground">
                No transactions recorded
              </h3>
              <p className="mt-1 max-w-sm text-xs text-muted-foreground">
                No transactions found matching your criteria. Try adjusting your
                search query, status filter, or date range.
              </p>
            </div>
          ) : (
            <div>
              <TransactionTable
                transactions={items}
                isLoading={false}
                selectedTx={selectedTx}
                onSelectTx={(tx) => setSelectedTx(tx)}
                onDrawerClose={() => setSelectedTx(null)}
                onTxUpdated={() => {
                  void handleRefresh();
                }}
              />

              <div className="border-t border-border/60 p-4">
                <DataTablePagination
                  offset={offset}
                  pageSize={pageSize}
                  total={total}
                  disabled={transactionsQuery.isFetching}
                  label="Transaction pagination"
                  onPageChange={(page) =>
                    setQuery((prev) => ({ ...prev, page }))
                  }
                />
              </div>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
