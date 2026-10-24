import type { DatePreset } from "../schemas/billing-schema";

/**
 * Returns the Vietnam Time (UTC+7) ISO range for a given preset.
 * Formatted as "YYYY-MM-DDTHH:mm:ss+07:00" or ISO string.
 */
export function getPresetDates(preset: DatePreset): {
  dateFrom: string;
  dateTo: string;
} {
  if (preset === "custom") {
    return { dateFrom: "", dateTo: "" };
  }

  // Get current time in Vietnam Time (UTC+7)
  const now = new Date();
  // Offset to UTC+7: UTC time + 7 hours
  const utc = now.getTime() + now.getTimezoneOffset() * 60000;
  const vnNow = new Date(utc + 7 * 3600000);

  const year = vnNow.getFullYear();
  const month = vnNow.getMonth(); // 0-indexed
  const date = vnNow.getDate();

  const pad = (n: number) => String(n).padStart(2, "0");
  const formatVNStart = (y: number, m: number, d: number) =>
    `${y}-${pad(m + 1)}-${pad(d)}T00:00:00+07:00`;
  const formatVNEnd = (y: number, m: number, d: number) =>
    `${y}-${pad(m + 1)}-${pad(d)}T23:59:59+07:00`;

  switch (preset) {
    case "today": {
      return {
        dateFrom: formatVNStart(year, month, date),
        dateTo: formatVNEnd(year, month, date),
      };
    }
    case "7d": {
      const past = new Date(vnNow);
      past.setDate(past.getDate() - 6);
      return {
        dateFrom: formatVNStart(
          past.getFullYear(),
          past.getMonth(),
          past.getDate(),
        ),
        dateTo: formatVNEnd(year, month, date),
      };
    }
    case "30d": {
      const past = new Date(vnNow);
      past.setDate(past.getDate() - 29);
      return {
        dateFrom: formatVNStart(
          past.getFullYear(),
          past.getMonth(),
          past.getDate(),
        ),
        dateTo: formatVNEnd(year, month, date),
      };
    }
    case "90d": {
      const past = new Date(vnNow);
      past.setDate(past.getDate() - 89);
      return {
        dateFrom: formatVNStart(
          past.getFullYear(),
          past.getMonth(),
          past.getDate(),
        ),
        dateTo: formatVNEnd(year, month, date),
      };
    }
    case "thisMonth": {
      return {
        dateFrom: formatVNStart(year, month, 1),
        dateTo: formatVNEnd(year, month, date),
      };
    }
    case "lastMonth": {
      // Last day of last month is day 0 of this month
      const lastMonthDate = new Date(year, month, 0);
      const lmYear = lastMonthDate.getFullYear();
      const lmMonth = lastMonthDate.getMonth();
      const lmDays = lastMonthDate.getDate();
      return {
        dateFrom: formatVNStart(lmYear, lmMonth, 1),
        dateTo: formatVNEnd(lmYear, lmMonth, lmDays),
      };
    }
    default:
      return { dateFrom: "", dateTo: "" };
  }
}
