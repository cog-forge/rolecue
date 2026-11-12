import type { useTranslations } from "next-intl";
import { z } from "zod";

// Mirrors the Go KAN-79 contract (snake_case, like every new feature).
export const postingStatusSchema = z.enum([
  "pending",
  "open",
  "intake_closed",
  "closed",
  "rejected",
]);
export type PostingStatus = z.infer<typeof postingStatusSchema>;

export const coreQuestionSchema = z.object({
  id: z.uuid(),
  content: z.string(),
});
export type CoreQuestion = z.infer<typeof coreQuestionSchema>;

export const bankQuestionSchema = coreQuestionSchema.extend({
  rubric_id: z.uuid().nullable().default(null),
  expected_answer: z.string().nullable().default(null),
  grading_incomplete: z.boolean().default(true),
  grading_issues: z
    .array(
      z.enum([
        "missing_rubric",
        "rubric_archived",
        "rubric_unavailable",
        "missing_expected_answer",
        "expected_answer_invalid",
      ]),
    )
    .default(["missing_rubric", "missing_expected_answer"]),
  revision: z.string().default(""),
});
export type BankQuestion = z.infer<typeof bankQuestionSchema>;

export const bankSchema = z.object({
  questions: z.array(bankQuestionSchema),
  grading_ready: z.boolean().default(false),
  grading_incomplete_count: z.number().int().nonnegative().optional(),
  max_expected_answer_length: z.number().int().positive().default(1000),
  max_expected_answer_words: z.number().int().positive().default(100),
  expected_answer_guidance_words: z.number().int().positive().default(80),
  max_bulk_questions: z.number().int().positive().default(100),
  posting_status: postingStatusSchema,
  editable: z.boolean(),
  max_question_length: z.number().int().positive(),
  generation_size: z.number().int().nonnegative(),
  // True once the Recruiter sent the posting to the Admin queue.
  review_submitted: z.boolean(),
});
export type Bank = z.infer<typeof bankSchema>;

const envelope = <T extends z.ZodType>(data: T) =>
  z.object({ success: z.literal(true), data });

export const bankEnvelopeSchema = envelope(bankSchema);
export const generatedEnvelopeSchema = envelope(
  z.object({ questions: z.array(bankQuestionSchema) }),
);
export const questionEnvelopeSchema = envelope(bankQuestionSchema);

// Go counts runes, so the limit counts Unicode code points, after trimming like the server does.
export const questionLength = (value: string) =>
  Array.from(value.trim()).length;

// Translator for the question form's messages; build the schema with
// `useMemo(() => createQuestionFormSchema(max, v), [max, v])`.
export type QuestionValidation = ReturnType<
  typeof useTranslations<"recruiter.questionBank.validation">
>;

export const createQuestionFormSchema = (max: number, t: QuestionValidation) =>
  z.object({
    content: z
      .string()
      .refine((value) => questionLength(value) > 0, t("required"))
      .refine((value) => questionLength(value) <= max, t("max", { max })),
  });
export type QuestionValues = z.infer<
  ReturnType<typeof createQuestionFormSchema>
>;
