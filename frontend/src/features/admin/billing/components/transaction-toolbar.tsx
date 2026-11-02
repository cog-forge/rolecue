"use client";

import { useTranslations } from "next-intl";
import { Download, RotateCcw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { TimeWindow, TimeWindowError } from "../lib/time-window";
import {
  transactionStatusFilterSchema,
  type TransactionStatusFilter,
  type TransactionTypeFilter,
} from "../schemas/billing-schema";
import { TimeWindowFilter } from "./time-window-filter";

type TransactionToolbarProps = {
  tab: TransactionTypeFilter;
  onTabChange: (tab: TransactionTypeFilter) => void;
  search: string;
  onSearchChange: (val: string) => void;
  status: TransactionStatusFilter;
  onStatusChange: (val: TransactionStatusFilter) => void;
  packageFilter: string;
  onPackageFilterChange: (val: string) => void;
  packageOptions?: { value: string; label: string }[];
  amountMin: string;
  onAmountMinChange: (val: string) => void;
  amountMax: string;
  onAmountMaxChange: (val: string) => void;
  timeWindow: TimeWindow;
  onTimeWindowChange: (window: TimeWindow) => void;
  timeWindowError?: TimeWindowError;
  onReset: () => void;
  onExportCSV: () => void;
  isExporting?: boolean;
};

const TABS: TransactionTypeFilter[] = ["all", "topup", "spend", "refund"];
const STATUSES = ["success", "pending", "failed", "cancelled"] as const;

export function TransactionToolbar({
  tab,
  onTabChange,
  search,
  onSearchChange,
  status,
  onStatusChange,
  packageFilter,
  onPackageFilterChange,
  packageOptions = [],
  amountMin,
  onAmountMinChange,
  amountMax,
  onAmountMaxChange,
  timeWindow,
  onTimeWindowChange,
  timeWindowError,
  onReset,
  onExportCSV,
  isExporting = false,
}: TransactionToolbarProps) {
  const t = useTranslations("admin.billing.transactions.toolbar");
  const tCommon = useTranslations("common");
  return (
    <div className="space-y-4 rounded-xl border border-border/80 bg-card p-4 shadow-xs">
      {/* Top Row: Tabs & Export */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/60 pb-3">
        <div
          className="inline-flex flex-wrap items-center gap-1 rounded-lg bg-muted/60 p-1"
          role="tablist"
          aria-label={t("categories")}
        >
          {TABS.map((id) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              onClick={() => onTabChange(id)}
              className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                tab === id
                  ? "bg-background text-foreground shadow-xs"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {id === "all" ? tCommon("labels.all") : t(`tabs.${id}`)}
            </button>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onReset}
            className="h-8 gap-1.5 text-xs"
          >
            <RotateCcw className="size-3.5" />
            {t("reset")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onExportCSV}
            disabled={isExporting}
            className="h-8 gap-1.5 text-xs"
          >
            <Download className="size-3.5" />
            {isExporting ? t("exporting") : t("export")}
          </Button>
        </div>
      </div>

      {/* Filters: one wrapping row, scope first */}
      <div className="flex flex-wrap items-start gap-2">
        <TimeWindowFilter
          value={timeWindow}
          onChange={onTimeWindowChange}
          error={timeWindowError}
        />
        <div className="relative min-w-[16rem] flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={t("searchPlaceholder")}
            aria-label={t("searchLabel")}
            className="h-9 pl-9 text-xs"
          />
        </div>

        <Select
          value={status}
          onValueChange={(val) =>
            onStatusChange(
              transactionStatusFilterSchema.catch("all").parse(val),
            )
          }
        >
          <SelectTrigger
            className="h-9 w-full text-xs sm:w-[150px]"
            aria-label={t("statusLabel")}
          >
            <SelectValue placeholder={t("statuses.all")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("statuses.all")}</SelectItem>
            {STATUSES.map((value) => (
              <SelectItem key={value} value={value}>
                {t(`statuses.${value}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={packageFilter}
          onValueChange={(val) => onPackageFilterChange(val ?? "all")}
        >
          <SelectTrigger
            className="h-9 w-full text-xs sm:w-[190px]"
            aria-label={t("packageLabel")}
          >
            <SelectValue placeholder={t("allPackages")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("allPackages")}</SelectItem>
            {packageOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="flex w-full items-center gap-1.5 sm:w-auto">
          <Input
            type="number"
            min={0}
            value={amountMin}
            onChange={(e) => onAmountMinChange(e.target.value)}
            placeholder={t("minAmountPlaceholder")}
            aria-label={t("minAmountLabel")}
            className="h-9 w-full text-xs sm:w-[9rem]"
          />
          <span className="text-xs text-muted-foreground">–</span>
          <Input
            type="number"
            min={0}
            value={amountMax}
            onChange={(e) => onAmountMaxChange(e.target.value)}
            placeholder={t("maxAmountPlaceholder")}
            aria-label={t("maxAmountLabel")}
            className="h-9 w-full text-xs sm:w-[9rem]"
          />
        </div>
      </div>
    </div>
  );
}
