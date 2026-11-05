import { createTranslator } from "next-intl";
import { describe, expect, it } from "vitest";
import messages from "@/lib/i18n/messages/en/candidate.json";
import vietnamese from "@/lib/i18n/messages/vi/candidate.json";
import { reportPrintDocument } from "@/features/report/utils/report-pdf";
import type { Report } from "@/features/report/schemas/report-schema";

const t = createTranslator({
  locale: "en",
  messages: { candidate: messages },
  namespace: "candidate.report",
});
const report: Report = {
  interview_id: "22222222-2222-4222-8222-222222222222",
  type: "practice",
  status: "completed",
  overall_score: 82,
  started_at: "2026-11-03T01:00:00Z",
  completed_at: "2026-11-03T01:30:00Z",
  end_reason: "completed",
  summary: "Practise explaining trade-offs.",
  breakdown: [
    {
      metric_id: "55555555-5555-4555-8555-555555555555",
      name: "Depth",
      score: 82,
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
    );
    const doc = new DOMParser().parseFromString(html, "text/html");
    expect(
      doc.body.textContent?.replace(/\s+/g, " ").trim(),
    ).toMatchInlineSnapshot(
      `"RoleCue — Practice interview 2026-11-03 Overall score82 / 100 SummaryPractise explaining trade-offs. BreakdownDepth: 82 / 100 QuestionsDesign a queue?Answer: Use partitions.Feedback: Explain ordering.Follow-up: What about retries?Answer: No answer recorded.Feedback: Consider idempotency.Describe a bug.Answer: A race."`,
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
    "missing score",
  ])("refuses %s even when called directly", (state) => {
    const data = { ...report };
    if (state === "recruitment") data.type = state;
    else if (state === "missing completion") data.completed_at = null;
    else if (state === "missing score") data.overall_score = null;
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
