"use client";

import { useId, useState } from "react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  ALL_TIME,
  billingPresets,
  resolveTimeWindow,
  timeWindowPresets,
  type ResolvedTimeWindow,
  type TimeWindow,
  type TimeWindowPreset,
} from "../lib/time-window";

export type AppliedTimeWindow = Readonly<{
  window: TimeWindow;
  dateFrom?: string;
  dateTo?: string;
}>;

type TimeWindowOptions = {
  initial?: TimeWindow;
  // The revenue report compares two closed windows, so it needs both ends.
  requireBothEnds?: boolean;
};

function resolve(
  window: TimeWindow,
  requireBothEnds: boolean,
): ResolvedTimeWindow {
  const result = resolveTimeWindow(window, new Date());
  if (result.ok && requireBothEnds && (!result.dateFrom || !result.dateTo)) {
    return { ok: false, error: "Choose both a start and an end date." };
  }
  return result;
}

// Keeps the window being edited and the last valid one. Requests only use the applied window, so a
// reversed custom range shows an error without sending anything. Presets resolve when chosen, which
// keeps query keys stable while the page stays open.
export function useTimeWindow({
  initial = ALL_TIME,
  requireBothEnds = false,
}: TimeWindowOptions = {}) {
  const [draft, setDraft] = useState<TimeWindow>(initial);
  const [applied, setApplied] = useState<AppliedTimeWindow>(() => {
    const result = resolve(initial, requireBothEnds);
    return result.ok
      ? { window: initial, dateFrom: result.dateFrom, dateTo: result.dateTo }
      : { window: ALL_TIME };
  });
  const resolved = resolve(draft, requireBothEnds);

  const change = (next: TimeWindow) => {
    setDraft(next);
    const result = resolve(next, requireBothEnds);
    if (result.ok) {
      setApplied({
        window: next,
        dateFrom: result.dateFrom,
        dateTo: result.dateTo,
      });
    }
  };

  return {
    draft,
    applied,
    error: resolved.ok ? undefined : resolved.error,
    change,
    reset: () => change(initial),
  };
}

type TimeWindowFilterProps = {
  value: TimeWindow;
  onChange: (window: TimeWindow) => void;
  error?: string;
  presets?: readonly TimeWindowPreset[];
};

export function TimeWindowFilter({
  value,
  onChange,
  error,
  presets = billingPresets,
}: TimeWindowFilterProps) {
  const id = useId();
  const fromId = `${id}-from`;
  const toId = `${id}-to`;
  const errorId = `${id}-error`;
  const isCustom = value.preset === "custom";

  return (
    <div className="flex w-full flex-wrap items-end gap-2 sm:w-auto">
      <Select
        value={value.preset}
        onValueChange={(preset) =>
          onChange(
            preset === "custom"
              ? { preset: "custom", from: value.from, to: value.to }
              : { preset: preset as TimeWindowPreset },
          )
        }
      >
        <SelectTrigger
          className="h-9 w-full text-xs sm:w-[160px]"
          aria-label="Time window"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {timeWindowPresets
            .filter((preset) => presets.includes(preset.value))
            .map((preset) => (
              <SelectItem key={preset.value} value={preset.value}>
                {preset.label}
              </SelectItem>
            ))}
        </SelectContent>
      </Select>

      {isCustom && (
        <>
          <div className="flex min-w-0 flex-1 flex-col gap-1 sm:flex-none">
            <label
              htmlFor={fromId}
              className="text-xs font-medium text-muted-foreground"
            >
              From
            </label>
            <Input
              id={fromId}
              type="date"
              value={value.from ?? ""}
              max={value.to || undefined}
              onChange={(e) =>
                onChange({ ...value, from: e.target.value || undefined })
              }
              aria-invalid={Boolean(error)}
              aria-describedby={error ? errorId : undefined}
              className="h-9 text-xs"
            />
          </div>
          <div className="flex min-w-0 flex-1 flex-col gap-1 sm:flex-none">
            <label
              htmlFor={toId}
              className="text-xs font-medium text-muted-foreground"
            >
              To
            </label>
            <Input
              id={toId}
              type="date"
              value={value.to ?? ""}
              min={value.from || undefined}
              onChange={(e) =>
                onChange({ ...value, to: e.target.value || undefined })
              }
              aria-invalid={Boolean(error)}
              aria-describedby={error ? errorId : undefined}
              className="h-9 text-xs"
            />
          </div>
        </>
      )}

      {isCustom && (
        <p className="w-full text-xs text-muted-foreground">
          Dates in Vietnam time (UTC+07:00); both ends are included.
        </p>
      )}
      {error && (
        <p
          id={errorId}
          role="alert"
          className="w-full text-xs text-destructive"
        >
          {error}
        </p>
      )}
    </div>
  );
}
