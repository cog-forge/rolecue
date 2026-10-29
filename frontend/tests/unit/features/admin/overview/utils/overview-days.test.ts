import { describe, expect, it } from "vitest";
import {
  defaultOverviewDays,
  overviewSearch,
  parseOverviewDays,
} from "@/features/admin/overview/utils/overview-days";

describe("parseOverviewDays", () => {
  it("accepts the three supported windows", () => {
    expect(parseOverviewDays("7")).toBe(7);
    expect(parseOverviewDays("30")).toBe(30);
    expect(parseOverviewDays("90")).toBe(90);
  });

  it("falls back to 30 for anything else", () => {
    for (const value of [null, "", "0", "14", "365", "7 ", "abc", "-7"]) {
      expect(parseOverviewDays(value)).toBe(defaultOverviewDays);
    }
  });
});

describe("overviewSearch", () => {
  it("leaves the default window out of the query string", () => {
    expect(overviewSearch(30)).toBe("");
  });

  it("writes the other windows", () => {
    expect(overviewSearch(7)).toBe("?days=7");
    expect(overviewSearch(90)).toBe("?days=90");
  });
});
