import { describe, expect, it } from "vitest";
import {
  overviewDays,
  overviewEnvelopeSchema,
} from "@/features/admin/overview/schemas/overview-schema";

const data = {
  generated_at: "2026-10-30T08:00:00+07:00",
  window: {
    days: 30,
    from: "2026-10-01T00:00:00+07:00",
    to: "2026-10-30T08:00:00+07:00",
  },
  accounts: { candidates: 10, recruiters: 2, locked: 1, new_in_window: 3 },
  job_postings: { pending: 1, open: 2, intake_closed: 0, closed: 0, rejected: 1 },
  applications: {
    pending: 4,
    interview_eligible: 2,
    interviewed: 1,
    approved: 0,
    rejected: 1,
    submitted_in_window: 5,
  },
  interviews: {
    practice_started_in_window: 6,
    recruitment_started_in_window: 2,
    completed_in_window: 3,
    evaluation_failed: 0,
  },
  finance: {
    revenue_vnd: 100000,
    successful_deposits: 2,
    coins_sold: 1500,
    coins_spent: 300,
    coins_refunded: 0,
    unique_payers: 2,
    success_rate: 66.7,
  },
};

describe("overviewEnvelopeSchema", () => {
  it("accepts the contract envelope", () => {
    const parsed = overviewEnvelopeSchema.parse({ success: true, data });
    expect(parsed.data.finance.revenue_vnd).toBe(100000);
  });

  it("rejects a missing section", () => {
    const { finance: _finance, ...rest } = data;
    expect(() =>
      overviewEnvelopeSchema.parse({ success: true, data: rest }),
    ).toThrow();
  });

  it("rejects a count sent as a string", () => {
    expect(() =>
      overviewEnvelopeSchema.parse({
        success: true,
        data: { ...data, accounts: { ...data.accounts, candidates: "10" } },
      }),
    ).toThrow();
  });

  it("rejects a fractional count", () => {
    expect(() =>
      overviewEnvelopeSchema.parse({
        success: true,
        data: { ...data, applications: { ...data.applications, pending: 1.5 } },
      }),
    ).toThrow();
  });

  it("rejects a timestamp without an offset", () => {
    expect(() =>
      overviewEnvelopeSchema.parse({
        success: true,
        data: { ...data, generated_at: "2026-10-30T08:00:00" },
      }),
    ).toThrow();
  });

  it("rejects an unsuccessful envelope", () => {
    expect(() =>
      overviewEnvelopeSchema.parse({ success: false, data }),
    ).toThrow();
  });

  it("lists the three supported windows", () => {
    expect(overviewDays).toEqual([7, 30, 90]);
  });
});
