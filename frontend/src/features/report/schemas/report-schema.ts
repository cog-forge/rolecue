import { z } from "zod";

// Mirrors GET /interviews/:id/report (KAN-63, snake_case). For a recruitment interview the
// Candidate receives only the score and status: summary, breakdown and questions are null.
export const reportTypes = ["practice", "recruitment"] as const;
export const reportStatuses = [
  "in_progress",
  "pending",
  "completed",
  "failed",
] as const;

export const reportTypeSchema = z.enum(reportTypes);
export const reportStatusSchema = z.enum(reportStatuses);
export type ReportType = z.infer<typeof reportTypeSchema>;
export type ReportStatus = z.infer<typeof reportStatusSchema>;

const scoreSchema = z.number().int().min(0).max(100);

export const reportMetricSchema = z.object({
  metric_id: z.uuid(),
  name: z.string(),
  score: scoreSchema,
});

export const reportQuestionSchema = z.object({
  position: z.number().int().min(1),
  question: z.string(),
  answer: z.string().nullable(),
  feedback: z.string().nullable(),
});

export const reportSchema = z.object({
  interview_id: z.uuid(),
  type: reportTypeSchema,
  status: reportStatusSchema,
  overall_score: scoreSchema.nullable(),
  started_at: z.iso.datetime({ offset: true }),
  completed_at: z.iso.datetime({ offset: true }).nullable(),
  summary: z.string().nullable(),
  breakdown: z.array(reportMetricSchema).nullable(),
  questions: z.array(reportQuestionSchema).nullable(),
});
export type Report = z.infer<typeof reportSchema>;
export type ReportMetric = z.infer<typeof reportMetricSchema>;
export type ReportQuestion = z.infer<typeof reportQuestionSchema>;

export const reportEnvelopeSchema = z.object({
  success: z.literal(true),
  data: reportSchema,
});
