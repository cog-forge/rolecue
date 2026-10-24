import { describe, expect, it } from "vitest";
import { applicationPageEnvelopeSchema } from "@/features/recruiter/applications/schemas/application-schema";

const item = {
  id: "44444444-4444-4444-8444-444444444444",
  job_posting_id: "33333333-3333-4333-8333-333333333333",
  job_posting_title: "Backend Engineer",
  status: "interview_eligible",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-555555555555",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: true,
  interview_deadline: "2026-10-21T08:00:00Z",
  reject_reason: null,
  updated_at: "2026-10-20T09:00:00Z",
};
const page = (items: unknown[]) => ({
  success: true,
  data: { items, pagination: { limit: 20, offset: 0, total: items.length } },
});

describe("application schema", () => {
  it("accepts the Go list contract", () => {
    const parsed = applicationPageEnvelopeSchema.parse(page([item]));
    expect(parsed.data.items[0].candidate.name).toBe("Linh Tran");
    expect(parsed.data.pagination.total).toBe(1);
  });

  it("accepts nullable fields and an empty page", () => {
    const pending = {
      ...item,
      status: "pending",
      interview_eligible: false,
      interview_deadline: null,
    };
    expect(
      applicationPageEnvelopeSchema.parse(page([pending])).data.items,
    ).toHaveLength(1);
    expect(applicationPageEnvelopeSchema.parse(page([])).data.items).toEqual(
      [],
    );
  });

  it("rejects contract drift", () => {
    expect(() =>
      applicationPageEnvelopeSchema.parse(page([{ ...item, status: "hired" }])),
    ).toThrow();
    expect(() =>
      applicationPageEnvelopeSchema.parse(
        page([{ ...item, candidate: { ...item.candidate, email: undefined } }]),
      ),
    ).toThrow();
    expect(() =>
      applicationPageEnvelopeSchema.parse({ success: false, data: null }),
    ).toThrow();
  });
});
