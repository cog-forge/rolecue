import { apiClient } from "@/lib/api/client";
import {
  rubricEnvelopeSchema,
  rubricPageEnvelopeSchema,
  rubricMetricsEnvelopeSchema,
  type RubricValues,
} from "../schemas/rubric-schema";
export const rubricKeys = {
  all: (userId: string) => ["rubrics", userId] as const,
};
export async function listRubrics(
  scope: "system" | "own",
  status: string,
  offset: number,
  signal?: AbortSignal,
) {
  const res = await apiClient.get<unknown>("/rubrics", {
    params: { scope, status, limit: 20, offset },
    signal,
  });
  return rubricPageEnvelopeSchema.parse(res.data).data;
}
export async function getRubric(id: string, signal?: AbortSignal) {
  const res = await apiClient.get<unknown>(
    `/rubrics/${encodeURIComponent(id)}`,
    { signal },
  );
  return rubricEnvelopeSchema.parse(res.data).data;
}
export async function getRubricMetrics(signal?: AbortSignal) {
  const res = await apiClient.get<unknown>("/evaluation-metrics", { signal });
  return rubricMetricsEnvelopeSchema.parse(res.data).data.items;
}
export async function writeRubric(input: {
  id?: string;
  body:
    | RubricValues
    | { updated_at: string; is_active: boolean }
    | (RubricValues & { updated_at: string });
}) {
  const res = input.id
    ? await apiClient.patch<unknown>(`/rubrics/${input.id}`, input.body)
    : await apiClient.post<unknown>("/rubrics", input.body);
  return rubricEnvelopeSchema.parse(res.data).data;
}
export async function copyRubric(id: string) {
  const res = await apiClient.post<unknown>(`/rubrics/${id}/copies`, {});
  return rubricEnvelopeSchema.parse(res.data).data;
}
export async function deleteRubric(id: string) {
  await apiClient.delete(`/rubrics/${id}`);
}
