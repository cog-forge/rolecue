import { describe, expect, it } from "vitest";
import {
  applicationDetailEnvelopeSchema,
  screeningEnvelopeSchema,
} from "@/features/recruiter/applications/schemas/application-detail-schema";

const detail = {
  id: "44444444-4444-4444-8444-444444444444",
  job_posting_id: "33333333-3333-4333-8333-333333333333",
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-555555555555",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T09:00:00Z",
  job_posting: {
    id: "33333333-3333-4333-8333-333333333333",
    title: "Backend Engineer",
    status: "open",
    interview_slot: 5,
    cv_approvals: 2,
  },
  cv: {
    url: "https://storage.example.com/cv/linh.pdf?X-Amz-Expires=300",
    expires_at: "2026-10-20T08:05:00Z",
  },
};
const envelope = (data: unknown) => ({ success: true, data });

const screening = {
  id: "44444444-4444-4444-8444-444444444444",
  job_posting_id: "33333333-3333-4333-8333-333333333333",
  status: "interview_eligible",
  interview_deadline: "2026-10-21T08:00:00Z",
  reject_reason: null,
  updated_at: "2026-10-20T09:00:00Z",
  job_posting_status: "open",
};

describe("application detail schema", () => {
  it("accepts the full detail payload", () => {
    const parsed = applicationDetailEnvelopeSchema.parse(envelope(detail));
    expect(parsed.data.job_posting.cv_approvals).toBe(2);
    expect(parsed.data.cv?.url).toBe(detail.cv.url);
  });

  it("accepts cv null and interview_slot null", () => {
    const parsed = applicationDetailEnvelopeSchema.parse(
      envelope({
        ...detail,
        cv: null,
        job_posting: { ...detail.job_posting, interview_slot: null },
      }),
    );
    expect(parsed.data.cv).toBeNull();
    expect(parsed.data.job_posting.interview_slot).toBeNull();
  });

  it("rejects a payload missing job_posting", () => {
    expect(() =>
      applicationDetailEnvelopeSchema.parse(
        envelope({ ...detail, job_posting: undefined }),
      ),
    ).toThrow();
  });

  it("rejects a malformed CV link", () => {
    expect(() =>
      applicationDetailEnvelopeSchema.parse(
        envelope({ ...detail, cv: { url: "not a url", expires_at: "x" } }),
      ),
    ).toThrow();
  });
});

describe("screening schema", () => {
  it("accepts the approve/reject response", () => {
    const parsed = screeningEnvelopeSchema.parse(envelope(screening));
    expect(parsed.data.job_posting_status).toBe("open");
  });

  it("accepts a rejected response with a reason and no deadline", () => {
    const parsed = screeningEnvelopeSchema.parse(
      envelope({
        ...screening,
        status: "rejected",
        interview_deadline: null,
        reject_reason: "Missing Go experience",
      }),
    );
    expect(parsed.data.reject_reason).toBe("Missing Go experience");
  });

  it("rejects a response without job_posting_status", () => {
    expect(() =>
      screeningEnvelopeSchema.parse(
        envelope({ ...screening, job_posting_status: undefined }),
      ),
    ).toThrow();
  });
});
