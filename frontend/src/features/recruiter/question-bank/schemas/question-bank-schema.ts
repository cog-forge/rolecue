import { z } from "zod";
import { defaultLocale } from "@/lib/i18n/config";
import { formatNumber } from "@/lib/i18n/formatters";

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

export const bankSchema = z.object({
  questions: z.array(coreQuestionSchema),
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
  z.object({ questions: z.array(coreQuestionSchema) }),
);
export const questionEnvelopeSchema = envelope(coreQuestionSchema);

// Go counts runes, so the limit counts Unicode code points, after trimming like the server does.
export const questionLength = (value: string) =>
  Array.from(value.trim()).length;

export const questionFormSchema = (max: number) =>
  z.object({
    content: z
      .string()
      .refine((value) => questionLength(value) > 0, "Write the question first.")
      .refine(
        (value) => questionLength(value) <= max,
        `Keep the question to ${formatNumber(max, defaultLocale)} characters or fewer.`,
      ),
  });
export type QuestionValues = z.infer<ReturnType<typeof questionFormSchema>>;
