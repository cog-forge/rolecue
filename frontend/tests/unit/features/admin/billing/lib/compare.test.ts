import { describe, expect, it } from "vitest";
import {
  compareValues,
  formatWindowRange,
} from "@/features/admin/billing/lib/compare";

describe("compareValues", () => {
  it("gives the percentage and the absolute change for money", () => {
    expect(compareValues(200000, 150000, "money")).toEqual({
      state: "change",
      tone: "positive",
      direction: "up",
      percent: "33%",
      absolute: "+50.000 ₫",
    });
    expect(compareValues(66666, 75000, "money")).toEqual({
      state: "change",
      tone: "negative",
      direction: "down",
      percent: "11%",
      absolute: "−8.334 ₫",
    });
  });

  it("says New instead of an infinite rise, and No change for equal values", () => {
    expect(compareValues(50000, 0, "money")).toMatchObject({
      state: "new",
      tone: "neutral",
    });
    expect(compareValues(0, 0, "count")).toMatchObject({
      state: "same",
      tone: "neutral",
    });
    expect(compareValues(3, 3, "count")).toMatchObject({ state: "same" });
  });

  it("measures rates in percentage points", () => {
    expect(compareValues(50, 66.7, "rate")).toEqual({
      state: "change",
      tone: "negative",
      direction: "down",
      percent: "16.7 pts",
      absolute: "−16.7 pts",
    });
  });

  it("counts whole numbers with a sign", () => {
    expect(compareValues(3, 2, "count")).toMatchObject({
      percent: "50%",
      absolute: "+1",
    });
  });

  it("treats a drop to zero as a full fall", () => {
    expect(compareValues(0, 100000, "money")).toMatchObject({
      tone: "negative",
      percent: "100%",
      absolute: "−100.000 ₫",
    });
  });
});

describe("formatWindowRange", () => {
  it("names an exclusive [from, to) window as inclusive Vietnam days", () => {
    expect(
      formatWindowRange("2026-09-30T17:00:00Z", "2026-10-27T17:00:00Z"),
    ).toBe("1–27 Oct 2026");
    expect(
      formatWindowRange("2026-07-31T17:00:00Z", "2026-10-27T17:00:00Z"),
    ).toBe("1 Aug – 27 Oct 2026");
    expect(
      formatWindowRange("2025-10-31T17:00:00Z", "2026-10-27T17:00:00Z"),
    ).toBe("1 Nov 2025 – 27 Oct 2026");
    expect(
      formatWindowRange("2026-10-26T17:00:00Z", "2026-10-27T17:00:00Z"),
    ).toBe("27 Oct 2026");
  });
});
