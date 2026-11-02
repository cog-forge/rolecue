import type { useTranslations } from "next-intl";

// Labels for the days and months in the revenue report. The windows are Vietnam calendar days
// (UTC+07:00); only the month names and the order of the parts follow the active language.

export type DatesTranslator = ReturnType<
  typeof useTranslations<"admin.billing.dates">
>;

const VIETNAM_OFFSET_MS = 7 * 60 * 60 * 1000;

export function vietnamParts(instant: number) {
  const date = new Date(instant + VIETNAM_OFFSET_MS);
  return {
    day: date.getUTCDate(),
    month: date.getUTCMonth() + 1,
    year: date.getUTCFullYear(),
  };
}

export function monthName(month: number, t: DatesTranslator) {
  return t("month", { month });
}
