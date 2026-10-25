import { z } from "zod";
import {
  interviewStatusSchema,
  recordStatusSchema,
} from "./application-detail-schema";

// GET /interviews/:id/report for the owning Recruiter (evidence): every field is present. The
// Candidate's view of the same endpoint carries only the score and status, and is not read here.
export const interviewReportSchema = z.object({
  interview_id: z.uuid(),
  type: z.enum(["practice", "recruitment"]),
  status: interviewStatusSchema,
  overall_score: z.number().int().min(0).max(100).nullable(),
  started_at: z.iso.datetime({ offset: true }),
  completed_at: z.iso.datetime({ offset: true }).nullable(),
  summary: z.string().nullable(),
  breakdown: z
    .array(
      z.object({
        metric_id: z.uuid(),
        name: z.string(),
        score: z.number().int().min(0).max(100),
      }),
    )
    .nullable(),
  questions: z
    .array(
      z.object({
        position: z.number().int(),
        question: z.string(),
        answer: z.string().nullable(),
        feedback: z.string().nullable(),
      }),
    )
    .nullable(),
});
export type InterviewReport = z.infer<typeof interviewReportSchema>;

export const interviewReportEnvelopeSchema = z.object({
  success: z.literal(true),
  data: interviewReportSchema,
});

// GET .../recording. The url is a short-lived read link, present only when the record is ready.
// Retention, consent and Admin access to recordings are open product gates; no policy is encoded here.
export const recordingSchema = z.object({
  record_status: recordStatusSchema,
  url: z.url().nullable(),
  expires_at: z.iso.datetime({ offset: true }).nullable(),
});
export type Recording = z.infer<typeof recordingSchema>;

export const recordingEnvelopeSchema = z.object({
  success: z.literal(true),
  data: recordingSchema,
});

// GET .../transcript: one entry per question of a completed recruitment interview.
export const transcriptSchema = z.object({
  turns: z.array(
    z.object({
      position: z.number().int(),
      question: z.string(),
      answer: z.string().nullable(),
    }),
  ),
});
export type Transcript = z.infer<typeof transcriptSchema>;

export const transcriptEnvelopeSchema = z.object({
  success: z.literal(true),
  data: transcriptSchema,
});
