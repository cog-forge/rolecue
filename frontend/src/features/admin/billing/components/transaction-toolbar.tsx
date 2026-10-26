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
import { DatePresetPicker } from "./date-preset-picker";
import type {
  DatePreset,
  TransactionStatusFilter,
  TransactionTypeFilter,
} from "../schemas/billing-schema";

type TransactionToolbarProps = {
  tab: TransactionTypeFilter;
  onTabChange: (tab: TransactionTypeFilter) => void;
  search: string;
  onSearchChange: (val: string) => void;
  status: TransactionStatusFilter;
  onStatusChange: (val: TransactionStatusFilter) => void;
  packageFilter: string;
  onPackageFilterChange: (val: string) => void;
  packageOptions?: string[];
  amountMin: string;
  onAmountMinChange: (val: string) => void;
  amountMax: string;
  onAmountMaxChange: (val: string) => void;
  datePreset: DatePreset;
  dateFrom: string;
  dateTo: string;
  onPresetSelect: (preset: DatePreset, from: string, to: string) => void;
  onCustomDateChange: (from: string, to: string) => void;
  onReset: () => void;
  onExportCSV: () => void;
  isExporting?: boolean;
};

const TABS: { id: TransactionTypeFilter; label: string }[] = [
  { id: "all", label: "All Transactions" },
  { id: "topup", label: "Payments (PayOS Top-ups)" },
  { id: "spend", label: "Coin Ledger (Spending)" },
  { id: "refund", label: "Slot / Coin Refunds" },
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
  datePreset,
  dateFrom,
  dateTo,
  onPresetSelect,
  onCustomDateChange,
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

      {/* Middle Row: Search, Status, Package, Amount Range */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <div className="relative sm:col-span-2">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search by user name, email, order code, or transaction ID..."
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
          <SelectTrigger className="h-9 text-xs" aria-label="Filter by status">
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
            className="h-9 text-xs"
            aria-label="Filter by package or feature"
          >
            <SelectValue placeholder="All Packages / Features" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Packages / Features</SelectItem>
            {packageOptions.map((pkg) => (
              <SelectItem key={pkg} value={pkg}>
                {pkg}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="flex items-center gap-1.5">
          <Input
            type="number"
            min={0}
            value={amountMin}
            onChange={(e) => onAmountMinChange(e.target.value)}
            placeholder="Min amount"
            aria-label="Minimum amount"
            className="h-9 text-xs"
          />
          <span className="text-xs text-muted-foreground">–</span>
          <Input
            type="number"
            min={0}
            value={amountMax}
            onChange={(e) => onAmountMaxChange(e.target.value)}
            placeholder="Max amount"
            aria-label="Maximum amount"
            className="h-9 text-xs"
          />
        </div>
      </div>

      {/* Bottom Row: Date Presets & Custom Range */}
      <div className="flex flex-wrap items-center justify-between gap-3 pt-1">
        <DatePresetPicker
          activePreset={datePreset}
          dateFrom={dateFrom}
          dateTo={dateTo}
          onPresetSelect={onPresetSelect}
          onCustomDateChange={onCustomDateChange}
        />
      </div>
    </div>
  );
}
