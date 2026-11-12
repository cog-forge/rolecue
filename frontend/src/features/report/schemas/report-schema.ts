import { z } from "zod";

// Mirrors GET /interviews/:id/report (KAN-63, KAN-336, snake_case). For a recruitment interview the
// Candidate receives only the status: summary, scoring, breakdown, questions and graded_questions are
// null. Scores keep two decimals (KAN-335).
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

// A 0..100 score with at most two decimals, as the server stores it.
export const scoreSchema = z.number().min(0).max(100);

export const scoreOutcomes = ["complete", "partial", "withheld"] as const;
export const scoreOutcomeSchema = z.enum(scoreOutcomes);
export type ScoreOutcome = z.infer<typeof scoreOutcomeSchema>;

export const withheldReasons = [
  "missing_metric_coverage",
  "partial_session",
  "no_answers",
  "no_gradable_questions",
] as const;
export const withheldReasonSchema = z.enum(withheldReasons);
export type WithheldReason = z.infer<typeof withheldReasonSchema>;

// How the overall was calculated: criterion points -> question metric -> session metric -> weighted
// overall. method "legacy" is an evaluation made before rubric grading (no criterion results).
export const reportScoringSchema = z.object({
  method: z.enum(["rubric", "legacy"]),
  outcome: scoreOutcomeSchema.nullable(),
  withheld_reason: withheldReasonSchema.nullable(),
  assigned_questions: z.number().int().nonnegative(),
  graded_questions: z.number().int().nonnegative(),
  weight_total: z.number().nonnegative(),
  decimals: z.number().int().nonnegative(),
});
export type ReportScoring = z.infer<typeof reportScoringSchema>;

// One session metric. score is null for a weighted metric no graded question assessed (never a zero).
// weight is the weight the overall used (0: not weighted); null for a legacy evaluation.
export const reportMetricSchema = z.object({
  metric_id: z.uuid(),
  name: z.string(),
  score: scoreSchema.nullable(),
  weight: z.number().nonnegative().nullish(),
  question_count: z.number().int().nonnegative().nullish(),
  // Legacy metric quote only; rubric evidence is per criterion.
  evidence: z.string().nullish(),
});

export const reportCriterionSchema = z.object({
  criterion_id: z.uuid(),
  // Empty for a Candidate when the rubric is not a system template.
  name: z.string(),
  metric_id: z.uuid(),
  metric_name: z.string(),
  points: z.number().int().nonnegative(),
  max_points: z.number().int().positive(),
  evidence: z.string().nullable(),
  evidence_position: z.number().int().nullable(),
  feedback: z.string().nullable(),
});

export const reportQuestionMetricSchema = z.object({
  metric_id: z.uuid(),
  name: z.string(),
  earned: z.number().int().nonnegative(),
  max: z.number().int().nonnegative(),
  score: scoreSchema,
});

// One assigned core question. total is for reading only: the overall never averages question totals.
export const gradedQuestionSchema = z.object({
  question_number: z.number().int().min(1),
  status: z.enum(["graded", "not_answered", "not_gradable"]),
  rubric_name: z.string().nullable(),
  total: scoreSchema.nullable(),
  earned: z.number().int().nonnegative(),
  max: z.number().int().nonnegative(),
  metrics: z.array(reportQuestionMetricSchema),
  criteria: z.array(reportCriterionSchema),
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
  // Omitted for a recruitment Candidate (the Recruiter announces the result) and null when withheld.
  overall_score: scoreSchema.nullish().transform((value) => value ?? null),
  started_at: z.iso.datetime({ offset: true }),
  completed_at: z.iso.datetime({ offset: true }).nullable(),
  end_reason: endReasonSchema.nullish().transform((value) => value ?? null),
  question_count: z.number().int().nonnegative().optional(),
  summary: z.string().nullable(),
  summary_source: z.literal("ai").nullish(),
  injection_flagged: z.boolean().optional(),
  scoring: reportScoringSchema.nullish().transform((value) => value ?? null),
  breakdown: z.array(reportMetricSchema).nullable(),
  questions: z.array(reportQuestionSchema).nullable(),
  graded_questions: z
    .array(gradedQuestionSchema)
    .nullish()
    .transform((value) => value ?? null),
});
export type Report = z.infer<typeof reportSchema>;
export type ReportMetric = z.infer<typeof reportMetricSchema>;
export type ReportQuestion = z.infer<typeof reportQuestionSchema>;
export type GradedQuestion = z.infer<typeof gradedQuestionSchema>;
export type ReportCriterion = z.infer<typeof reportCriterionSchema>;

export const reportEnvelopeSchema = z.object({
  success: z.literal(true),
  data: reportSchema,
});
