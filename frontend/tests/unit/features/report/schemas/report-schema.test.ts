import { describe, expect, it } from "vitest";
import {
  reportEnvelopeSchema,
  type Report,
} from "@/features/report/schemas/report-schema";

const INTERVIEW = "22222222-2222-4222-8222-222222222222";
const METRIC = "55555555-5555-4555-8555-555555555555";

const completedPractice: Report = {
  interview_id: INTERVIEW,
  type: "practice",
  status: "completed",
  overall_score: 72,
  started_at: "2026-10-20T08:00:00Z",
  completed_at: "2026-10-20T08:30:00Z",
  summary: "Clear structure, thin examples.",
  breakdown: [{ metric_id: METRIC, name: "Communication", score: 80 }],
  questions: [
    {
      position: 1,
      question: "Tell me about a project.",
      answer: "I built an API.",
      feedback: "Name the impact.",
    },
  ],
};

const envelope = (data: unknown) => ({ success: true, data });

describe("reportEnvelopeSchema", () => {
  it("accepts a completed practice report", () => {
    const parsed = reportEnvelopeSchema.parse(envelope(completedPractice));
    expect(parsed.data.breakdown?.[0].name).toBe("Communication");
  });

  it("accepts the recruitment shape the Candidate receives (null sections)", () => {
    const result = reportEnvelopeSchema.safeParse(
      envelope({
        ...completedPractice,
        type: "recruitment",
        summary: null,
        breakdown: null,
        questions: null,
      }),
    );
    expect(result.success).toBe(true);
  });

  it("accepts an in-progress report without a score", () => {
    const result = reportEnvelopeSchema.safeParse(
      envelope({
        ...completedPractice,
        status: "in_progress",
        overall_score: null,
        completed_at: null,
        summary: null,
        breakdown: null,
        questions: null,
      }),
    );
    expect(result.success).toBe(true);
  });

  it("rejects a score outside 0..100", () => {
    expect(
      reportEnvelopeSchema.safeParse(
        envelope({ ...completedPractice, overall_score: 101 }),
      ).success,
    ).toBe(false);
  });

  it("rejects an unknown status (contract drift)", () => {
    expect(
      reportEnvelopeSchema.safeParse(
        envelope({ ...completedPractice, status: "done" }),
      ).success,
    ).toBe(false);
  });

  it("rejects a missing field (contract drift)", () => {
    const withoutStart: Record<string, unknown> = { ...completedPractice };
    Reflect.deleteProperty(withoutStart, "started_at");
    expect(reportEnvelopeSchema.safeParse(envelope(withoutStart)).success).toBe(
      false,
    );
  });

  it("rejects a failed envelope", () => {
    expect(
      reportEnvelopeSchema.safeParse({
        success: false,
        error: { code: "INTERVIEW_NOT_FOUND", message: "missing" },
      }).success,
    ).toBe(false);
  });
});
