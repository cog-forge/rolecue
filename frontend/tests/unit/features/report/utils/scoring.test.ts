import { describe, expect, it } from "vitest";
import {
  formatScore,
  overallFormula,
  uncoveredMetrics,
  weightShare,
} from "@/features/report/utils/scoring";
import type { Report } from "@/features/report/schemas/report-schema";
import { formattersFor } from "@tests/helpers/intl";

const TA = "55555555-5555-4555-8555-555555555551";
const PS = "55555555-5555-4555-8555-555555555552";
const CC = "55555555-5555-4555-8555-555555555553";

// The spec's worked example: metrics 70 and 50 weighted 60/40 give 62, not the mean of question totals (60).
const worked: Pick<Report, "overall_score" | "breakdown" | "scoring"> = {
  overall_score: 62,
  scoring: {
    method: "rubric",
    outcome: "complete",
    withheld_reason: null,
    assigned_questions: 2,
    graded_questions: 2,
    weight_total: 100,
    decimals: 2,
  },
  breakdown: [
    {
      metric_id: TA,
      name: "Technical Accuracy",
      score: 70,
      weight: 60,
      question_count: 2,
    },
    {
      metric_id: PS,
      name: "Problem-Solving",
      score: 50,
      weight: 40,
      question_count: 2,
    },
    { metric_id: CC, name: "Clarity", score: 90, weight: 0, question_count: 1 },
  ],
};

describe("score explanation helpers", () => {
  it("formats every score with two decimals in the reader's locale", () => {
    expect(formatScore(62, formattersFor("en").number)).toBe("62.00");
    expect(formatScore(62.5, formattersFor("vi").number)).toBe("62,50");
  });

  it("writes out the weighted overall with the stored values only", () => {
    expect(overallFormula(worked, formattersFor("en").number)).toBe(
      "(70.00 × 60 + 50.00 × 40) ÷ 100 = 62.00",
    );
  });

  it("has no formula without an overall or for a legacy evaluation", () => {
    const number = formattersFor("en").number;
    expect(
      overallFormula({ ...worked, overall_score: null }, number),
    ).toBeNull();
    expect(
      overallFormula(
        { ...worked, scoring: { ...worked.scoring!, method: "legacy" } },
        number,
      ),
    ).toBeNull();
  });

  it("lists the weighted metrics without evidence and each weight's share", () => {
    const breakdown = [
      ...worked.breakdown!,
      {
        metric_id: CC,
        name: "Relevance",
        score: null,
        weight: 1,
        question_count: 0,
      },
    ];
    expect(uncoveredMetrics(breakdown).map((metric) => metric.name)).toEqual([
      "Relevance",
    ]);
    expect(weightShare(60, 100)).toBe(60);
    expect(weightShare(1, 3)).toBeCloseTo(33.333, 2);
    expect(weightShare(1, 0)).toBe(0);
  });
});
