import { describe, expect, it } from "vitest";
import {
  formatDate,
  formatLanguageName,
  formatNumber,
  formatPercent,
  formatRelative,
  formatVnd,
} from "@/lib/i18n/formatters";
import { formattersFor } from "@tests/helpers/intl";

// Midday UTC keeps the calendar date the same in every time zone the tests may run in.
const instant = "2026-10-12T12:30:00Z";

describe("formatDate", () => {
  it("shows a calendar day the same for everyone, in each language", () => {
    expect(formatDate(instant, "en", "calendarDay")).toBe("Oct 12, 2026");
    expect(formatDate(instant, "vi", "calendarDay")).toBe("12 thg 10, 2026");
  });

  it("keeps a calendar day on the UTC date whatever the viewer's zone", () => {
    expect(formatDate("2026-10-12T23:59:00Z", "en", "calendarDay")).toBe(
      "Oct 12, 2026",
    );
  });

  it("shows a UTC instant with its time", () => {
    expect(formatDate(instant, "en", "calendarDayTime")).toBe(
      "Oct 12, 2026, 12:30 PM",
    );
    expect(formatDate(instant, "vi", "calendarDayTime")).toBe(
      "12:30 12 thg 10, 2026",
    );
  });

  it("formats a month and year", () => {
    expect(formatDate(instant, "en", "monthYear")).toBe("October 2026");
    expect(formatDate(instant, "vi", "monthYear")).toBe("tháng 10 năm 2026");
  });

  it("keeps the English day-first short date", () => {
    expect(formatDate(instant, "en", "dayMonthYear")).toBe("12 Oct 2026");
    expect(formatDate(instant, "vi", "dayMonthYear")).toBe("12 thg 10, 2026");
  });

  it("shows a moment in the viewer's own time zone and names the zone", () => {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    expect(formatDate(instant, "en", "moment")).toBe(
      new Intl.DateTimeFormat("en", {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "numeric",
        minute: "2-digit",
        timeZoneName: "short",
        timeZone: zone,
      }).format(new Date(instant)),
    );
    expect(formatDate(instant, "vi", "moment")).toMatch(/2026/);
    expect(formatDate(instant, "vi", "moment")).toMatch(/GMT|UTC/);
  });

  it("keeps Vietnam finance dates fixed as dd/MM/yyyy HH:mm in both languages", () => {
    for (const locale of ["en", "vi"] as const) {
      expect(formatDate(instant, locale, "vietnamDateTime")).toBe(
        "12/10/2026 19:30",
      );
      expect(formatDate(instant, locale, "vietnamDay")).toBe("12/10/2026");
    }
  });

  it("returns an empty string for an unreadable date instead of throwing", () => {
    expect(formatDate("not a date", "en")).toBe("");
    expect(formatDate("not a date", "vi")).toBe("");
  });
});

describe("formatRelative", () => {
  const now = Date.parse("2026-10-12T12:00:00Z");

  it("picks the unit and speaks the language", () => {
    expect(formatRelative(now - 5 * 60_000, "en", now)).toBe("5 minutes ago");
    expect(formatRelative(now - 5 * 60_000, "vi", now)).toBe("5 phút trước");
    expect(formatRelative(now - 3 * 3_600_000, "en", now)).toBe("3 hours ago");
    expect(formatRelative(now - 3 * 3_600_000, "vi", now)).toBe("3 giờ trước");
    expect(formatRelative(now + 3 * 86_400_000, "en", now)).toBe("in 3 days");
    expect(formatRelative(now + 3 * 86_400_000, "vi", now)).toMatch(
      /^sau 3 ngày/,
    );
  });

  it("uses a word for yesterday", () => {
    expect(formattersFor("en").relativeUnit(-1, "day")).toBe("yesterday");
    expect(formattersFor("vi").relativeUnit(-1, "day")).toBe("hôm qua");
  });
});

describe("numbers", () => {
  it("groups with the locale's separators", () => {
    expect(formatNumber(1234567.5, "en")).toBe("1,234,567.5");
    expect(formatNumber(1234567.5, "vi")).toBe("1.234.567,5");
  });

  it("formats a percentage with one decimal", () => {
    expect(formatPercent(12.5, "en")).toBe("12.5%");
    expect(formatPercent(12.5, "vi")).toBe("12,5%");
    expect(formatPercent(0, "en")).toBe("0.0%");
  });
});

describe("coins", () => {
  it("formats an amount without a unit", () => {
    expect(formattersFor("en").coinAmount(1500)).toBe("1,500");
    expect(formattersFor("vi").coinAmount(1500)).toBe("1.500");
  });

  it("adds the unit with the right plural", () => {
    const en = formattersFor("en");
    expect(en.coins(1)).toBe("1 coin");
    expect(en.coins(0)).toBe("0 coins");
    expect(en.coins(1500)).toBe("1,500 coins");
    const vi = formattersFor("vi");
    expect(vi.coins(1)).toBe("1 xu");
    expect(vi.coins(1500)).toBe("1.500 xu");
  });

  it("never converts the amount", () => {
    expect(formattersFor("vi").coinAmount(42)).toBe("42");
  });
});

describe("formatVnd", () => {
  it("keeps Vietnamese conventions in every language, unconverted", () => {
    expect(formatVnd(100000)).toBe("100.000 ₫");
    expect(formatVnd(2600000)).toBe("2.600.000 ₫");
    expect(formatVnd(0)).toBe("0 ₫");
    expect(formatVnd(-50000)).toBe("-50.000 ₫");
  });
});

describe("time left on the 24-hour interview deadline", () => {
  const now = Date.parse("2026-10-12T12:00:00Z");
  const after = (ms: number) => new Date(now + ms).toISOString();
  const cases: [number, string, string][] = [
    [-1000, "Deadline passed", "Đã hết hạn"],
    [30_000, "Less than a minute left", "Còn chưa đầy 1 phút"],
    [12 * 60_000, "12 min left", "Còn 12 phút"],
    [3 * 3_600_000 + 20 * 60_000, "3 h 20 min left", "Còn 3 giờ 20 phút"],
    [24 * 3_600_000, "24 h 0 min left", "Còn 24 giờ 0 phút"],
  ];

  it.each(cases)("reads correctly for %i ms remaining", (ms, en, vi) => {
    expect(formattersFor("en").timeLeft(after(ms), now)).toBe(en);
    expect(formattersFor("vi").timeLeft(after(ms), now)).toBe(vi);
  });
});

describe("formatLanguageName", () => {
  it("names a locale code in the UI language and falls back to the code", () => {
    expect(formatLanguageName("en-US", "en")).toBe("English (United States)");
    expect(formatLanguageName("en-US", "vi")).not.toBe("en-US");
    expect(formatLanguageName("not a code!", "en")).toBe("not a code!");
  });
});
