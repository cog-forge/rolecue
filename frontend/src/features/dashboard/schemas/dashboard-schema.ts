import { z } from "zod";

// Wire schemas for the dashboard cards. They mirror the Go responses exactly. Role Profiles use
// camelCase (`GET /role-profiles`); interview history uses snake_case (`GET /interviews`).
// The dashboard never reads questions, turns, feedback text or record fields.

export const roleProfileStatusSchema = z.enum([
  "draft",
  "analyzing",
  "review",
  "confirmed",
]);
export type RoleProfileStatus = z.infer<typeof roleProfileStatusSchema>;

export const roleProfileSummarySchema = z.object({
  id: z.uuid(),
  title: z.string(),
  seniorityLevel: z
    .enum(["intern", "junior", "mid", "senior", "lead"])
    .nullable(),
  status: roleProfileStatusSchema,
  createdAt: z.iso.datetime({ offset: true }),
});
export type RoleProfileSummary = z.infer<typeof roleProfileSummarySchema>;

export const roleProfilePageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(roleProfileSummarySchema),
    pagination: z.object({
      limit: z.number().int(),
      offset: z.number().int(),
      total: z.number().int(),
    }),
  }),
});

export const interviewTypeSchema = z.enum(["practice", "recruitment"]);

// Same derivation as the Report status: in_progress until completed_at, then the evaluation status.
export const interviewStatusSchema = z.enum([
  "in_progress",
  "pending",
  "completed",
  "failed",
]);
export type InterviewStatus = z.infer<typeof interviewStatusSchema>;

export const interviewHistoryItemSchema = z.object({
  id: z.uuid(),
  type: interviewTypeSchema,
  started_at: z.iso.datetime({ offset: true }),
  completed_at: z.iso.datetime({ offset: true }).nullable(),
  evaluation_status: z.enum(["pending", "completed", "failed"]),
  status: interviewStatusSchema,
  score: z.number().int().min(0).max(100).nullable(),
  role_profile_title: z.string(),
  job_posting_title: z.string().nullable(),
  application_id: z.uuid().nullable(),
});
export type InterviewHistoryItem = z.infer<typeof interviewHistoryItemSchema>;

export const interviewHistoryPageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(interviewHistoryItemSchema),
    total: z.number().int(),
    page: z.number().int(),
    page_size: z.number().int(),
  }),
});
