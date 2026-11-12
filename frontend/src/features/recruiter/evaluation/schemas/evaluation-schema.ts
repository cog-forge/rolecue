import { z } from "zod";
import { coverageSchema } from "@/lib/api/coverage-schema";
import { postingStatusSchema } from "../../job-postings/schemas/job-posting-schema";

// Mirrors the Go KAN-184 evaluation contract (snake_case).
export const metricSchema = z.object({ id: z.uuid(), name: z.string() });
export type Metric = z.infer<typeof metricSchema>;

export const weightSchema = z.object({
  metric_id: z.uuid(),
  name: z.string(),
  percentage: z.number(),
});
export type Weight = z.infer<typeof weightSchema>;

// GET, PUT and DELETE (reset) all answer with this view. `weights` holds the saved set, or the
// equal-split defaults when `using_defaults` is true. A saved set may leave metrics out; `total`
// is the sum it must reach (0 = no sum rule).
export const postingWeightsSchema = z.object({
  configured: z.boolean(),
  using_defaults: z.boolean(),
  editable: z.boolean(),
  posting_status: postingStatusSchema,
  total: z.number(),
  weights: z.array(weightSchema),
  // Whether the posting's question bank still measures every active metric under these weights (KAN-377).
  coverage: coverageSchema.nullable().optional(),
  // Only on the answer to a PUT or DELETE: the change took the posting out of the Admin queue because the bank
  // no longer covers every active metric (KAN-377).
  submission_withdrawn: z.boolean().optional(),
});
export type PostingWeights = z.infer<typeof postingWeightsSchema>;

const envelope = <T extends z.ZodType>(data: T) =>
  z.object({ success: z.literal(true), data });

export const metricsEnvelopeSchema = envelope(
  z.object({ items: z.array(metricSchema) }),
);
export const postingWeightsEnvelopeSchema = envelope(postingWeightsSchema);

// The request body of PUT /job-postings/:id/evaluation-weights. Go rejects unknown fields.
export type WeightsBody = {
  weights: { metric_id: string; percentage: number }[];
};
