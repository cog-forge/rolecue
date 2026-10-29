import { z } from "zod";

// The reporting windows the Go endpoint accepts; any other value is rejected by the API.
export const overviewDays = [7, 30, 90] as const;
export type OverviewDays = (typeof overviewDays)[number];

const count = z.number().int();
const timestamp = z.iso.datetime({ offset: true });

export const overviewSchema = z.object({
  generated_at: timestamp,
  window: z.object({
    days: z.number().int(),
    from: timestamp,
    to: timestamp,
  }),
  accounts: z.object({
    candidates: count,
    recruiters: count,
    locked: count,
    new_in_window: count,
  }),
  job_postings: z.object({
    pending: count,
    open: count,
    intake_closed: count,
    closed: count,
    rejected: count,
  }),
  applications: z.object({
    pending: count,
    interview_eligible: count,
    interviewed: count,
    approved: count,
    rejected: count,
    submitted_in_window: count,
  }),
  interviews: z.object({
    practice_started_in_window: count,
    recruitment_started_in_window: count,
    completed_in_window: count,
    evaluation_failed: count,
  }),
  finance: z.object({
    revenue_vnd: z.number(),
    successful_deposits: count,
    coins_sold: count,
    coins_spent: count,
    coins_refunded: count,
    unique_payers: count,
    success_rate: z.number(),
  }),
});

export type Overview = z.infer<typeof overviewSchema>;

export const overviewEnvelopeSchema = z.object({
  success: z.literal(true),
  data: overviewSchema,
});
