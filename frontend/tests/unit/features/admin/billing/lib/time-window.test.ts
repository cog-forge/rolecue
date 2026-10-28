import { describe, expect, it } from "vitest";
import {
  ALL_TIME,
  resolveTimeWindow,
  timeWindowFileSuffix,
  timeWindowLabel,
  type TimeWindow,
} from "@/features/admin/billing/lib/time-window";

// 27/10/2026 15:00 in Vietnam.
const now = new Date("2026-10-27T08:00:00Z");

function resolved(window: TimeWindow, at: Date = now) {
  const result = resolveTimeWindow(window, at);
  if (!result.ok) throw new Error(result.error);
  return { dateFrom: result.dateFrom, dateTo: result.dateTo };
}

describe("resolveTimeWindow", () => {
  it("sends no window for All time", () => {
    expect(resolved(ALL_TIME)).toEqual({
      dateFrom: undefined,
      dateTo: undefined,
    });
  });

  it("turns presets into [start, next midnight) in Vietnam time", () => {
    expect(resolved({ preset: "today" })).toEqual({
      dateFrom: "2026-10-27T00:00:00+07:00",
      dateTo: "2026-10-28T00:00:00+07:00",
    });
    expect(resolved({ preset: "last_7_days" })).toEqual({
      dateFrom: "2026-10-21T00:00:00+07:00",
      dateTo: "2026-10-28T00:00:00+07:00",
    });
    expect(resolved({ preset: "last_30_days" })).toEqual({
      dateFrom: "2026-09-28T00:00:00+07:00",
      dateTo: "2026-10-28T00:00:00+07:00",
    });
    expect(resolved({ preset: "this_month" })).toEqual({
      dateFrom: "2026-10-01T00:00:00+07:00",
      dateTo: "2026-10-28T00:00:00+07:00",
    });
  });

  it("uses the Vietnam calendar day, not the UTC one", () => {
    // 31/10/2026 23:30 UTC is already 01/11/2026 06:30 in Vietnam.
    const lateUtc = new Date("2026-10-31T23:30:00Z");
    expect(resolved({ preset: "today" }, lateUtc)).toEqual({
      dateFrom: "2026-11-01T00:00:00+07:00",
      dateTo: "2026-11-02T00:00:00+07:00",
    });
    expect(resolved({ preset: "this_month" }, lateUtc).dateFrom).toBe(
      "2026-11-01T00:00:00+07:00",
    );
  });

  it("crosses month and year ends", () => {
    const newYearsEve = new Date("2026-12-31T10:00:00Z");
    expect(resolved({ preset: "today" }, newYearsEve).dateTo).toBe(
      "2027-01-01T00:00:00+07:00",
    );
    const earlyJanuary = new Date("2027-01-03T10:00:00Z");
    expect(resolved({ preset: "last_7_days" }, earlyJanuary).dateFrom).toBe(
      "2026-12-28T00:00:00+07:00",
    );
  });

  it("makes the custom end date inclusive", () => {
    expect(
      resolved({ preset: "custom", from: "2026-10-01", to: "2026-10-31" }),
    ).toEqual({
      dateFrom: "2026-10-01T00:00:00+07:00",
      dateTo: "2026-11-01T00:00:00+07:00",
    });
    expect(
      resolved({ preset: "custom", from: "2026-10-05", to: "2026-10-05" }),
    ).toEqual({
      dateFrom: "2026-10-05T00:00:00+07:00",
      dateTo: "2026-10-06T00:00:00+07:00",
    });
  });

  it("allows a custom range with one open end", () => {
    expect(resolved({ preset: "custom", from: "2026-10-01" })).toEqual({
      dateFrom: "2026-10-01T00:00:00+07:00",
      dateTo: undefined,
    });
    expect(resolved({ preset: "custom", to: "2026-02-28" })).toEqual({
      dateFrom: undefined,
      dateTo: "2026-03-01T00:00:00+07:00",
    });
  });

  it("rejects a reversed or invalid custom range", () => {
    expect(
      resolveTimeWindow(
        { preset: "custom", from: "2026-10-10", to: "2026-10-01" },
        now,
      ),
    ).toEqual({
      ok: false,
      error: "The start date must be on or before the end date.",
    });
    expect(
      resolveTimeWindow({ preset: "custom", from: "2026-02-30" }, now).ok,
    ).toBe(false);
    expect(
      resolveTimeWindow({ preset: "custom", to: "27/10/2026" }, now).ok,
    ).toBe(false);
  });
});

