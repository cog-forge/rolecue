import { describe, expect, it } from "vitest";
import type {
  Metric,
  PostingWeights,
} from "@/features/recruiter/evaluation/schemas/evaluation-schema";
import {
  evaluateDrafts,
  formatPercent,
  formatSum,
  weightDrafts,
} from "@/features/recruiter/evaluation/utils/weight-drafts";

const accuracy: Metric = {
  id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
  name: "Accuracy",
};
const depth: Metric = {
  id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
  name: "Depth",
};
const clarity: Metric = {
  id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
  name: "Clarity",
};
const metrics = [accuracy, depth, clarity];

const saved = (over: Partial<PostingWeights>): PostingWeights => ({
  configured: true,
  using_defaults: false,
  editable: true,
  posting_status: "pending",
  total: 100,
  weights: [],
  ...over,
});

describe("weightDrafts", () => {
  it("starts a metric the saved set left out empty", () => {
    expect(
      weightDrafts(
        metrics,
        saved({
          weights: [
            { metric_id: accuracy.id, name: "Accuracy", percentage: 70 },
            { metric_id: depth.id, name: "Depth", percentage: 30 },
          ],
        }),
      ),
    ).toEqual({ [accuracy.id]: "70", [depth.id]: "30", [clarity.id]: "" });
  });

  it("shows the equal-split defaults as the starting values", () => {
    const drafts = weightDrafts(
      metrics,
      saved({
        configured: false,
        using_defaults: true,
        weights: metrics.map((m) => ({
          metric_id: m.id,
          name: m.name,
          percentage: 100 / 3,
        })),
      }),
    );
    expect(Number(drafts[accuracy.id])).toBeCloseTo(100 / 3, 10);
    // Saving the defaults unchanged must still add up to the total.
    expect(evaluateDrafts(metrics, drafts, 100).valid).toBe(true);
  });
});

describe("evaluateDrafts", () => {
  it("builds the request from the metrics above 0%, leaving the rest out", () => {
    const result = evaluateDrafts(
      metrics,
      { [accuracy.id]: "60", [depth.id]: "40", [clarity.id]: "0" },
      100,
    );
    expect(result.valid).toBe(true);
    expect(result.sum).toBe(100);
    expect(result.body).toEqual({
      weights: [
        { metric_id: accuracy.id, percentage: 60 },
        { metric_id: depth.id, percentage: 40 },
      ],
    });
  });

  it("treats a blank field as not weighted", () => {
    const result = evaluateDrafts(
      metrics,
      { [accuracy.id]: " 100 ", [depth.id]: "", [clarity.id]: "  " },
      100,
    );
    expect(result.valid).toBe(true);
    expect(result.body?.weights).toEqual([
      { metric_id: accuracy.id, percentage: 100 },
    ]);
  });

  it("accepts decimals that add up to the total", () => {
    expect(
      evaluateDrafts(
        metrics,
        { [accuracy.id]: "33.5", [depth.id]: "33.5", [clarity.id]: "33" },
        100,
      ).valid,
    ).toBe(true);
  });

  it("explains a sum that misses the total", () => {
    const result = evaluateDrafts(
      metrics,
      { [accuracy.id]: "60", [depth.id]: "30", [clarity.id]: "" },
      100,
    );
    expect(result.valid).toBe(false);
    expect(result.body).toBeNull();
    expect(result.sum).toBe(90);
    expect(result.formError).toBe(
      "The weights add up to 90, but they must add up to 100.",
    );
    expect(result.fieldErrors).toEqual({});
  });

  it("needs at least one weighted metric", () => {
    for (const drafts of [{}, { [accuracy.id]: "0", [depth.id]: "" }]) {
      const result = evaluateDrafts(metrics, drafts, 100);
      expect(result.valid).toBe(false);
      expect(result.formError).toBe("Give at least one metric a weight.");
    }
  });

  it("flags the field that cannot be sent", () => {
    const result = evaluateDrafts(
      metrics,
      { [accuracy.id]: "-5", [depth.id]: "abc", [clarity.id]: "101" },
      100,
    );
    expect(result.valid).toBe(false);
    expect(result.fieldErrors).toEqual({
      [accuracy.id]: "A weight can't be negative.",
      [depth.id]: "Enter a number.",
      [clarity.id]: "A weight can be at most 100.",
    });
    // The sum message waits until every field is fixed.
    expect(result.formError).toBeNull();
  });

  it("rejects values that are not finite numbers", () => {
    for (const bad of ["Infinity", "NaN", "1e999"]) {
      expect(
        evaluateDrafts(metrics, { [accuracy.id]: bad }, 100).fieldErrors[
          accuracy.id
        ],
      ).toBe("Enter a number.");
    }
  });

  it("has no sum rule when the total is 0, but still caps one weight", () => {
    const free = evaluateDrafts(
      metrics,
      { [accuracy.id]: "3", [depth.id]: "7" },
      0,
    );
    expect(free.valid).toBe(true);
    expect(
      evaluateDrafts(metrics, { [accuracy.id]: "1001" }, 0).fieldErrors[
        accuracy.id
      ],
    ).toBe("A weight can be at most 1000.");
  });
});

describe("formatSum", () => {
  it("shows enough digits that a sum never reads as equal to the total it misses", () => {
    expect(formatSum(99.996, 100)).toBe("99.996");
    expect(formatSum(100.004, 100)).toBe("100.004");
    expect(formatSum(80, 100)).toBe("80");
    expect(formatSum(100, 100)).toBe("100");
  });

  it("explains a near miss with different numbers", () => {
    const result = evaluateDrafts(
      metrics,
      { [accuracy.id]: "33.332", [depth.id]: "33.332", [clarity.id]: "33.332" },
      100,
    );
    expect(result.formError).toBe(
      "The weights add up to 99.996, but they must add up to 100.",
    );
  });
});

describe("formatPercent", () => {
  it("trims to two decimals", () => {
    expect(formatPercent(100)).toBe("100");
    expect(formatPercent(100 / 3)).toBe("33.33");
    expect(formatPercent(12.5)).toBe("12.5");
  });
});
