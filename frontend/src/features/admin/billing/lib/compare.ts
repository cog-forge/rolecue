import { formatCoinAmount, formatVND } from "./format";

// Period comparison for the revenue report. Every metric here is better when it rises. A rate moves
// in percentage points; a prior zero is "new", never an infinite rise.

export type ComparisonKind = "money" | "count" | "rate";

export type Comparison =
  | { state: "new"; tone: "neutral" }
  | { state: "same"; tone: "neutral" }
  | {
      state: "change";
      tone: "positive" | "negative";
      direction: "up" | "down";
      percent: string;
      absolute: string;
    };

const MINUS = "−";

function signed(value: number, kind: ComparisonKind) {
  const sign = value > 0 ? "+" : MINUS;
  const size = Math.abs(value);
  if (kind === "money") return `${sign}${formatVND(size)}`;
  if (kind === "rate") return `${sign}${size.toFixed(1)} pts`;
  return `${sign}${formatCoinAmount(size)}`;
}

export function compareValues(
  current: number,
  previous: number,
  kind: ComparisonKind,
): Comparison {
  const diff =
    kind === "rate"
      ? Math.round((current - previous) * 10) / 10
      : current - previous;
  if (diff === 0) return { state: "same", tone: "neutral" };
  if (previous === 0 && kind !== "rate")
    return { state: "new", tone: "neutral" };
  const up = diff > 0;
  return {
    state: "change",
    tone: up ? "positive" : "negative",
    direction: up ? "up" : "down",
    percent:
      kind === "rate"
        ? `${Math.abs(diff).toFixed(1)} pts`
        : `${Math.round((Math.abs(diff) / previous) * 100)}%`,
    absolute: signed(diff, kind),
  };
}

const VIETNAM_OFFSET_MS = 7 * 60 * 60 * 1000;
const months = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

function vietnamParts(instant: number) {
  const date = new Date(instant + VIETNAM_OFFSET_MS);
  return {
    day: date.getUTCDate(),
    month: months[date.getUTCMonth()],
    year: date.getUTCFullYear(),
  };
}

// "1–27 Oct 2026" for the window [from, to); the end is exclusive, so the last day is the day before.
export function formatWindowRange(fromISO: string, toISO: string): string {
  const from = vietnamParts(Date.parse(fromISO));
  const last = vietnamParts(Date.parse(toISO) - 1);
  if (from.year !== last.year) {
    return `${from.day} ${from.month} ${from.year} – ${last.day} ${last.month} ${last.year}`;
  }
  if (from.month !== last.month) {
    return `${from.day} ${from.month} – ${last.day} ${last.month} ${last.year}`;
  }
  if (from.day !== last.day) {
    return `${from.day}–${last.day} ${last.month} ${last.year}`;
  }
  return `${last.day} ${last.month} ${last.year}`;
}
