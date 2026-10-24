/**
 * Shared formatting utilities for RoleCue Admin Billing & Finance.
 *
 * Rules:
 * - Currency: VND formatted as "100.000 ₫" (dot thousands separator, space, ₫ symbol).
 * - Timezone: Vietnam Time (UTC+7 / Asia/Ho_Chi_Minh).
 *   Tables: dd/MM/yyyy HH:mm
 *   Daily reports: dd/MM/yyyy
 * - Correct singular/plural (e.g. "1 deposit", "2 deposits").
 * - Coins and real money are ALWAYS separate fields and never mixed.
 */

export function formatVND(amount: number | null | undefined): string {
  if (amount == null || Number.isNaN(amount)) return "0 ₫";
  const isNegative = amount < 0;
  const abs = Math.abs(Math.round(amount));
  const formatted = abs.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${isNegative ? "-" : ""}${formatted} ₫`;
}

export function formatCoins(coins: number | null | undefined): string {
  if (coins == null || Number.isNaN(coins)) return "0 coins";
  const isNegative = coins < 0;
  const abs = Math.abs(Math.round(coins));
  const formatted = abs.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const label = abs === 1 ? "coin" : "coins";
  return `${isNegative ? "-" : ""}${formatted} ${label}`;
}

export function formatCoinAmount(coins: number | null | undefined): string {
  if (coins == null || Number.isNaN(coins)) return "0";
  const isNegative = coins < 0;
  const abs = Math.abs(Math.round(coins));
  const formatted = abs.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${isNegative ? "-" : ""}${formatted}`;
}

const vnDateFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const vnDateOnlyFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
});

export function formatDateTime(
  isoOrDate: string | Date | null | undefined,
): string {
  if (!isoOrDate) return "-";
  try {
    const d = typeof isoOrDate === "string" ? new Date(isoOrDate) : isoOrDate;
    if (Number.isNaN(d.getTime())) return "-";
    // en-GB outputs dd/mm/yyyy, hh:mm
    const parts = vnDateFormatter.format(d).replace(",", "");
    return parts;
  } catch {
    return "-";
  }
}

export function formatDateOnly(
  isoOrDate: string | Date | null | undefined,
): string {
  if (!isoOrDate) return "-";
  try {
    // If it's already YYYY-MM-DD string, parse carefully to avoid local TZ shift
    if (
      typeof isoOrDate === "string" &&
      /^\d{4}-\d{2}-\d{2}$/.test(isoOrDate)
    ) {
      const [year, month, day] = isoOrDate.split("-");
      return `${day}/${month}/${year}`;
    }
    const d = typeof isoOrDate === "string" ? new Date(isoOrDate) : isoOrDate;
    if (Number.isNaN(d.getTime())) return "-";
    return vnDateOnlyFormatter.format(d);
  } catch {
    return "-";
  }
}

export function plural(
  count: number,
  singular: string,
  pluralForm?: string,
): string {
  const form =
    Math.abs(count) === 1 ? singular : (pluralForm ?? `${singular}s`);
  return `${count.toLocaleString("en-US")} ${form}`;
}

export function formatPercentChange(pct: number | null | undefined): {
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
    text: `${prefix}${pct.toFixed(1)}%`,
    isPositive,
    isNeutral,
  };
}
