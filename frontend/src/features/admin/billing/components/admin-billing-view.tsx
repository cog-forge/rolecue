"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { gooeyToast } from "goey-toast";
import {
  adminBillingKeys,
  exportBillingCSV,
  getBillingSummary,
  listAdminTransactions,
} from "../api/billing";
import { timeWindowFileSuffix } from "../lib/time-window";
import { BillingSummaryStrip } from "./billing-summary-strip";
import { useTimeWindow } from "./time-window-filter";
import { TransactionToolbar } from "./transaction-toolbar";
import { TransactionTable } from "./transaction-table";
import type {
  AdminTransactionQuery,
  TransactionStatusFilter,
  TransactionTypeFilter,
} from "../schemas/billing-schema";

export function AdminBillingView() {
  const t = useTranslations("admin.billing.transactions");
  const tCommon = useTranslations("common");
  const [tab, setTab] = useState<TransactionTypeFilter>("all");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<TransactionStatusFilter>("all");
  const [packageFilter, setPackageFilter] = useState("all");
  const [amountMin, setAmountMin] = useState("");
  const [amountMax, setAmountMax] = useState("");
  const timeWindow = useTimeWindow();
  const { dateFrom, dateTo } = timeWindow.applied;

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortBy, setSortBy] = useState("created_at");
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
      dateFrom,
      dateTo,
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
      dateFrom,
      dateTo,
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

  // Packs come from the data; features are matched on the wording the ledger writes for them.
  const packageOptions = useMemo(
    () => [
      ...(summaryQuery.data?.package_breakdown ?? [])
        .filter((pkg) => pkg.package_name)
        .map((pkg) => ({ value: pkg.package_name, label: pkg.package_name })),
      {
        value: "Practice interview",
        label: t("packageOptions.practiceInterview"),
      },
      {
        value: "Interview capacity",
        label: t("packageOptions.interviewSlots"),
      },
      { value: "Avatar inventory", label: t("packageOptions.avatarStorage") },
    ],
    [summaryQuery.data?.package_breakdown, t],
  );

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
    timeWindow.reset();
    setSortBy("created_at");
    setSortOrder("desc");
    setPage(1);
  };

  const handleExportCSV = async () => {
    try {
      setIsExporting(true);
      const { csv, totalCount, truncated } =
        await exportBillingCSV(queryFilter);
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `rolecue-transactions${timeWindowFileSuffix(timeWindow.applied.window, new Date())}.csv`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);

      if (truncated) {
        gooeyToast.warning(t("exportToast.truncatedTitle", { limit: 5000 }), {
          description: t("exportToast.truncatedBody", {
            limit: 5000,
            total: totalCount,
          }),
        });
      }
    } catch {
      gooeyToast.error(t("exportToast.failedTitle"), {
        description: t("exportToast.failedBody"),
      });
    } finally {
      setIsExporting(false);
    }
  };

  const hasError = summaryQuery.isError || transactionsQuery.isError;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">
          {t("title")}
        </h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {/* Summary Cards */}
      <BillingSummaryStrip
        summary={summaryQuery.data}
        isLoading={summaryQuery.isLoading}
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
        timeWindow={timeWindow.draft}
        onTimeWindowChange={(next) => {
          timeWindow.change(next);
          setPage(1);
        }}
        timeWindowError={timeWindow.error}
        onReset={handleReset}
        onExportCSV={() => {
          void handleExportCSV();
        }}
        isExporting={isExporting}
      />

      {/* Error State */}
      {hasError ? (
        <ErrorState
          title={t("loadError.title")}
          description={t("loadError.hint")}
          onRetry={() => {
            void summaryQuery.refetch();
            void transactionsQuery.refetch();
          }}
          retryLabel={tCommon("actions.retry")}
        />
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
