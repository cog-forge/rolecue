import { z } from "zod";

export const criterionSchema = z.object({
  id: z.uuid().optional(),
  metric_id: z.uuid(),
  name: z.string(),
  max_points: z.number().int(),
  scoring_guide: z.string(),
});
export const rubricSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  description: z.string(),
  question_type: z.string(),
  scope: z.enum(["system", "recruiter"]),
  is_active: z.boolean(),
  updated_at: z.string(),
  created_at: z.string(),
  criteria_count: z.number(),
  max_points: z.number(),
  is_own: z.boolean(),
  can_manage: z.boolean(),
  locked: z.boolean(),
  reference_count: z.number().optional(),
  criteria: z.array(criterionSchema).optional(),
});
export const rubricEnvelopeSchema = z.object({
  success: z.literal(true),
  data: rubricSchema,
});
export const rubricPageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(rubricSchema).nullable(),
    pagination: z.object({ total: z.number() }),
  }),
});
export const rubricMetricsEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(z.object({ id: z.uuid(), name: z.string() })),
  }),
});
export type Rubric = z.infer<typeof rubricSchema>;
export type RubricValues = {
  name: string;
  description: string;
  question_type: string;
  criteria: z.infer<typeof criterionSchema>[];
};
export function createRubricFormSchema(
  required: string,
  invalid: string,
  metricIds: readonly string[],
) {
  const text = (max: number) =>
    z.string().trim().min(1, required).max(max, invalid);
  return z.object({
    name: text(120),
    description: z.string().trim().max(1000, invalid),
    question_type: text(64).regex(/^[a-z][a-z0-9_]*$/, invalid),
    criteria: z
      .array(
        z.object({
          id: z.uuid().optional(),
          metric_id: z
            .uuid(invalid)
            .refine((id) => metricIds.includes(id), invalid),
          name: text(160),
          max_points: z.number().int(invalid).min(1, invalid).max(10, invalid),
          scoring_guide: text(2000),
        }),
      )
      .min(1, required)
      .max(10, invalid),
  });
}
export const emptyCriterion = () => ({
  metric_id: "",
  name: "",
  max_points: 1,
  scoring_guide: "",
});
