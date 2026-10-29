import { z } from "zod";

// Mirrors the Go job board contract (snake_case). The API never sends the question bank,
// evaluation weights, interview capacity or the required avatar/voice, so neither does this schema.
export const levels = ["intern", "junior", "mid", "senior", "lead"] as const;
export const levelSchema = z.enum(levels);
export type Level = z.infer<typeof levelSchema>;

export const boardRequirementSchema = z.object({
  name: z.string(),
  // Shown as a label only, so a target added later in Go does not break the page.
  target: z.string(),
  type: z.enum(["required", "preferred"]),
});
export type BoardRequirement = z.infer<typeof boardRequirementSchema>;

export const boardPostingSchema = z.object({
  id: z.uuid(),
  title: z.string(),
  company_name: z.string().nullable(),
  location: z.string().nullable(),
  employment_type: z.string().nullable(),
  salary_min: z.number().int().nullable(),
  salary_max: z.number().int().nullable(),
  seniority_level: levelSchema,
  created_at: z.iso.datetime({ offset: true }),
  published_at: z.iso.datetime({ offset: true }).nullable().optional(),
  // A preview for the card: up to five required skills, and how many required skills there are.
  top_skills: z.array(z.string()).optional(),
  required_skill_count: z.number().int().optional(),
});
export type BoardPosting = z.infer<typeof boardPostingSchema>;

export const boardPostingDetailSchema = boardPostingSchema.extend({
  company_website: z.string().nullable(),
  interview_duration: z.number().int().nullable(),
  raw_text: z.string(),
  requirements: z.array(boardRequirementSchema),
  already_applied: z.boolean(),
  // The Candidate's own Application to this posting, so "Applied" can link straight to it.
  application_id: z.uuid().nullable().optional(),
});
export type BoardPostingDetail = z.infer<typeof boardPostingDetailSchema>;

const envelope = <T extends z.ZodType>(data: T) =>
  z.object({ success: z.literal(true), data });

export const boardPageEnvelopeSchema = envelope(
  z.object({
    items: z.array(boardPostingSchema),
    pagination: z.object({
      limit: z.number().int(),
      offset: z.number().int(),
      total: z.number().int(),
    }),
  }),
);
export type BoardPage = z.infer<typeof boardPageEnvelopeSchema>["data"];
export const boardPostingEnvelopeSchema = envelope(boardPostingDetailSchema);
