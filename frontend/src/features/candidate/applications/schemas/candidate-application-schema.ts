import { z } from "zod";

// Mirrors the Go Candidate Application contract (KAN-112, snake_case). It never carries the CV
// storage key, transcript, audio or video.
export const candidateApplicationStatuses = [
  "pending",
  "interview_eligible",
  "interviewed",
  "approved",
  "rejected",
] as const;
export const candidateApplicationStatusSchema = z.enum(
  candidateApplicationStatuses,
);
export type CandidateApplicationStatus = z.infer<
  typeof candidateApplicationStatusSchema
>;

export const candidateApplicationSchema = z.object({
  id: z.uuid(),
  job_posting_id: z.uuid(),
  job_posting_title: z.string(),
  status: candidateApplicationStatusSchema,
  reject_reason: z.string().nullable(),
  interview_deadline: z.iso.datetime({ offset: true }).nullable(),
  can_start_interview: z.boolean(),
  created_at: z.iso.datetime({ offset: true }),
});
export type CandidateApplication = z.infer<typeof candidateApplicationSchema>;

export const candidateApplicationPageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(candidateApplicationSchema),
    total: z.number().int(),
    page: z.number().int(),
    page_size: z.number().int(),
  }),
});
export type CandidateApplicationPage = z.infer<
  typeof candidateApplicationPageEnvelopeSchema
>["data"];

export const candidateApplicationEnvelopeSchema = z.object({
  success: z.literal(true),
  data: candidateApplicationSchema,
});

// Response of POST /job-board/:id/applications (201).
export const createdApplicationSchema = z.object({
  id: z.uuid(),
  job_posting_id: z.uuid(),
  status: candidateApplicationStatusSchema,
  created_at: z.iso.datetime({ offset: true }),
});
export type CreatedApplication = z.infer<typeof createdApplicationSchema>;

export const createdApplicationEnvelopeSchema = z.object({
  success: z.literal(true),
  data: createdApplicationSchema,
});

// Mirrors the backend limits for the CV: one PDF, at most 5 MB (same as the JD PDF).
export const CV_MAX_BYTES = 5 * 1024 * 1024;
export const CV_FILE_TYPES = "application/pdf,.pdf";

export const APPLICATION_PAGE_SIZE = 20;
