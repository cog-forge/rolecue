import type { DatePreset } from "../schemas/billing-schema";

/**
 * Returns the Vietnam Time (UTC+7) YYYY-MM-DD range for a given preset.
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
  const utc = now.getTime() + now.getTimezoneOffset() * 60000;
  const vnNow = new Date(utc + 7 * 3600000);

  const year = vnNow.getFullYear();
  const month = vnNow.getMonth(); // 0-indexed
  const date = vnNow.getDate();

  const pad = (n: number) => String(n).padStart(2, "0");
  const formatVNDate = (y: number, m: number, d: number) =>
    `${y}-${pad(m + 1)}-${pad(d)}`;

  switch (preset) {
    case "today": {
      const todayStr = formatVNDate(year, month, date);
      return { dateFrom: todayStr, dateTo: todayStr };
    }
    case "7d": {
      const past = new Date(vnNow);
      past.setDate(past.getDate() - 6);
      return {
        dateFrom: formatVNDate(
          past.getFullYear(),
          past.getMonth(),
          past.getDate(),
        ),
        dateTo: formatVNDate(year, month, date),
      };
    }
    case "30d": {
      const past = new Date(vnNow);
      past.setDate(past.getDate() - 29);
      return {
        dateFrom: formatVNDate(
          past.getFullYear(),
          past.getMonth(),
          past.getDate(),
        ),
        dateTo: formatVNDate(year, month, date),
      };
    }
    case "90d": {
      const past = new Date(vnNow);
      past.setDate(past.getDate() - 89);
      return {
        dateFrom: formatVNDate(
          past.getFullYear(),
          past.getMonth(),
          past.getDate(),
        ),
        dateTo: formatVNDate(year, month, date),
      };
    }
    case "thisMonth": {
      return {
        dateFrom: formatVNDate(year, month, 1),
        dateTo: formatVNDate(year, month, date),
      };
    }
    case "lastMonth": {
      const lastMonthDate = new Date(year, month, 0);
      const lmYear = lastMonthDate.getFullYear();
      const lmMonth = lastMonthDate.getMonth();
      const lmDays = lastMonthDate.getDate();
      return {
        dateFrom: formatVNDate(lmYear, lmMonth, 1),
        dateTo: formatVNDate(lmYear, lmMonth, lmDays),
      };
    }
    default:
      return { dateFrom: "", dateTo: "" };
  }
}

export function getPresetDateRange(preset: DatePreset): {
  from: string;
  to: string;
} {
  const { dateFrom, dateTo } = getPresetDates(preset);
  return { from: dateFrom, to: dateTo };
}
