import { z } from "zod";

// Mirrors the Go KAN-172 Recruiter Application list contract (snake_case). The list carries only
// what Application Detail needs: never the CV file, transcript or recording.
export const applicationStatuses = [
  "pending",
  "interview_eligible",
  "interviewed",
  "approved",
  "rejected",
] as const;
export const applicationStatusSchema = z.enum(applicationStatuses);
export type ApplicationStatus = z.infer<typeof applicationStatusSchema>;

export const applicationSchema = z.object({
  id: z.uuid(),
  job_posting_id: z.uuid(),
  job_posting_title: z.string(),
  status: applicationStatusSchema,
  submitted_at: z.iso.datetime({ offset: true }),
  candidate: z.object({
    id: z.uuid(),
    name: z.string(),
    email: z.string(),
    image: z.string().nullable(),
  }),
  interview_eligible: z.boolean(),
  interview_deadline: z.iso.datetime({ offset: true }).nullable(),
  reject_reason: z.string().nullable(),
  updated_at: z.iso.datetime({ offset: true }),
});
export type Application = z.infer<typeof applicationSchema>;

export const applicationPageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(applicationSchema),
    pagination: z.object({
      limit: z.number().int(),
      offset: z.number().int(),
      total: z.number().int(),
    }),
  }),
});
export type ApplicationPage = z.infer<
  typeof applicationPageEnvelopeSchema
>["data"];
