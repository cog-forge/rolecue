/**
 * Shared formatting utilities for RoleCue Admin Billing & Finance.
 *
 * Rules:
 * - Currency: VND formatted as "100.000 ₫" (dot thousands separator, space, ₫ symbol).
 * - Coin counts use the active locale's grouping ("1,500" in English).
 * - Timezone: Vietnam Time (UTC+7 / Asia/Ho_Chi_Minh).
 *   Tables: dd/MM/yyyy HH:mm
 *   Daily reports: dd/MM/yyyy
 * - Correct singular/plural (e.g. "1 deposit", "2 deposits").
 * - Coins and real money are ALWAYS separate fields and never mixed.
 */

import { defaultLocale, type AppLocale } from "@/lib/i18n/config";
import {
  formatDate,
  formatNumber,
  formatPercent,
  formatVnd,
} from "@/lib/i18n/formatters";

// VND and Vietnam time follow Vietnamese conventions in every UI language ("100.000 ₫"); coin
// counts group like any other number in the active locale ("1,500" / "1.500").
export function formatVND(amount: number | null | undefined): string {
  if (amount == null || Number.isNaN(amount)) return "0 ₫";
  return formatVnd(Math.round(amount));
}

export function formatCoinAmount(
  coins: number | null | undefined,
  locale: AppLocale = defaultLocale,
): string {
  if (coins == null || Number.isNaN(coins)) return "0";
  return formatNumber(Math.round(coins), locale);
}

export function formatCoins(
  coins: number | null | undefined,
  locale: AppLocale = defaultLocale,
): string {
  const label = Math.abs(Math.round(coins ?? 0)) === 1 ? "coin" : "coins";
  return `${formatCoinAmount(coins, locale)} ${label}`;
}

export function formatDateTime(
  isoOrDate: string | Date | null | undefined,
): string {
  if (!isoOrDate) return "-";
  return formatDate(isoOrDate, defaultLocale, "vietnamDateTime") || "-";
}

export function formatDateOnly(
  isoOrDate: string | Date | null | undefined,
): string {
  if (!isoOrDate) return "-";
  // A plain YYYY-MM-DD is already a calendar day; parsing it would shift it by the time zone.
  if (typeof isoOrDate === "string" && /^\d{4}-\d{2}-\d{2}$/.test(isoOrDate)) {
    const [year, month, day] = isoOrDate.split("-");
    return `${day}/${month}/${year}`;
  }
  return formatDate(isoOrDate, defaultLocale, "vietnamDay") || "-";
}

export function plural(
  count: number,
  singular: string,
  pluralForm?: string,
  locale: AppLocale = defaultLocale,
): string {
  const form =
    Math.abs(count) === 1 ? singular : (pluralForm ?? `${singular}s`);
  return `${formatNumber(count, locale)} ${form}`;
}

export const formatPlural = plural;

export function formatPercentChange(
  pct: number | null | undefined,
  locale: AppLocale = defaultLocale,
): {
  text: string;
  isPositive: boolean;
  isNeutral: boolean;
} {
  if (pct == null || Number.isNaN(pct)) {
    return { text: "0.0%", isPositive: false, isNeutral: true };
  }
  const isNeutral = Math.abs(pct) < 0.05;
  const isPositive = pct > 0;
  const prefix = isPositive ? "+" : "";
  return {
    text: `${prefix}${formatPercent(pct, locale)}`,
    isPositive,
    isNeutral,
  };
}

/**
 * Prevents CSV formula injection (CWE-1236) in spreadsheet applications
 * using the exact same rules as the backend `sanitizeCSVCell` helper.
 * Only string values are sanitized so negative numbers are never corrupted.
 */
export function sanitizeCSVCell(val: string | null | undefined): string {
  if (val == null || val === "") return "";
  switch (val[0]) {
    case "=":
    case "+":
    case "-":
    case "@":
    case "\t":
    case "\r":
      return `'${val}`;
    default:
      return val;
  }
}

export function encodeCSVRow(
  cells: (string | number | null | undefined)[],
): string {
  return cells
    .map((cell) => {
      if (cell == null) return '""';
      if (typeof cell === "number") {
        return `"${String(cell)}"`;
      }
      return `"${sanitizeCSVCell(cell).replace(/"/g, '""')}"`;
    })
    .join(",");
}
