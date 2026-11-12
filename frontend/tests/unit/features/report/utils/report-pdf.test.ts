import { createTranslator } from "next-intl";
import { describe, expect, it } from "vitest";
import messages from "@/lib/i18n/messages/en/candidate.json";
import vietnamese from "@/lib/i18n/messages/vi/candidate.json";
import { reportPrintDocument } from "@/features/report/utils/report-pdf";
import type { Report } from "@/features/report/schemas/report-schema";
import { formattersFor } from "@tests/helpers/intl";

const t = createTranslator({
  locale: "en",
  messages: { candidate: messages },
  namespace: "candidate.report",
});
const ts = createTranslator({
  locale: "en",
  messages: { candidate: messages },
  namespace: "candidate.report.scoring",
});
const scoringT = (key: string, values?: Record<string, string | number>) =>
  ts(key as Parameters<typeof ts>[0], values);
const report: Report = {
  interview_id: "22222222-2222-4222-8222-222222222222",
  type: "practice",
  status: "completed",
  overall_score: 82,
  scoring: {
    method: "rubric",
    outcome: "complete",
    withheld_reason: null,
    assigned_questions: 2,
    graded_questions: 1,
    weight_total: 1,
    decimals: 2,
  },
  graded_questions: [
    {
      question_number: 1,
      status: "graded",
      rubric_name: "Explain a mechanism",
      total: 82,
      earned: 9,
      max: 11,
      metrics: [
        {
          metric_id: "55555555-5555-4555-8555-555555555555",
          name: "Depth",
          earned: 9,
          max: 11,
          score: 81.82,
        },
      ],
      criteria: [
        {
          criterion_id: "66666666-6666-4666-8666-666666666666",
          name: "Explains ordering",
          metric_id: "55555555-5555-4555-8555-555555555555",
          metric_name: "Depth",
          points: 9,
          max_points: 11,
          evidence: "Use partitions.",
          evidence_position: 1,
          feedback: "Good start.",
        },
      ],
    },
  ],
  started_at: "2026-11-03T01:00:00Z",
  completed_at: "2026-11-03T01:30:00Z",
  end_reason: "completed",
  summary: "Practise explaining trade-offs.",
  breakdown: [
    {
      metric_id: "55555555-5555-4555-8555-555555555555",
      name: "Depth",
      score: 82,
      weight: 1,
      question_count: 1,
    },
  ],
  questions: [
    {
      position: 1,
      question_number: 1,
      kind: "core",
      question: "Design a queue?",
      answer: "Use partitions.",
      feedback: "Explain ordering.",
    },
    {
      position: 2,
      question_number: 1,
      kind: "follow_up",
      question: "What about retries?",
      answer: null,
      feedback: "Consider idempotency.",
    },
    {
      position: 3,
      question_number: 2,
      kind: "core",
      question: "Describe a bug.",
      answer: "A race.",
      feedback: null,
    },
  ],
};

describe("practice PDF", () => {
  it("snapshots permitted text and groups follow-ups", () => {
    const html = reportPrintDocument(
      report,
      t,
      "en",
      "data:font/ttf;base64,TEST",
      formattersFor("en").number,
      scoringT,
    );
    const doc = new DOMParser().parseFromString(html, "text/html");
    expect(
      doc.body.textContent?.replace(/\s+/g, " ").trim(),
    ).toMatchInlineSnapshot(
      `"RoleCue — Practice interview 2026-11-03 Overall score82.00 / 100Complete Weighted average of the metric scores: sum of (metric score × weight) divided by the sum of the weights.(82.00 × 1) ÷ 1 = 82.00 Question totals are for reference only: the overall comes from the metric scores and their weights, not from the average of question totals. SummaryPractise explaining trade-offs. BreakdownDepth: 82.00 / 100 — Weight 100% QuestionsDesign a queue?Answer: Use partitions.Feedback: Explain ordering.Follow-up: What about retries?Answer: No answer recorded.Feedback: Consider idempotency.Grading: 9 / 11 points — Question total 82.00 (reference)Metrics in this question: Depth: 9/11 = 81.82Explains ordering (Depth): 9 / 11 pointsEvidence: “Use partitions.”Why: Good start.Describe a bug.Answer: A race. Scores are produced by AI against the rubric. They have not yet been calibrated against human experts."`,
    );
    expect(
      doc.querySelector("li .follow-up")?.parentElement?.querySelector("h3")
        ?.textContent,
    ).toBe("Design a queue?");
    expect(doc.title).toBe("rolecue-practice-report-2026-11-03.pdf");
  });
  it.each([
    "recruitment",
    "pending",
    "failed",
    "in_progress",
    "missing completion",
  ])("refuses %s even when called directly", (state) => {
    const data = { ...report };
    if (state === "recruitment") data.type = state;
    else if (state === "missing completion") data.completed_at = null;
    else if (
      state === "pending" ||
      state === "failed" ||
      state === "in_progress"
    )
      data.status = state;
    expect(() => reportPrintDocument(data, t, "en", "")).toThrow(
      "not exportable",
    );
  });
  it("exports a withheld overall with its reason instead of a score", () => {
    const html = reportPrintDocument(
      {
        ...report,
        overall_score: null,
        scoring: {
          ...report.scoring!,
          outcome: "withheld",
          withheld_reason: "missing_metric_coverage",
        },
        breakdown: [{ ...report.breakdown![0], score: null }],
      },
      t,
      "en",
      "",
      String,
      scoringT,
    );
    const text = new DOMParser()
      .parseFromString(html, "text/html")
      .body.textContent?.replace(/\s+/g, " ");
    expect(text).toContain("No overall score");
    expect(text).toContain("No graded question assessed Depth");
    expect(text).toContain("Depth: No evidence");
    expect(text).not.toContain("÷");
  });
  it("refuses a malformed completion date instead of injecting it into the title", () => {
    const data = { ...report, completed_at: "</title><script>x</script>" };
    expect(() => reportPrintDocument(data, t, "en", "")).toThrow(
      "not exportable",
    );
  });
  it("uses Vietnamese chrome and escapes stored content", () => {
    const vi = createTranslator({
      locale: "vi",
      messages: { candidate: vietnamese },
      namespace: "candidate.report",
    });
    const html = reportPrintDocument(
      { ...report, summary: '<script>alert("x")</script>' },
      vi,
      "vi",
      "data:font/ttf;base64,TEST",
    );
    const doc = new DOMParser().parseFromString(html, "text/html");
    expect(doc.querySelector("script")).toBeNull();
    expect(doc.body.textContent).toContain(vi("overallScore"));
    expect(doc.body.textContent).toContain("Explain ordering.");
    expect(doc.querySelector("style")?.textContent).toContain("@font-face");
    expect(doc.documentElement.lang).toBe("vi");
  });
});
