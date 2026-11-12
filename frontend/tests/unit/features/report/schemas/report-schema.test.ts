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
  end_reason: "completed",
  question_count: 1,
  summary: "Clear structure, thin examples.",
  scoring: {
    method: "rubric",
    outcome: "complete",
    withheld_reason: null,
    assigned_questions: 1,
    graded_questions: 1,
    weight_total: 1,
    decimals: 2,
  },
  breakdown: [
    {
      metric_id: METRIC,
      name: "Communication",
      score: 72.5,
      weight: 1,
      question_count: 1,
    },
  ],
  graded_questions: [
    {
      question_number: 1,
      status: "graded",
      rubric_name: null,
      total: 72.5,
      earned: 29,
      max: 40,
      metrics: [
        {
          metric_id: METRIC,
          name: "Communication",
          earned: 29,
          max: 40,
          score: 72.5,
        },
      ],
      criteria: [
        {
          criterion_id: "66666666-6666-4666-8666-666666666666",
          name: "",
          metric_id: METRIC,
          metric_name: "Communication",
          points: 29,
          max_points: 40,
          evidence: "built an API",
          evidence_position: 1,
          feedback: null,
        },
      ],
    },
  ],
  questions: [
    {
      position: 1,
      question_number: 1,
      kind: "core",
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

  it("reads the end reason, and treats a report without one as null", () => {
    const left = reportEnvelopeSchema.parse(
      envelope({ ...completedPractice, end_reason: "left", question_count: 4 }),
    );
    expect(left.data).toMatchObject({ end_reason: "left", question_count: 4 });
    const { end_reason: _omitted, ...older } = completedPractice;
    void _omitted;
    expect(
      reportEnvelopeSchema.parse(envelope(older)).data.end_reason,
    ).toBeNull();
    expect(
      reportEnvelopeSchema.safeParse(
        envelope({ ...completedPractice, end_reason: "paused" }),
      ).success,
    ).toBe(false);
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

  it("keeps two-decimal scores and reads a withheld overall (KAN-335)", () => {
    const parsed = reportEnvelopeSchema.parse(
      envelope({
        ...completedPractice,
        overall_score: null,
        scoring: {
          ...completedPractice.scoring,
          outcome: "withheld",
          withheld_reason: "missing_metric_coverage",
        },
        breakdown: [{ ...completedPractice.breakdown![0], score: null }],
      }),
    );
    expect(parsed.data.overall_score).toBeNull();
    expect(parsed.data.scoring?.withheld_reason).toBe(
      "missing_metric_coverage",
    );
    expect(parsed.data.breakdown?.[0].score).toBeNull();
    expect(
      reportEnvelopeSchema.parse(envelope(completedPractice)).data
        .graded_questions?.[0].total,
    ).toBe(72.5);
  });

  it("rejects an unknown withheld reason or a criterion without a maximum (contract drift)", () => {
    const graded = completedPractice.graded_questions![0];
    for (const data of [
      {
        ...completedPractice,
        scoring: { ...completedPractice.scoring, withheld_reason: "guess" },
      },
      {
        ...completedPractice,
        graded_questions: [
          { ...graded, criteria: [{ ...graded.criteria[0], max_points: 0 }] },
        ],
      },
    ])
      expect(reportEnvelopeSchema.safeParse(envelope(data)).success).toBe(
        false,
      );
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

describe("recruitment Candidate report without overall_score (KAN-253)", () => {
  it("reads a missing overall_score as null", () => {
    const parsed = reportEnvelopeSchema.parse({
      success: true,
      data: {
        ...completedPractice,
        overall_score: undefined,
        type: "recruitment",
        summary: null,
        breakdown: null,
        questions: null,
      },
    });
    expect(parsed.data.overall_score).toBeNull();
  });

  it("accepts the recruiter fields summary_source, injection_flagged and evidence", () => {
    const parsed = reportEnvelopeSchema.parse({
      success: true,
      data: {
        ...completedPractice,
        summary_source: "ai",
        injection_flagged: true,
        breakdown: [
          {
            metric_id: METRIC,
            name: "Communication",
            score: 80,
            evidence: "I built an API.",
          },
        ],
      },
    });
    expect(parsed.data.injection_flagged).toBe(true);
    expect(parsed.data.breakdown?.[0]?.evidence).toBe("I built an API.");
  });

  it("rejects a summary_source other than ai", () => {
    expect(
      reportEnvelopeSchema.safeParse({
        success: true,
        data: { ...completedPractice, summary_source: "human" },
      }).success,
    ).toBe(false);
  });
});
