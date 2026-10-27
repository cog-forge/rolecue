import { describe, expect, it } from "vitest";
import { applicationDetailEnvelopeSchema } from "@/features/recruiter/applications/schemas/application-detail-schema";
import {
  interviewReportEnvelopeSchema,
  recordingEnvelopeSchema,
  transcriptEnvelopeSchema,
} from "@/features/recruiter/applications/schemas/interview-evidence-schema";

const envelope = (data: unknown) => ({ success: true, data });

const report = {
  interview_id: "66666666-6666-4666-8666-666666666666",
  type: "recruitment",
  status: "completed",
  overall_score: 82,
  started_at: "2026-10-20T08:00:00Z",
  completed_at: "2026-10-20T08:30:00Z",
  summary: "Solid Go answers, weaker on testing.",
  breakdown: [
    {
      metric_id: "77777777-7777-4777-8777-777777777777",
      name: "Go depth",
      score: 90,
    },
  ],
  questions: [
    {
      position: 1,
      question: "Tell us about a service you built.",
      answer: "I built a queue worker.",
      feedback: "Clear ownership.",
    },
  ],
};

const detail = {
  id: "44444444-4444-4444-8444-444444444444",
  job_posting_id: "33333333-3333-4333-8333-333333333333",
  job_posting_title: "Backend Engineer",
  status: "interviewed",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-555555555555",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: "2026-10-21T08:00:00Z",
  reject_reason: null,
  updated_at: "2026-10-20T09:00:00Z",
  job_posting: {
    id: "33333333-3333-4333-8333-333333333333",
    title: "Backend Engineer",
    status: "open",
    interview_slot: 5,
    cv_approvals: 2,
  },
  cv: null,
};

describe("interview report schema", () => {
  it("accepts the owning Recruiter's full report", () => {
    const parsed = interviewReportEnvelopeSchema.parse(envelope(report));
    expect(parsed.data.breakdown?.[0]?.score).toBe(90);
  });

  it("accepts a pending report with no summary, breakdown or questions", () => {
    const parsed = interviewReportEnvelopeSchema.parse(
      envelope({
        ...report,
        status: "pending",
        overall_score: null,
        completed_at: null,
        summary: null,
        breakdown: null,
        questions: null,
      }),
    );
    expect(parsed.data.breakdown).toBeNull();
  });

  it("rejects a score outside 0..100", () => {
    expect(() =>
      interviewReportEnvelopeSchema.parse(
        envelope({
          ...report,
          breakdown: [{ ...report.breakdown[0], score: 101 }],
        }),
      ),
    ).toThrow();
  });

  it("rejects an unknown interview status", () => {
    expect(() =>
      interviewReportEnvelopeSchema.parse(
        envelope({ ...report, status: "done" }),
      ),
    ).toThrow();
  });
});

describe("recording and transcript schemas", () => {
  it("accepts a ready recording with a read link", () => {
    const parsed = recordingEnvelopeSchema.parse(
      envelope({
        record_status: "ready",
        url: "https://files.example.com/rec/linh.webm?signature=test",
        expires_at: "2026-10-24T10:05:00Z",
      }),
    );
    expect(parsed.data.url).toContain("signature=test");
  });

  it("refuses a read link that is not http or https", () => {
    for (const url of [
      "javascript:alert(1)",
      "data:text/html,x",
      "file:///x",
    ]) {
      expect(() =>
        recordingEnvelopeSchema.parse(
          envelope({
            record_status: "ready",
            url,
            expires_at: "2026-10-24T10:05:00Z",
          }),
        ),
      ).toThrow();
    }
  });

  it("accepts a recording that is not ready, with no link", () => {
    const parsed = recordingEnvelopeSchema.parse(
      envelope({ record_status: "uploading", url: null, expires_at: null }),
    );
    expect(parsed.data.url).toBeNull();
  });

  it("accepts a transcript turn with no answer", () => {
    const parsed = transcriptEnvelopeSchema.parse(
      envelope({
        turns: [{ position: 1, question: "How do you test it?", answer: null }],
      }),
    );
    expect(parsed.data.turns[0]?.answer).toBeNull();
  });

  it("rejects a malformed recording link", () => {
    expect(() =>
      recordingEnvelopeSchema.parse(
        envelope({
          record_status: "ready",
          url: "not a url",
          expires_at: null,
        }),
      ),
    ).toThrow();
  });
});

describe("application detail interview and final review fields", () => {
  it("accepts the interview summary and the final review gate", () => {
    const parsed = applicationDetailEnvelopeSchema.parse(
      envelope({
        ...detail,
        interview: {
          id: "66666666-6666-4666-8666-666666666666",
          started_at: "2026-10-20T08:00:00Z",
          completed_at: "2026-10-20T08:30:00Z",
          status: "completed",
          score: 82,
          record_status: "ready",
        },
        final_review: { opens_at: "2026-10-21T08:00:00Z", can_decide: true },
      }),
    );
    expect(parsed.data.interview?.score).toBe(82);
    expect(parsed.data.final_review?.can_decide).toBe(true);
  });

  it("accepts a null interview and a null final review", () => {
    const parsed = applicationDetailEnvelopeSchema.parse(
      envelope({ ...detail, interview: null, final_review: null }),
    );
    expect(parsed.data.interview).toBeNull();
    expect(parsed.data.final_review).toBeNull();
  });

  it("still parses a detail without the new fields", () => {
    const parsed = applicationDetailEnvelopeSchema.parse(envelope(detail));
    expect(parsed.data.interview).toBeUndefined();
    expect(parsed.data.final_review).toBeUndefined();
  });

  it("rejects an interview with an unknown record status", () => {
    expect(() =>
      applicationDetailEnvelopeSchema.parse(
        envelope({
          ...detail,
          interview: {
            id: "66666666-6666-4666-8666-666666666666",
            started_at: "2026-10-20T08:00:00Z",
            completed_at: null,
            status: "in_progress",
            score: null,
            record_status: "lost",
          },
        }),
      ),
    ).toThrow();
  });
});
