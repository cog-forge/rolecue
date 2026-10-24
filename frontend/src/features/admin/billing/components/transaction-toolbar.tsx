"use client";

import { Search, SlidersHorizontal, X, Download } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  type AdminTransactionQuery,
  type TransactionStatusFilter,
  type TransactionTypeFilter,
  type DatePreset,
} from "../schemas/billing-schema";
import { DatePresetPicker, getPresetDates } from "./date-preset-picker";
import { getExportCSVUrl } from "../api/billing";

interface TransactionToolbarProps {
  query: AdminTransactionQuery;
  search: string;
  onSearchChange: (value: string) => void;
  onChange: (query: AdminTransactionQuery) => void;
  preset: DatePreset;
  onPresetChange: (preset: DatePreset) => void;
  disabled?: boolean;
}

const STATUS_OPTIONS: { value: TransactionStatusFilter; label: string }[] = [
  { value: "all", label: "All statuses" },
  { value: "success", label: "Success" },
  { value: "pending", label: "Pending" },
  { value: "failed", label: "Failed" },
  { value: "cancelled", label: "Cancelled" },
];

const TYPE_OPTIONS: { value: TransactionTypeFilter; label: string }[] = [
  { value: "all", label: "All types" },
  { value: "topup", label: "Top-up (PayOS)" },
  { value: "spend", label: "Coin spending" },
  { value: "refund", label: "Refund" },
];

const PACKAGE_OPTIONS = [
  { value: "all", label: "All packages" },
  { value: "Starter", label: "Starter Package" },
  { value: "Pro", label: "Pro Practice" },
  { value: "Enterprise", label: "Enterprise Boost" },
];

const PAGE_SIZE_OPTIONS = [
  { value: "20", label: "20 / page" },
  { value: "50", label: "50 / page" },
  { value: "100", label: "100 / page" },
];

