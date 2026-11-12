import { describe, expect, it } from "vitest";
import { coverageBlocks, coverageSchema } from "@/lib/api/coverage-schema";

const base = {
  passes: false,
  assessed: true,
  covered: 2,
  total: 5,
  question_limit: 8,
  missing_metrics: [],
};

describe("coverage contract", () => {
  it("parses the Go report and fills the optional hints", () => {
    const parsed = coverageSchema.parse({
      ...base,
      missing_metrics: [
        { id: "22222222-2222-4222-8222-222222222222", name: "Depth" },
      ],
    });
    expect(parsed.exceeds_limit).toBe(false);
    expect(parsed.min_questions).toBe(0);
    expect(parsed.missing_metrics[0].suggested_rubrics).toEqual([]);
  });

  it("blocks only an assessed bank that fails", () => {
    expect(coverageBlocks(coverageSchema.parse(base))).toBe(true);
    expect(
      coverageBlocks(coverageSchema.parse({ ...base, passes: true })),
    ).toBe(false);
    expect(
      coverageBlocks(coverageSchema.parse({ ...base, assessed: false })),
    ).toBe(false);
    expect(coverageBlocks(null)).toBe(false);
    expect(coverageBlocks(undefined)).toBe(false);
  });
});
