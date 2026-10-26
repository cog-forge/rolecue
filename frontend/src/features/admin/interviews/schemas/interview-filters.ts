import { z } from "zod";
import { interviewTypes } from "../api/interviews";

const optionalId = z
  .string()
  .trim()
  .refine(
    (value) => value === "" || z.uuid().safeParse(value).success,
    "Enter a complete ID, such as 3f2c…-…, or leave the field empty.",
  );

export const interviewFilterSchema = z.object({
  type: z.enum(["all", ...interviewTypes]),
  candidateId: optionalId,
  jobPostingId: optionalId,
});

export type InterviewFilterValues = z.infer<typeof interviewFilterSchema>;
