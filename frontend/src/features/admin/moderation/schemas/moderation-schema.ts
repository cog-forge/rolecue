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
    company_name: z.string().nullable().optional(),
    company_website: z.string().nullable().optional(),
    seniority_level: z.string().optional(),
    refinement_note: z.string().optional(),
    requirements: z
      .array(
        z.object({
          name: z.string(),
          target: z.string(),
          type: z.enum(["required", "preferred"]),
        }),
      )
      .optional(),
    weights_configured: z.boolean().optional(),
    evaluation: z
      .object({
        configured: z.boolean(),
        using_defaults: z.boolean(),
        total: z.number(),
        weights: z.array(
          z.object({
            metric_id: z.uuid(),
            name: z.string(),
            percentage: z.number(),
          }),
        ),
      })
      .nullable()
      .optional(),
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
    // A storage key for postings created in the app, or a full URL for older rows.
    content_path: z.string(),
    // A short-lived link to the JD PDF; null when the posting has no stored PDF.
    document_url: z.url({ protocol: /^https?$/ }).nullable(),
    document_expires_at: z.iso.datetime({ offset: true }).nullable(),
    // The JD text; null when content_path is a URL or the stored JD is missing.
    raw_text: z.string().nullable(),
    review_submitted: z.boolean(),
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
