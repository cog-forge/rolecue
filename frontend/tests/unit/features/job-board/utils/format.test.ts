import { describe, expect, it } from "vitest";
import {
  postedText,
  salaryText,
  targetLabel,
} from "@/features/job-board/utils/format";
import { createTranslator } from "next-intl";
import { formattersFor, messagesFor } from "@tests/helpers/intl";
import type { AppLocale } from "@/lib/i18n/config";

const fmt = formattersFor("en");
const tFor = (locale: AppLocale) =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "candidate.jobBoard",
  });
const t = tFor("en");

describe("salaryText", () => {
  it.each([
    [10_000_000, 20_000_000, "10,000,000 – 20,000,000"],
    [1500, 1500, "1,500"],
    [1500, null, "From 1,500"],
    [null, 2500, "Up to 2,500"],
    [null, null, null],
  ])("formats %s to %s with no currency", (min, max, expected) => {
    expect(salaryText({ salary_min: min, salary_max: max }, fmt, t)).toBe(
      expected,
    );
  });
});

describe("postedText", () => {
  const now = Date.parse("2026-10-18T12:00:00Z");
  it.each([
    ["2026-10-18T09:00:00Z", "Posted today"],
    ["2026-10-19T09:00:00Z", "Posted today"], // clock skew never says "in 1 day"
    ["2026-10-17T09:00:00Z", "Posted yesterday"],
    ["2026-10-15T12:00:00Z", "Posted 3 days ago"],
    ["2026-09-01T12:00:00Z", "Posted Sep 1, 2026"],
  ])("%s → %s", (createdAt, expected) => {
    expect(postedText(createdAt, fmt, t, now)).toBe(expected);
  });
});

describe("targetLabel", () => {
  it("names known targets and humanizes unknown ones", () => {
    expect(targetLabel("programming_language", t)).toBe("Programming language");
    expect(targetLabel("cloud_platform", t)).toBe("Cloud platform");
    expect(targetLabel("", t)).toBe("Skill");
  });
});

it("uses UTC calendar days for publication labels, matching the new-today badge", () => {
  const now = Date.parse("2026-10-29T00:01:00Z");
  expect(postedText("2026-10-28T23:59:00Z", fmt, t, now)).toBe(
    "Posted yesterday",
  );
  expect(postedText("2026-10-29T00:00:00Z", fmt, t, now)).toBe("Posted today");
});

describe("in Vietnamese", () => {
  const viT = tFor("vi");
  const viFmt = formattersFor("vi");

  it("translates the salary prefixes and keeps the numbers as stored", () => {
    expect(salaryText({ salary_min: 1500, salary_max: null }, viFmt, viT)).toBe(
      "Từ 1.500",
    );
    expect(salaryText({ salary_min: null, salary_max: 2500 }, viFmt, viT)).toBe(
      "Tối đa 2.500",
    );
  });

  it("words the publication age in Vietnamese", () => {
    const now = Date.parse("2026-10-18T12:00:00Z");
    expect(postedText("2026-10-18T09:00:00Z", viFmt, viT, now)).toBe(
      "Đăng hôm nay",
    );
    expect(postedText("2026-10-15T12:00:00Z", viFmt, viT, now)).toBe(
      "Đăng 3 ngày trước",
    );
  });

  it("translates known skill targets and leaves unknown ones as words", () => {
    expect(targetLabel("database", viT)).toBe("Cơ sở dữ liệu");
    expect(targetLabel("cloud_platform", viT)).toBe("Cloud platform");
  });
});
