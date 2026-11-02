import type { useTranslations } from "next-intl";
import { z } from "zod";
import { interviewTypes } from "../api/interviews";

// Translator for the filter messages (`admin.interviews.validation`).
export type InterviewFilterValidation = ReturnType<
  typeof useTranslations<"admin.interviews.validation">
>;

export function createInterviewFilterSchema(t: InterviewFilterValidation) {
  const optionalId = z
    .string()
    .trim()
    .refine(
      (value) => value === "" || z.uuid().safeParse(value).success,
      t("invalidId"),
    );
  return z.object({
    type: z.enum(["all", ...interviewTypes]),
    candidateId: optionalId,
    jobPostingId: optionalId,
  });
}

export type InterviewFilterValues = z.infer<
  ReturnType<typeof createInterviewFilterSchema>
>;
