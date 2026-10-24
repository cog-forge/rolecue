"use client";

import { Button } from "@/components/ui/button";
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

export function DatePresetPicker({
  value,
  onChange,
  presets = DEFAULT_PRESETS,
}: {
  value: DatePreset;
  onChange: (preset: DatePreset) => void;
  presets?: PresetOption[];
}) {
  return (
    <div
      className="flex flex-wrap items-center gap-1.5"
      role="group"
      aria-label="Date range presets"
    >
      {presets.map((p) => {
        const isActive = value === p.value;
        return (
          <Button
            key={p.value}
            type="button"
            size="sm"
            variant={isActive ? "secondary" : "outline"}
            className={`h-8 text-xs font-medium transition-all duration-150 ${
              isActive
                ? "bg-primary text-primary-foreground hover:bg-primary/90 shadow-xs"
                : "text-muted-foreground hover:text-foreground"
            }`}
            onClick={() => onChange(p.value)}
            aria-pressed={isActive}
          >
            {p.label}
          </Button>
        );
      })}
    </div>
  );
}
