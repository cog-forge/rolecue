import { z } from "zod";
import { reportSchema } from "@/features/report/schemas/report-schema";
import { recordStatusSchema } from "./application-detail-schema";

// GET /interviews/:id/report for the owning Recruiter (evidence): every field is present, including the
// rubric score explanation (KAN-336). It is the same contract the Candidate's report reads.
export const interviewReportSchema = reportSchema;
export type InterviewReport = z.infer<typeof interviewReportSchema>;

export const interviewReportEnvelopeSchema = z.object({
  success: z.literal(true),
  data: interviewReportSchema,
});

// GET .../recording. The url is a short-lived read link, present only when the record is ready.
// Retention, consent and Admin access to recordings are open product gates; no policy is encoded here.
export const recordingSchema = z.object({
  record_status: recordStatusSchema,
  url: z.url({ protocol: /^https?$/ }).nullable(),
  expires_at: z.iso.datetime({ offset: true }).nullable(),
});
export type Recording = z.infer<typeof recordingSchema>;

export const recordingEnvelopeSchema = z.object({
  success: z.literal(true),
  data: recordingSchema,
});

// GET .../transcript: one entry per question of a completed recruitment interview. A follow-up
// (`kind: "follow_up"`) shares the `question_number` of the core question it probes.
export const transcriptSchema = z.object({
  turns: z.array(
    z.object({
      position: z.number().int(),
      question_number: z.number().int(),
      kind: z.enum(["core", "follow_up"]),
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
