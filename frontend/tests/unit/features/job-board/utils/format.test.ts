import { describe, expect, it } from "vitest";
import {
  postedText,
  salaryText,
  targetLabel,
} from "@/features/job-board/utils/format";

describe("salaryText", () => {
  it.each([
    [10_000_000, 20_000_000, "10,000,000 – 20,000,000"],
    [1500, 1500, "1,500"],
    [1500, null, "From 1,500"],
    [null, 2500, "Up to 2,500"],
    [null, null, null],
  ])("formats %s to %s with no currency", (min, max, expected) => {
    expect(salaryText({ salary_min: min, salary_max: max })).toBe(expected);
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
    expect(postedText(createdAt, now)).toBe(expected);
  });
});

describe("targetLabel", () => {
  it("names known targets and humanizes unknown ones", () => {
    expect(targetLabel("programming_language")).toBe("Programming language");
    expect(targetLabel("cloud_platform")).toBe("Cloud platform");
    expect(targetLabel("")).toBe("Skill");
  });
});
