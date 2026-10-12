import { z } from "zod";

// Mirrors api/internal/features/admin/moderation/domain (snake_case JSON).
export const postingStatuses = [
  "pending",
  "open",
  "intake_closed",
  "closed",
  "rejected",
] as const;
export const postingStatusSchema = z.enum(postingStatuses);
export type PostingStatus = z.infer<typeof postingStatusSchema>;

const recruiterSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  email: z.string(),
});
const namedRefSchema = z
  .object({ id: z.uuid(), name: z.string().nullable() })
  .nullable();

const postingSummarySchema = z.object({
  id: z.uuid(),
  title: z.string(),
  status: postingStatusSchema,
  recruiter: recruiterSchema,
  created_at: z.iso.datetime({ offset: true }),
});

export const postingPageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(postingSummarySchema),
    pagination: z.object({
      limit: z.number().int(),
      offset: z.number().int(),
      total: z.number().int(),
    }),
  }),
});
export type PostingPage = z.infer<typeof postingPageEnvelopeSchema>["data"];
export type PostingSummary = PostingPage["items"][number];

export const reviewPostingEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    id: z.uuid(),
    title: z.string(),
    status: postingStatusSchema,
    reject_reason: z.string().nullable(),
    recruiter: recruiterSchema,
    location: z.string().nullable(),
    employment_type: z.string().nullable(),
    salary_min: z.number().int().nullable(),
    salary_max: z.number().int().nullable(),
    interview_slot: z.number().int().nullable(),
    interview_duration: z.number().int().nullable(),
    required_avatar: namedRefSchema,
    required_voice: namedRefSchema,
    created_at: z.iso.datetime({ offset: true }),
    content_path: z.string(),
    core_question_count: z.number().int().nonnegative(),
  }),
});
export type ReviewPosting = z.infer<typeof reviewPostingEnvelopeSchema>["data"];

// Same rules as the Go service: trimmed, 1-1000 Unicode code points, no NUL.
export const MAX_REJECT_REASON = 1000;
const codePoints = (value: string) => Array.from(value).length;
export const rejectFormSchema = z.object({
  reject_reason: z.string().superRefine((value, ctx) => {
    const reason = value.trim();
    if (!reason)
      ctx.addIssue({
        code: "custom",
        message: "Tell the recruiter why this posting is rejected.",
      });
    else if (codePoints(reason) > MAX_REJECT_REASON)
      ctx.addIssue({
        code: "custom",
        message: `Keep the reason to ${MAX_REJECT_REASON.toLocaleString("en")} characters or fewer.`,
      });
    else if (reason.includes("\u0000"))
      ctx.addIssue({
        code: "custom",
        message: "Remove the unsupported characters from the reason.",
      });
  }),
});
export type RejectValues = z.infer<typeof rejectFormSchema>;
export function rejectReasonLength(value: string) {
  return codePoints(value.trim());
}
