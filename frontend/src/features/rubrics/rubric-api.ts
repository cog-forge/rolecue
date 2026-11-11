import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import { rubricSchema, type RubricValues } from "./rubric-schema";
const envelope = <T extends z.ZodType>(data: T) =>
  z.object({ success: z.literal(true), data });
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
  return envelope(
    z.object({
      items: z.array(rubricSchema).nullable(),
      pagination: z.object({ total: z.number() }),
    }),
  ).parse(res.data).data;
}
export async function getRubric(id: string, signal?: AbortSignal) {
  const res = await apiClient.get<unknown>(
    `/rubrics/${encodeURIComponent(id)}`,
    { signal },
  );
  return envelope(rubricSchema).parse(res.data).data;
}
export async function getRubricMetrics(signal?: AbortSignal) {
  const res = await apiClient.get<unknown>("/evaluation-metrics", { signal });
  return envelope(
    z.object({ items: z.array(z.object({ id: z.uuid(), name: z.string() })) }),
  ).parse(res.data).data.items;
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
  return envelope(rubricSchema).parse(res.data).data;
}
export async function copyRubric(id: string) {
  const res = await apiClient.post<unknown>(`/rubrics/${id}/copies`, {});
  return envelope(rubricSchema).parse(res.data).data;
}
export async function deleteRubric(id: string) {
  await apiClient.delete(`/rubrics/${id}`);
}
