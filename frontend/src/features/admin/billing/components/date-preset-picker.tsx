"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { DatePreset } from "../schemas/billing-schema";
import { getPresetDates } from "../lib/date-presets";

export { getPresetDates };

interface PresetOption {
  value: DatePreset;
  label: string;
}

const DEFAULT_PRESETS: PresetOption[] = [
  { value: "today", label: "Today" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "thisMonth", label: "This month" },
  { value: "lastMonth", label: "Last month" },
  { value: "custom", label: "Custom" },
];

type DatePresetPickerProps = {
  activePreset?: DatePreset;
  value?: DatePreset;
  dateFrom?: string;
  dateTo?: string;
  onChange?: (preset: DatePreset) => void;
  onPresetSelect?: (preset: DatePreset, from: string, to: string) => void;
  onCustomDateChange?: (from: string, to: string) => void;
  presets?: PresetOption[];
};

export function DatePresetPicker({
  activePreset,
  value,
  dateFrom = "",
  dateTo = "",
  onChange,
  onPresetSelect,
  onCustomDateChange,
  presets = DEFAULT_PRESETS,
}: DatePresetPickerProps) {
  const currentPreset = activePreset ?? value ?? "30d";

  const handlePresetClick = (preset: DatePreset) => {
    onChange?.(preset);
    if (onPresetSelect) {
      if (preset === "custom") {
        onPresetSelect("custom", dateFrom, dateTo);
      } else {
        const range = getPresetDates(preset);
        onPresetSelect(preset, range.dateFrom, range.dateTo);
      }
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <div
        className="flex flex-wrap items-center gap-1.5"
        role="group"
        aria-label="Date range presets"
      >
        {presets.map((p) => {
          const isActive = currentPreset === p.value;
          return (
            <Button
              key={p.value}
              type="button"
              size="sm"
              variant={isActive ? "secondary" : "outline"}
              className={`h-8 text-xs font-medium transition-all duration-150 ${
                isActive
                  ? "bg-primary text-primary-foreground shadow-xs hover:bg-primary/90"
                  : "text-muted-foreground hover:text-foreground"
              }`}
              onClick={() => handlePresetClick(p.value)}
              aria-pressed={isActive}
            >
              {p.label}
            </Button>
          );
        })}
      </div>

      {onCustomDateChange && (
        <div className="flex items-center gap-1.5">
          <Input
            type="date"
            value={dateFrom}
            onChange={(e) => onCustomDateChange(e.target.value, dateTo)}
            aria-label="From date (UTC+7)"
            className="h-8 w-[138px] text-xs"
          />
          <span className="text-xs text-muted-foreground">to</span>
          <Input
            type="date"
            value={dateTo}
            onChange={(e) => onCustomDateChange(dateFrom, e.target.value)}
            aria-label="To date (UTC+7)"
            className="h-8 w-[138px] text-xs"
          />
        </div>
      )}
    </div>
  );
}
