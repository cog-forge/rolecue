import type { useTranslations } from "next-intl";
import type { AppLocale } from "./config";

// Display formatting only: values (dates, amounts) come from the API unchanged and are never
// converted. Every screen formats through these helpers so a locale switch reaches all of them.

export type FormatTranslator = ReturnType<
  typeof useTranslations<"common.format">
>;
export type DateInput = Date | string | number;

const intlLocales: Record<AppLocale, string> = { en: "en", vi: "vi" };

// Named date shapes, one per way the app already shows a date. `timeZone` is set only where the
// value is a calendar day or a stored UTC instant; otherwise the viewer's own time zone applies.
export const dateStyles = {
  // A calendar day such as a posting date: the same day for everyone ("Oct 12, 2026").
  calendarDay: { dateStyle: "medium", timeZone: "UTC" },
  // A UTC instant with its time, for audit rows. Callers add the "UTC" label themselves.
  calendarDayTime: { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" },
  monthYear: { month: "long", year: "numeric", timeZone: "UTC" },
  // An instant in the viewer's time zone, without naming the zone.
  localDateTime: { dateStyle: "medium", timeStyle: "short" },
  // A short day-first date ("12 Oct 2026" in English).
  dayMonthYear: { day: "numeric", month: "short", year: "numeric" },
  // A moment such as an interview deadline: viewer's time zone, named to avoid misreading.
  moment: {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    timeZoneName: "short",
  },
  // The same moment with a padded hour, for dense tables.
  momentPadded: {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZoneName: "short",
  },
  transaction: {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  },
  // Admin finance reports are kept in Vietnam time as dd/MM/yyyy HH:mm in every language.
  vietnamDateTime: {
    timeZone: "Asia/Ho_Chi_Minh",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  },
  vietnamDay: {
    timeZone: "Asia/Ho_Chi_Minh",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  },
} satisfies Record<string, Intl.DateTimeFormatOptions>;
export type DateStyle = keyof typeof dateStyles;

// English keeps the day-first order it already showed; the Vietnam finance shapes are fixed.
function dateLocale(locale: AppLocale, style: DateStyle) {
  if (style === "vietnamDateTime" || style === "vietnamDay") return "en-GB";
  if (style === "dayMonthYear" && locale === "en") return "en-GB";
  return intlLocales[locale];
}

const dateFormats = new Map<string, Intl.DateTimeFormat>();
function dateFormat(locale: AppLocale, style: DateStyle) {
  const key = `${locale}:${style}`;
  let format = dateFormats.get(key);
  if (!format) {
    format = new Intl.DateTimeFormat(
      dateLocale(locale, style),
      dateStyles[style],
    );
    dateFormats.set(key, format);
  }
  return format;
}

function toDate(value: DateInput) {
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

// An unreadable date renders as an empty string instead of throwing during render.
export function formatDate(
  value: DateInput,
  locale: AppLocale,
  style: DateStyle = "calendarDay",
) {
  const date = toDate(value);
  if (!date) return "";
  const text = dateFormat(locale, style).format(date);
  return style === "vietnamDateTime" ? text.replace(",", "") : text;
}

const relativeFormats = new Map<AppLocale, Intl.RelativeTimeFormat>();
function relativeFormat(locale: AppLocale) {
  let format = relativeFormats.get(locale);
  if (!format) {
    format = new Intl.RelativeTimeFormat(intlLocales[locale], {
      numeric: "auto",
    });
    relativeFormats.set(locale, format);
  }
  return format;
}

// "yesterday" / "3 days ago" / "in 2 hours" for a known unit.
export function formatRelativeUnit(
  value: number,
  unit: Intl.RelativeTimeFormatUnit,
  locale: AppLocale,
) {
  const text = relativeFormat(locale).format(value, unit);
  // CLDR capitalises Vietnamese words such as "Hôm qua"; callers embed the text in a sentence.
  return locale === "vi" ? text.charAt(0).toLowerCase() + text.slice(1) : text;
}

const relativeSteps: [Intl.RelativeTimeFormatUnit, number][] = [
  ["minute", 60],
  ["hour", 24],
  ["day", 7],
  ["week", 4.35],
  ["month", 12],
];

// Picks the largest sensible unit: "5 minutes ago", "2 weeks ago".
export function formatRelative(
  value: DateInput,
  locale: AppLocale,
  now = Date.now(),
) {
  const date = toDate(value);
  if (!date) return "";
  let amount = (date.getTime() - now) / 60_000;
  for (const [unit, size] of relativeSteps) {
    if (Math.abs(amount) < size)
      return formatRelativeUnit(Math.round(amount), unit, locale);
    amount /= size;
  }
  return formatRelativeUnit(Math.round(amount), "year", locale);
}

export function formatNumber(
  value: number,
  locale: AppLocale,
  options?: Intl.NumberFormatOptions,
) {
  return new Intl.NumberFormat(intlLocales[locale], options).format(value);
}

// One decimal place with the locale's separator and a percent sign: "12.5%" / "12,5%".
export function formatPercent(value: number, locale: AppLocale, digits = 1) {
  return `${formatNumber(value, locale, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}%`;
}

// VND prices keep Vietnamese conventions in every UI language ("100.000 ₫"), matching what the
// payment provider and the bank show. The amount is never converted or rescaled.
export function formatVndAmount(amount: number) {
  return new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 0 }).format(
    amount,
  );
}

