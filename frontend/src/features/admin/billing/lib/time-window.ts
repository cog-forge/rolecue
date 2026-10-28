import { formatDateOnly } from "./format";

/**
 * Admin billing time windows.
 *
 * Days are calendar days in Vietnam time (UTC+07:00), the same timezone the admin billing screens use to
 * show dates. Vietnam has no DST, so every day starts at T00:00:00+07:00. The API takes a [date_from,
 * date_to) window, so an inclusive last day is sent as the next midnight.
 */

export const timeWindowPresets = [
  { value: "all", label: "All time" },
  { value: "today", label: "Today" },
  { value: "last_7_days", label: "Last 7 days" },
  { value: "last_30_days", label: "Last 30 days" },
  { value: "this_month", label: "This month" },
  { value: "last_month", label: "Last month" },
  { value: "this_quarter", label: "This quarter" },
  { value: "this_year", label: "This year" },
  { value: "last_12_months", label: "Last 12 months" },
  { value: "custom", label: "Custom range" },
] as const;

export type TimeWindowPreset = (typeof timeWindowPresets)[number]["value"];

// The audit list reads day-sized windows; the revenue report reads calendar periods that compare
// cleanly with the same period a year earlier.
export const billingPresets: readonly TimeWindowPreset[] = [
  "all",
  "today",
  "last_7_days",
  "last_30_days",
  "this_month",
  "custom",
];
export const revenuePresets: readonly TimeWindowPreset[] = [
  "this_month",
  "last_month",
  "this_quarter",
  "this_year",
  "last_12_months",
  "custom",
];

// from and to are YYYY-MM-DD and both inclusive; they only apply to the custom preset.
export type TimeWindow = Readonly<{
  preset: TimeWindowPreset;
  from?: string;
  to?: string;
}>;

export type ResolvedTimeWindow =
  | { ok: true; dateFrom?: string; dateTo?: string }
  | { ok: false; error: string };

export const ALL_TIME: TimeWindow = { preset: "all" };

const VIETNAM_OFFSET_MS = 7 * 60 * 60 * 1000;
const dayPattern = /^\d{4}-\d{2}-\d{2}$/;

function isValidDay(value: string) {
  if (!dayPattern.test(value)) return false;
  const date = new Date(`${value}T00:00:00Z`);
  return (
    !Number.isNaN(date.getTime()) && date.toISOString().slice(0, 10) === value
  );
}

function addDays(day: string, days: number) {
  const date = new Date(`${day}T00:00:00Z`);
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

function vietnamToday(now: Date) {
  return new Date(now.getTime() + VIETNAM_OFFSET_MS).toISOString().slice(0, 10);
}

// The first day of the month `months` away from the month of `day`.
function monthStart(day: string, months = 0) {
  const date = new Date(`${day.slice(0, 8)}01T00:00:00Z`);
  date.setUTCMonth(date.getUTCMonth() + months);
  return date.toISOString().slice(0, 10);
}

function startOfDay(day: string) {
  return `${day}T00:00:00+07:00`;
}

type WindowDays =
  { ok: true; from?: string; to?: string } | { ok: false; error: string };

// The inclusive first and last day of a window.
function windowDays(window: TimeWindow, now: Date): WindowDays {
  const today = vietnamToday(now);
  switch (window.preset) {
    case "all":
      return { ok: true };
    case "today":
      return { ok: true, from: today, to: today };
    case "last_7_days":
      return { ok: true, from: addDays(today, -6), to: today };
    case "last_30_days":
      return { ok: true, from: addDays(today, -29), to: today };
    case "this_month":
      return { ok: true, from: monthStart(today), to: today };
    case "last_month":
      return {
        ok: true,
        from: monthStart(today, -1),
        to: addDays(monthStart(today), -1),
      };
    case "this_quarter": {
      const month = Number(today.slice(5, 7));
      return {
        ok: true,
        from: monthStart(today, -((month - 1) % 3)),
        to: today,
      };
    }
    case "this_year":
      return { ok: true, from: `${today.slice(0, 4)}-01-01`, to: today };
    case "last_12_months":
      return { ok: true, from: monthStart(today, -11), to: today };
    case "custom": {
      const { from, to } = window;
      if ((from && !isValidDay(from)) || (to && !isValidDay(to))) {
        return { ok: false, error: "Enter dates as valid calendar days." };
      }
      if (from && to && from > to) {
        return {
          ok: false,
          error: "The start date must be on or before the end date.",
        };
      }
      return { ok: true, from: from || undefined, to: to || undefined };
    }
  }
}

export function resolveTimeWindow(
  window: TimeWindow,
  now: Date,
): ResolvedTimeWindow {
  const days = windowDays(window, now);
  if (!days.ok) return days;
  return {
    ok: true,
    dateFrom: days.from ? startOfDay(days.from) : undefined,
    dateTo: days.to ? startOfDay(addDays(days.to, 1)) : undefined,
  };
}

export function timeWindowLabel(window: TimeWindow): string {
  if (window.preset !== "custom") {
    return (
      timeWindowPresets.find((p) => p.value === window.preset)?.label ??
      "All time"
    );
  }
  const { from, to } = window;
  if (from && to) return `${formatDateOnly(from)} – ${formatDateOnly(to)}`;
  if (from) return `From ${formatDateOnly(from)}`;
  if (to) return `Until ${formatDateOnly(to)}`;
  return "All time";
}

// File name part such as "_2026-10-01_2026-10-27"; empty for all time or an invalid range.
export function timeWindowFileSuffix(window: TimeWindow, now: Date): string {
  const days = windowDays(window, now);
  if (!days.ok) return "";
  if (days.from && days.to) return `_${days.from}_${days.to}`;
  if (days.from) return `_from_${days.from}`;
  if (days.to) return `_until_${days.to}`;
  return "";
}
