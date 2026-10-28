"use client";

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
import type { TimeWindow } from "../lib/time-window";
import type {
  TransactionStatusFilter,
  TransactionTypeFilter,
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
  timeWindowError?: string;
  onReset: () => void;
  onExportCSV: () => void;
  isExporting?: boolean;
};

const TABS: { id: TransactionTypeFilter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "topup", label: "PayOS top-ups" },
  { id: "spend", label: "Coin spending" },
  { id: "refund", label: "Refunds" },
];

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
  return (
    <div className="space-y-4 rounded-xl border border-border/80 bg-card p-4 shadow-xs">
      {/* Top Row: Tabs & Export */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/60 pb-3">
        <div
          className="inline-flex flex-wrap items-center gap-1 rounded-lg bg-muted/60 p-1"
          role="tablist"
          aria-label="Transaction categories"
        >
          {TABS.map((t) => (
            <button
              key={t.id}
              type="button"
              role="tab"
              aria-selected={tab === t.id}
              onClick={() => onTabChange(t.id)}
              className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                tab === t.id
                  ? "bg-background text-foreground shadow-xs"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onReset}
            className="h-8 gap-1.5 text-xs"
          >
            <RotateCcw className="size-3.5" />
            Reset Filters
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
            {isExporting ? "Exporting..." : "Export CSV"}
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
            placeholder="Search name, email, order or ID"
            aria-label="Search transactions"
            className="h-9 pl-9 text-xs"
          />
        </div>

        <Select
          value={status}
          onValueChange={(val) =>
            onStatusChange((val ?? "all") as TransactionStatusFilter)
          }
        >
          <SelectTrigger
            className="h-9 w-full text-xs sm:w-[150px]"
            aria-label="Filter by status"
          >
            <SelectValue placeholder="All Statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Statuses</SelectItem>
            <SelectItem value="success">Success</SelectItem>
            <SelectItem value="pending">Pending</SelectItem>
            <SelectItem value="failed">Failed</SelectItem>
            <SelectItem value="cancelled">Cancelled</SelectItem>
          </SelectContent>
        </Select>

        <Select
          value={packageFilter}
          onValueChange={(val) => onPackageFilterChange(val ?? "all")}
        >
          <SelectTrigger
            className="h-9 w-full text-xs sm:w-[190px]"
            aria-label="Filter by package or feature"
          >
            <SelectValue placeholder="All packs and features" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All packs and features</SelectItem>
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
            placeholder="Min amount"
            aria-label="Minimum amount"
            className="h-9 w-full text-xs sm:w-[7.5rem]"
          />
          <span className="text-xs text-muted-foreground">–</span>
          <Input
            type="number"
            min={0}
            value={amountMax}
            onChange={(e) => onAmountMaxChange(e.target.value)}
            placeholder="Max amount"
            aria-label="Maximum amount"
            className="h-9 w-full text-xs sm:w-[7.5rem]"
          />
        </div>
      </div>
    </div>
  );
}
