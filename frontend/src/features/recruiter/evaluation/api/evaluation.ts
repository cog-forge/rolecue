import { apiClient } from "@/lib/api/client";
import {
  metricsEnvelopeSchema,
  postingWeightsEnvelopeSchema,
  type WeightsBody,
} from "../schemas/evaluation-schema";

export const evaluationKeys = {
  all: (userId: string) => ["recruiter-evaluation", userId] as const,
  metrics: (userId: string) =>
    [...evaluationKeys.all(userId), "metrics"] as const,
  weights: (userId: string, postingId: string) =>
    [...evaluationKeys.all(userId), "weights", postingId] as const,
};

const weightsPath = (postingId: string) =>
  `/job-postings/${encodeURIComponent(postingId)}/evaluation-weights`;

export async function getMetrics(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/evaluation-metrics", {
    signal,
  });
  return metricsEnvelopeSchema.parse(response.data).data.items;
}

export async function getWeights(postingId: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(weightsPath(postingId), {
    signal,
  });
  return postingWeightsEnvelopeSchema.parse(response.data).data;
}

export async function saveWeights(postingId: string, body: WeightsBody) {
  const response = await apiClient.put<unknown>(weightsPath(postingId), body);
  return postingWeightsEnvelopeSchema.parse(response.data).data;
}
