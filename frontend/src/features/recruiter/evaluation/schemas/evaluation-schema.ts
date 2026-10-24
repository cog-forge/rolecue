import { z } from "zod";
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

// `weights` holds the saved set, or the equal-split defaults when `using_defaults` is true. A saved
// set may leave metrics out; `total` is the sum it must reach (0 = no sum rule).
export const postingWeightsSchema = z.object({
  configured: z.boolean(),
  using_defaults: z.boolean(),
  editable: z.boolean(),
  posting_status: postingStatusSchema,
  total: z.number(),
  weights: z.array(weightSchema),
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