describe("timeWindowLabel", () => {
  it("names presets and formats custom ranges as dd/MM/yyyy", () => {
    expect(timeWindowLabel(ALL_TIME)).toBe("All time");
    expect(timeWindowLabel({ preset: "last_7_days" })).toBe("Last 7 days");
    expect(timeWindowLabel({ preset: "this_month" })).toBe("This month");
    expect(
      timeWindowLabel({
        preset: "custom",
        from: "2026-10-01",
        to: "2026-10-27",
      }),
    ).toBe("01/10/2026 – 27/10/2026");
    expect(timeWindowLabel({ preset: "custom", from: "2026-10-01" })).toBe(
      "From 01/10/2026",
    );
    expect(timeWindowLabel({ preset: "custom", to: "2026-10-27" })).toBe(
      "Until 27/10/2026",
    );
    expect(timeWindowLabel({ preset: "custom" })).toBe("All time");
  });
});

describe("timeWindowFileSuffix", () => {
  it("puts the inclusive Vietnam dates into export file names", () => {
    expect(timeWindowFileSuffix(ALL_TIME, now)).toBe("");
    expect(timeWindowFileSuffix({ preset: "this_month" }, now)).toBe(
      "_2026-10-01_2026-10-27",
    );
    expect(
      timeWindowFileSuffix({ preset: "custom", from: "2026-10-01" }, now),
    ).toBe("_from_2026-10-01");
    expect(
      timeWindowFileSuffix({ preset: "custom", to: "2026-10-27" }, now),
    ).toBe("_until_2026-10-27");
    expect(
      timeWindowFileSuffix(
        { preset: "custom", from: "2026-10-10", to: "2026-10-01" },
        now,
      ),
    ).toBe("");
  });
});

describe("revenue presets", () => {
  it("resolves month, quarter, year and 12-month windows in Vietnam time", () => {
    expect(resolved({ preset: "last_month" })).toEqual({
      dateFrom: "2026-09-01T00:00:00+07:00",
      dateTo: "2026-10-01T00:00:00+07:00",
    });
    expect(resolved({ preset: "this_quarter" })).toEqual({
      dateFrom: "2026-10-01T00:00:00+07:00",
      dateTo: "2026-10-28T00:00:00+07:00",
    });
    expect(resolved({ preset: "this_year" })).toEqual({
      dateFrom: "2026-01-01T00:00:00+07:00",
      dateTo: "2026-10-28T00:00:00+07:00",
    });
    expect(resolved({ preset: "last_12_months" })).toEqual({
      dateFrom: "2025-11-01T00:00:00+07:00",
      dateTo: "2026-10-28T00:00:00+07:00",
    });
  });

  it("rolls last month back over a year end and finds the quarter's first month", () => {
    const january = new Date("2027-01-10T03:00:00Z");
    expect(resolved({ preset: "last_month" }, january)).toEqual({
      dateFrom: "2026-12-01T00:00:00+07:00",
      dateTo: "2027-01-01T00:00:00+07:00",
    });
    const may = new Date("2026-05-20T03:00:00Z");
    expect(resolved({ preset: "this_quarter" }, may).dateFrom).toBe(
      "2026-04-01T00:00:00+07:00",
    );
    expect(resolved({ preset: "last_12_months" }, may).dateFrom).toBe(
      "2025-06-01T00:00:00+07:00",
    );
  });

  it("labels the new presets", () => {
    expect(timeWindowLabel({ preset: "last_12_months" })).toBe(
      "Last 12 months",
    );
    expect(timeWindowLabel({ preset: "this_quarter" })).toBe("This quarter");
  });
});
