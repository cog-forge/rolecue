import { describe, expect, it } from "vitest";
import {
  postingPageEnvelopeSchema,
  rejectFormSchema,
  reviewPostingEnvelopeSchema,
} from "@/features/admin/moderation/schemas/moderation-schema";

const recruiter = {
  id: "22222222-2222-4222-8222-222222222222",
  name: "Rec Ruiter",
  email: "rec@example.com",
};
const review = {
  id: "33333333-3333-4333-8333-333333333333",
  title: "Go Backend Engineer",
  status: "pending",
  reject_reason: null,
  recruiter,
  location: null,
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_slot: null,
  interview_duration: null,
  required_avatar: null,
  required_voice: { id: "44444444-4444-4444-8444-444444444444", name: "Ava" },
  created_at: "2026-10-12T08:00:00Z",
  content_path: "https://files.example.com/jd/go.pdf",
  document_url: null,
  document_expires_at: null,
  raw_text: null,
  core_question_count: 50,
};

describe("moderation wire schemas", () => {
  it("parses the queue page and the review posting", () => {
    expect(
      postingPageEnvelopeSchema.safeParse({
        success: true,
        data: {
          items: [
            {
              id: review.id,
              title: review.title,
              status: "pending",
              recruiter,
              created_at: "2026-10-12T08:00:00.123456789Z",
            },
          ],
          pagination: { limit: 20, offset: 0, total: 1 },
        },
      }).success,
    ).toBe(true);
    expect(
      reviewPostingEnvelopeSchema.safeParse({ success: true, data: review })
        .success,
    ).toBe(true);
    expect(
      reviewPostingEnvelopeSchema.safeParse({
        success: true,
        data: {
          ...review,
          content_path: "job-descriptions/a.txt",
          raw_text: "JD text",
        },
      }).success,
    ).toBe(true);
    expect(
      reviewPostingEnvelopeSchema.safeParse({
        success: true,
        data: {
          ...review,
          content_path: "0b6c3a52-6a3d-4b7e-9d55-8c1f2a3b4c5d",
          document_url:
            "https://storage.example.com/role-cue/key?X-Amz-Signature=s",
          document_expires_at: "2026-10-17T08:15:00Z",
          raw_text: "JD text",
        },
      }).success,
    ).toBe(true);
    // Only a web link is embedded.
    expect(
      reviewPostingEnvelopeSchema.safeParse({
        success: true,
        data: { ...review, document_url: "javascript:alert(1)" },
      }).success,
    ).toBe(false);
  });

  it("rejects contract drift", () => {
    expect(
      reviewPostingEnvelopeSchema.safeParse({
        success: true,
        data: { ...review, status: "archived" },
      }).success,
    ).toBe(false);
    const { content_path: _unused, ...withoutPath } = review;
    void _unused;
    expect(
      reviewPostingEnvelopeSchema.safeParse({
        success: true,
        data: withoutPath,
      }).success,
    ).toBe(false);
    const { raw_text: _noText, ...withoutText } = review;
    void _noText;
    expect(
      reviewPostingEnvelopeSchema.safeParse({
        success: true,
        data: withoutText,
      }).success,
    ).toBe(false);
    expect(
      reviewPostingEnvelopeSchema.safeParse({ success: false, data: review })
        .success,
    ).toBe(false);
  });
});

describe("reject form", () => {
  const ok = (reason: string) =>
    rejectFormSchema.safeParse({ reject_reason: reason }).success;

  it("requires a non-blank reason", () => {
    expect(ok("")).toBe(false);
    expect(ok("   ")).toBe(false);
    expect(ok("Salary range is missing")).toBe(true);
  });

  it("counts Unicode code points like Go, after trimming", () => {
    expect(ok("界".repeat(1000))).toBe(true);
    expect(ok(` ${"界".repeat(1000)} `)).toBe(true);
    expect(ok("界".repeat(1001))).toBe(false);
  });

  it("rejects NUL characters", () => {
    expect(ok("bad\u0000reason")).toBe(false);
  });
});
