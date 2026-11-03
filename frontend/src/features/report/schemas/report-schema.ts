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

// `position` is the turn's place in the interview; `question_number` the core question it belongs to,
// which a follow-up (`kind: "follow_up"`) shares with its core question.
export const reportQuestionSchema = z.object({
  position: z.number().int().min(1),
  question_number: z.number().int().min(1),
  kind: z.enum(["core", "follow_up"]),
  question: z.string(),
  answer: z.string().nullable(),
  feedback: z.string().nullable(),
});

// How the interview ended (KAN-215): null while it runs and for interviews ended before it was
// recorded. `question_count` is the number of core questions; `questions` lists only answered ones.
export const endReasonSchema = z.enum(["completed", "left", "abandoned"]);
export type EndReason = z.infer<typeof endReasonSchema>;

export const reportSchema = z.object({
  interview_id: z.uuid(),
  type: reportTypeSchema,
  status: reportStatusSchema,
  overall_score: scoreSchema.nullable(),
  started_at: z.iso.datetime({ offset: true }),
  completed_at: z.iso.datetime({ offset: true }).nullable(),
  end_reason: endReasonSchema.nullish().transform((value) => value ?? null),
  question_count: z.number().int().nonnegative().optional(),
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
