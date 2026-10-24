import { z } from "zod";
import {
  applicationSchema,
  applicationStatusSchema,
} from "./application-schema";

// Mirrors the Go Application Detail contract (KTD6, snake_case): the list item plus the posting
// block and a short-lived CV link. The CV storage key never leaves the API.
export const applicationDetailSchema = applicationSchema.extend({
  job_posting: z.object({
    id: z.uuid(),
    title: z.string(),
    status: z.string(),
    interview_slot: z.number().int().nullable(),
    cv_approvals: z.number().int(),
  }),
  cv: z
    .object({
      url: z.url(),
      expires_at: z.iso.datetime({ offset: true }),
    })
    .nullable(),
});
export type ApplicationDetail = z.infer<typeof applicationDetailSchema>;

export const applicationDetailEnvelopeSchema = z.object({
  success: z.literal(true),
  data: applicationDetailSchema,
});

// Response of approve-cv and reject-cv: the Application's new state plus its posting's status.
export const screeningSchema = z.object({
  id: z.uuid(),
  job_posting_id: z.uuid(),
  status: applicationStatusSchema,
  interview_deadline: z.iso.datetime({ offset: true }).nullable(),
  reject_reason: z.string().nullable(),
  updated_at: z.iso.datetime({ offset: true }),
  job_posting_status: z.string(),
});
export type Screening = z.infer<typeof screeningSchema>;

export const screeningEnvelopeSchema = z.object({
  success: z.literal(true),
  data: screeningSchema,
});

// The reject reason is optional and capped at 500 characters, counted after trimming.
export const MAX_SCREENING_REASON = 500;
export const screeningRejectSchema = z.object({
  reject_reason: z
    .string()
    .refine(
      (value) => Array.from(value.trim()).length <= MAX_SCREENING_REASON,
      `Keep the reason to ${MAX_SCREENING_REASON} characters or fewer.`,
    ),
});
export type ScreeningRejectValues = z.infer<typeof screeningRejectSchema>;
