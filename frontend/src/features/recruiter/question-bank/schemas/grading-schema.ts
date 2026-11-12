import { z } from "zod";
export const answerWordCount = (text: string) =>
  text.trim() ? text.trim().split(/\s+/u).length : 0;
export function createGradingSchema(
  maxCharacters: number,
  maxWords: number,
  invalid: string,
) {
  return z.object({
    rubric_id: z.string(),
    expected_answer: z
      .string()
      .refine(
        (text) =>
          Array.from(text.trim()).length <= maxCharacters &&
          answerWordCount(text) <= maxWords,
        invalid,
      ),
    reviewed: z.boolean(),
  });
}
export type GradingValues = z.infer<ReturnType<typeof createGradingSchema>>;
