import { z } from "zod";

// Mirrors api/internal/features/evaluation/domain CoverageReport (snake_case): whether a posting's question
// bank can cover every metric it is scored on, within the number of questions one interview asks (KAN-377).
// `passes` is the binary gate; `covered` of `total` only feeds the display. Recruiter and Admin only.
export const coverageSchema = z.object({
  passes: z.boolean(),
  // False when there was nothing to assess; the screens show no coverage then.
  assessed: z.boolean(),
  covered: z.number().int().nonnegative(),
  total: z.number().int().nonnegative(),
  question_limit: z.number().int().positive(),
  missing_metrics: z.array(
    z.object({
      id: z.uuid(),
      name: z.string(),
      suggested_rubrics: z
        .array(z.object({ id: z.uuid(), name: z.string() }))
        .default([]),
    }),
  ),
  exceeds_limit: z.boolean().default(false),
  min_questions: z.number().int().nonnegative().default(0),
});
export type Coverage = z.infer<typeof coverageSchema>;

// A coverage that blocks sending the posting to review or approving it.
export function coverageBlocks(coverage: Coverage | null | undefined) {
  return !!coverage && coverage.assessed && !coverage.passes;
}
