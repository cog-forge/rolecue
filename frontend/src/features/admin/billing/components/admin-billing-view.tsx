"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  adminBillingKeys,
  exportBillingCSV,
  getBillingSummary,
  listAdminTransactions,
} from "../api/billing";
import { BillingStatCards } from "./revenue-stat-cards";
import { TransactionToolbar } from "./transaction-toolbar";
import { TransactionTable } from "./transaction-table";
import type {
  AdminTransactionQuery,
  TransactionStatusFilter,
  TransactionTypeFilter,
} from "../schemas/billing-schema";

export function AdminBillingView() {
  const [tab, setTab] = useState<TransactionTypeFilter>("all");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<TransactionStatusFilter>("all");
  const [packageFilter, setPackageFilter] = useState("all");
  const [amountMin, setAmountMin] = useState("");
  const [amountMax, setAmountMax] = useState("");

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortBy, setSortBy] = useState("id");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");
  const [isExporting, setIsExporting] = useState(false);

  const parsedMin =
    amountMin.trim() !== "" && !Number.isNaN(Number(amountMin))
      ? Number(amountMin)
      : undefined;
  const parsedMax =
    amountMax.trim() !== "" && !Number.isNaN(Number(amountMax))
      ? Number(amountMax)
      : undefined;

  const queryFilter: AdminTransactionQuery = useMemo(
    () => ({
      tab,
      page,
      pageSize,
      search: search.trim() || undefined,
      status: status !== "all" ? status : undefined,
      type: tab !== "all" ? tab : undefined,
      package: packageFilter !== "all" ? packageFilter : undefined,
      amountMin: parsedMin,
      amountMax: parsedMax,
      sortBy,
      sortOrder,
    }),
    [
      tab,
      page,
      pageSize,
      search,
      status,
      packageFilter,
      parsedMin,
      parsedMax,
      sortBy,
      sortOrder,
    ],
  );

  // Summary and Transactions share the exact same filter model so cards and table always agree.
  const summaryQuery = useQuery({
    queryKey: adminBillingKeys.summary(queryFilter),
    queryFn: ({ signal }) => getBillingSummary(queryFilter, signal),
  });

  const transactionsQuery = useQuery({
    queryKey: adminBillingKeys.transactions(queryFilter),
    queryFn: ({ signal }) => listAdminTransactions(queryFilter, signal),
  });

  const packageOptions = useMemo(() => {
    const set = new Set<string>();
    for (const pkg of summaryQuery.data?.package_breakdown ?? []) {
      if (pkg.package_name) set.add(pkg.package_name);
    }
    for (const feat of summaryQuery.data?.feature_breakdown ?? []) {
      if (feat.feature_name) set.add(feat.feature_name);
    }
    return Array.from(set);
  }, [
    summaryQuery.data?.package_breakdown,
    summaryQuery.data?.feature_breakdown,
  ]);

  const handleSortChange = (col: string) => {
    if (sortBy === col) {
      setSortOrder((prev) => (prev === "asc" ? "desc" : "asc"));
    } else {
      setSortBy(col);
      setSortOrder("desc");
    }
    setPage(1);
  };

  const handleReset = () => {
    setTab("all");
    setSearch("");
    setStatus("all");
    setPackageFilter("all");
    setAmountMin("");
    setAmountMax("");
    setSortBy("id");
    setSortOrder("desc");
    setPage(1);
  };

  const handleExportCSV = async () => {
    try {
      setIsExporting(true);
      const csvText = await exportBillingCSV(queryFilter);
      const blob = new Blob([csvText], { type: "text/csv;charset=utf-8;" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "rolecue-transactions.csv";
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } finally {
      setIsExporting(false);
    }
  };

  const hasError = summaryQuery.isError || transactionsQuery.isError;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-bold tracking-tight text-foreground">
          Payment Transactions
        </h1>
        <p className="text-sm text-muted-foreground">
          Read-only ledger of PayOS bank top-ups, internal coin spending, and
          slot refunds. All monetary figures are in VND (₫); coin balances are
          tracked separately.
        </p>
      </div>

      {/* Summary Cards */}
      <BillingStatCards
        summary={summaryQuery.data}
        isLoading={summaryQuery.isLoading}
        periodLabel="Matching filters"
        variant="transactions"
      />

      {/* Filter & Export Toolbar */}
      <TransactionToolbar
        tab={tab}
        onTabChange={(nextTab) => {
          setTab(nextTab);
          setPage(1);
        }}
        search={search}
        onSearchChange={(val) => {
          setSearch(val);
          setPage(1);
        }}
        status={status}
        onStatusChange={(val) => {
          setStatus(val);
          setPage(1);
        }}
        packageFilter={packageFilter}
        onPackageFilterChange={(val) => {
          setPackageFilter(val);
          setPage(1);
        }}
        packageOptions={packageOptions}
        amountMin={amountMin}
        onAmountMinChange={(val) => {
          setAmountMin(val);
          setPage(1);
        }}
        amountMax={amountMax}
        onAmountMaxChange={(val) => {
          setAmountMax(val);
          setPage(1);
        }}
        onReset={handleReset}
        onExportCSV={() => {
          void handleExportCSV();
        }}
        isExporting={isExporting}
      />

      {/* Error State */}
      {hasError ? (
        <div
          role="alert"
          className="flex flex-col items-center justify-center gap-3 rounded-xl border border-destructive/40 bg-destructive/5 p-8 text-center"
        >
          <AlertCircle className="size-8 text-destructive" />
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-foreground">
              Unable to load transactions
            </h2>
            <p className="text-xs text-muted-foreground">
              Please verify your filter parameters (such as amount bounds) and
              try again.
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              void summaryQuery.refetch();
              void transactionsQuery.refetch();
            }}
          >
            Retry
          </Button>
        </div>
      ) : (
        <TransactionTable
          items={transactionsQuery.data?.items ?? []}
          total={transactionsQuery.data?.total ?? 0}
          page={page}
          pageSize={pageSize}
          isLoading={transactionsQuery.isLoading}
          sortBy={sortBy}
          sortOrder={sortOrder}
          onSortChange={handleSortChange}
          onPageChange={setPage}
          onPageSizeChange={(nextSize) => {
            setPageSize(nextSize);
            setPage(1);
          }}
        />
      )}
    </div>
  );
}
