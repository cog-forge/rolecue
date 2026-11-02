import { describe, expect, it } from "vitest";
import {
  compareValues,
  formatWindowRange,
} from "@/features/admin/billing/lib/compare";
import { compareFor, datesFor } from "../fixtures/translators";

const t = compareFor("en");
const dates = datesFor("en");

describe("compareValues", () => {
  it("gives the percentage and the absolute change for money", () => {
    expect(compareValues(200000, 150000, "money", "en", t)).toEqual({
      state: "change",
      tone: "positive",
      direction: "up",
      kind: "money",
      percent: "33%",
      absolute: "+50.000 ₫",
    });
    expect(compareValues(66666, 75000, "money", "en", t)).toEqual({
      state: "change",
      tone: "negative",
      direction: "down",
      kind: "money",
      percent: "11%",
      absolute: "−8.334 ₫",
    });
  });

  it("says New instead of an infinite rise, and No change for equal values", () => {
    expect(compareValues(50000, 0, "money", "en", t)).toMatchObject({
      state: "new",
      tone: "neutral",
    });
    expect(compareValues(0, 0, "count", "en", t)).toMatchObject({
      state: "same",
      tone: "neutral",
    });
    expect(compareValues(3, 3, "count", "en", t)).toMatchObject({
      state: "same",
    });
  });

  it("measures rates in percentage points", () => {
    expect(compareValues(50, 66.7, "rate", "en", t)).toEqual({
      state: "change",
      tone: "negative",
      direction: "down",
      kind: "rate",
      percent: "16.7 pts",
      absolute: "−16.7 pts",
    });
  });

  it("counts whole numbers with a sign", () => {
    expect(compareValues(3, 2, "count", "en", t)).toMatchObject({
      percent: "50%",
      absolute: "+1",
    });
  });

  it("groups counts in the active language and names point changes in it", () => {
    const vi = compareFor("vi");
    expect(compareValues(2600, 100, "count", "vi", vi)).toMatchObject({
      absolute: "+2.500",
    });
    expect(compareValues(2600, 100, "count", "en", t)).toMatchObject({
      absolute: "+2,500",
    });
    expect(compareValues(50, 66.7, "rate", "vi", vi)).toMatchObject({
      kind: "rate",
      percent: "16,7 điểm %",
      absolute: "−16,7 điểm %",
    });
  });

  it("treats a drop to zero as a full fall", () => {
    expect(compareValues(0, 100000, "money", "en", t)).toMatchObject({
      tone: "negative",
      percent: "100%",
      absolute: "−100.000 ₫",
    });
  });
});

describe("formatWindowRange", () => {
  it("names an exclusive [from, to) window as inclusive Vietnam days", () => {
    expect(
      formatWindowRange("2026-09-30T17:00:00Z", "2026-10-27T17:00:00Z", dates),
    ).toBe("1–27 Oct 2026");
    expect(
      formatWindowRange("2026-07-31T17:00:00Z", "2026-10-27T17:00:00Z", dates),
    ).toBe("1 Aug – 27 Oct 2026");
    expect(
      formatWindowRange("2025-10-31T17:00:00Z", "2026-10-27T17:00:00Z", dates),
    ).toBe("1 Nov 2025 – 27 Oct 2026");
    expect(
      formatWindowRange("2026-10-26T17:00:00Z", "2026-10-27T17:00:00Z", dates),
    ).toBe("27 Oct 2026");
  });

  it("writes the window in Vietnamese month names", () => {
    const vi = datesFor("vi");
    expect(
      formatWindowRange("2026-09-30T17:00:00Z", "2026-10-27T17:00:00Z", vi),
    ).toBe("1–27 thg 10, 2026");
    expect(
      formatWindowRange("2026-07-31T17:00:00Z", "2026-10-27T17:00:00Z", vi),
    ).toBe("1 thg 8 – 27 thg 10, 2026");
    expect(
      formatWindowRange("2025-10-31T17:00:00Z", "2026-10-27T17:00:00Z", vi),
    ).toBe("1 thg 11, 2025 – 27 thg 10, 2026");
    expect(
      formatWindowRange("2026-10-26T17:00:00Z", "2026-10-27T17:00:00Z", vi),
    ).toBe("27 thg 10, 2026");
  });
});