export function formatVnd(amount: number) {
  return `${formatVndAmount(amount)} ₫`;
}

// "Google, GitHub or Facebook" / "Google, GitHub hoặc Facebook".
export function formatList(
  items: string[],
  locale: AppLocale,
  type: "conjunction" | "disjunction" = "conjunction",
) {
  return new Intl.ListFormat(intlLocales[locale], { type }).format(items);
}

// The viewer's IANA time zone, for copy such as "Choose a future time in Asia/Bangkok".
export function userTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;

// Countdown to a deadline: "3 h 20 min left" / "Còn 3 giờ 20 phút". It never reads "0 min"
// while time remains.
export function formatTimeLeft(
  deadline: DateInput,
  now: number,
  t: FormatTranslator,
) {
  const date = toDate(deadline);
  if (!date) return "";
  const remaining = date.getTime() - now;
  if (remaining <= 0) return t("timeLeft.passed");
  if (remaining < MINUTE) return t("timeLeft.lessThanMinute");
  const hours = Math.floor(remaining / HOUR);
  const minutes = Math.floor((remaining % HOUR) / MINUTE);
  return hours === 0
    ? t("timeLeft.minutes", { minutes })
    : t("timeLeft.hoursMinutes", { hours, minutes });
}

// The helpers above bound to the active locale; `useFormatters()` and `getFormatters()` build it.
export function createFormatters(locale: AppLocale, t: FormatTranslator) {
  return {
    locale,
    date: (value: DateInput, style?: DateStyle) =>
      formatDate(value, locale, style),
    relative: (value: DateInput, now?: number) =>
      formatRelative(value, locale, now),
    relativeUnit: (value: number, unit: Intl.RelativeTimeFormatUnit) =>
      formatRelativeUnit(value, unit, locale),
    timeLeft: (deadline: DateInput, now: number) =>
      formatTimeLeft(deadline, now, t),
    list: (items: string[], type?: "conjunction" | "disjunction") =>
      formatList(items, locale, type),
    number: (value: number, options?: Intl.NumberFormatOptions) =>
      formatNumber(value, locale, options),
    percent: (value: number, digits?: number) =>
      formatPercent(value, locale, digits),
    // A coin balance or amount without a unit: "1,500" / "1.500".
    coinAmount: (value: number) => formatNumber(value, locale),
    // Coins with their unit and plural form: "1 coin", "2 coins".
    coins: (value: number) => t("coins", { count: value }),
    // A VND price with the ₫ sign, or just the digits when the page prints its own unit.
    vnd: formatVnd,
    vndAmount: formatVndAmount,
  };
}
export type Formatters = ReturnType<typeof createFormatters>;
