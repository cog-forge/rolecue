import { z } from "zod";

// The reporting windows the Go endpoint accepts; any other value is rejected by the API.
export const overviewDays = [7, 30, 90] as const;
export type OverviewDays = (typeof overviewDays)[number];

const count = z.number().int();
const timestamp = z.iso.datetime({ offset: true });
const windowSchema = z.object({
  days: z.number().int(),
  from: timestamp,
  to: timestamp,
});

export const overviewSchema = z.object({
  generated_at: timestamp,
  window: windowSchema,
  // Older API deployments can still supply the current overview during rollout.
  previous_window: windowSchema.optional(),
  accounts: z.object({
    candidates: count,
    recruiters: count,
    locked: count,
    new_in_window: count,
    new_in_previous_window: count.optional(),
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
    submitted_in_previous_window: count.optional(),
  }),
  interviews: z.object({
    practice_started_in_window: count,
    recruitment_started_in_window: count,
    completed_in_window: count,
    evaluation_failed: count,
  }),
  interview_activity: z
    .object({
      bucket_days: z.union([z.literal(1), z.literal(7)]),
      buckets: z
        .array(
          z.object({
            practice: count.nonnegative(),
            recruitment: count.nonnegative(),
            previous_practice: count.nonnegative(),
            previous_recruitment: count.nonnegative(),
          }),
        )
        .nonempty(),
    })
    .optional(),
  finance: z.object({
    revenue_vnd: z.number(),
    previous_revenue_vnd: z.number().optional(),
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