export function TransactionToolbar({
  query,
  search,
  onSearchChange,
  onChange,
  preset,
  onPresetChange,
  disabled = false,
}: TransactionToolbarProps) {
  const isFiltered =
    search.trim() !== "" ||
    (query.status && query.status !== "all") ||
    (query.type && query.type !== "all") ||
    (query.package && query.package !== "all") ||
    query.amountMin != null ||
    query.amountMax != null ||
    (query.dateFrom && query.dateFrom !== "") ||
    (query.dateTo && query.dateTo !== "") ||
    query.includeTest === true;

  function clearFilters() {
    onSearchChange("");
    onPresetChange("custom");
    onChange({
      page: 1,
      pageSize: query.pageSize || 20,
      search: "",
      status: "all",
      type: "all",
      package: "all",
      amountMin: undefined,
      amountMax: undefined,
      dateFrom: "",
      dateTo: "",
      includeTest: false,
      tab: query.tab,
    });
  }

  function handlePresetSelect(newPreset: DatePreset) {
    onPresetChange(newPreset);
    if (newPreset !== "custom") {
      const { dateFrom, dateTo } = getPresetDates(newPreset);
      onChange({
        ...query,
        page: 1,
        dateFrom,
        dateTo,
      });
    }
  }

  function handleExport() {
    const url = getExportCSVUrl({
      ...query,
      search,
    });
    window.open(url, "_blank");
  }

  return (
    <div
      className="space-y-3.5 bg-card rounded-xl border border-border/70 p-4 shadow-xs"
      role="search"
      aria-label="Filter transactions"
    >
      {/* Row 1: Search + Status + Type + Package */}
      <div className="grid gap-2.5 sm:grid-cols-2 lg:grid-cols-4">
        {/* Search Input */}
        <div className="relative">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <Input
            id="tx-search"
            type="search"
            className="h-9.5 bg-background pl-9 text-xs"
            placeholder="Search ID or description…"
            maxLength={200}
            disabled={disabled}
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
          />
        </div>

        {/* Status */}
        <Select
          value={query.status || "all"}
          disabled={disabled}
          onValueChange={(v) =>
            onChange({
              ...query,
              page: 1,
              status: v,
            })
          }
        >
          <SelectTrigger
            className="h-9.5 bg-background text-xs"
            aria-label="Filter by status"
          >
            <SlidersHorizontal
              className="mr-2 h-3.5 w-3.5 text-muted-foreground"
              aria-hidden
            />
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            {STATUS_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value} className="text-xs">
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Type */}
        <Select
          value={query.type || "all"}
          disabled={disabled}
          onValueChange={(v) =>
            onChange({
              ...query,
              page: 1,
              type: v,
            })
          }
        >
          <SelectTrigger
            className="h-9.5 bg-background text-xs"
            aria-label="Filter by type"
          >
            <SelectValue placeholder="Type" />
          </SelectTrigger>
          <SelectContent>
            {TYPE_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value} className="text-xs">
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Package */}
        <Select
          value={query.package || "all"}
          disabled={disabled}
          onValueChange={(v) =>
            onChange({
              ...query,
              page: 1,
              package: v,
            })
          }
        >
          <SelectTrigger
            className="h-9.5 bg-background text-xs"
            aria-label="Filter by package"
          >
            <SelectValue placeholder="Package" />
          </SelectTrigger>
          <SelectContent>
            {PACKAGE_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value} className="text-xs">
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Row 2: Date presets + Custom date range + Amount Range + Test switch + Export */}
      <div className="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-border/40">
        <div className="flex flex-wrap items-center gap-2">
          <DatePresetPicker value={preset} onChange={handlePresetSelect} />

          {preset === "custom" && (
            <div className="flex items-center gap-1.5 text-xs">
              <Input
                type="date"
                className="h-8 w-34 text-xs bg-background"
                aria-label="From date"
                value={query.dateFrom?.split("T")[0] || ""}
                onChange={(e) =>
                  onChange({
                    ...query,
                    page: 1,
                    dateFrom: e.target.value
                      ? `${e.target.value}T00:00:00+07:00`
                      : "",
                  })
                }
              />
              <span className="text-muted-foreground text-xs">—</span>
              <Input
                type="date"
                className="h-8 w-34 text-xs bg-background"
                aria-label="To date"
                value={query.dateTo?.split("T")[0] || ""}
                onChange={(e) =>
                  onChange({
                    ...query,
                    page: 1,
                    dateTo: e.target.value
                      ? `${e.target.value}T23:59:59+07:00`
                      : "",
                  })
                }
              />
            </div>
          )}
        </div>

        {/* Right controls: Page size, Test toggle, Export, Clear */}
        <div className="flex flex-wrap items-center gap-2.5">
          {/* Test toggle */}
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground bg-muted/40 px-2 py-1 rounded-md border border-border/50">
            <Switch
              id="include-test"
              checked={query.includeTest ?? false}
              onCheckedChange={(checked) =>
                onChange({
                  ...query,
                  page: 1,
                  includeTest: checked,
                })
              }
            />
            <label
              htmlFor="include-test"
              className="cursor-pointer select-none text-[11px] font-medium"
            >
              Show test data
            </label>
          </div>

          {/* Page size */}
          <Select
            value={String(query.pageSize || 20)}
            onValueChange={(v) =>
              onChange({
                ...query,
                page: 1,
                pageSize: parseInt(v, 10),
              })
            }
          >
            <SelectTrigger className="h-8 w-26 bg-background text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZE_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value} className="text-xs">
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {/* Export CSV button */}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-8 gap-1.5 text-xs"
            onClick={handleExport}
            disabled={disabled}
            title="Export CSV matching current filters"
          >
            <Download className="h-3.5 w-3.5" />
            Export CSV
          </Button>

          {/* Clear button */}
          {isFiltered && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-8 gap-1 text-xs text-muted-foreground hover:text-foreground"
              onClick={clearFilters}
              disabled={disabled}
            >
              <X className="h-3.5 w-3.5" />
              Reset
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
