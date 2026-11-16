import { z } from "zod";
import { apiClient } from "@/lib/api/client";

const score = z.number().min(0).max(100);
export const roadmapItemSchema = z.object({
  id: z.uuid(),
  title: z.string(),
  description: z.string().nullable(),
  position: z.number().int(),
  score: score.nullable(),
  evidence_question_count: z.number().int().nonnegative(),
  learning_status: z.enum(["not_started", "in_progress", "achieved"]),
  last_evaluated_at: z.iso.datetime({ offset: true }).nullable(),
});
export const roadmapSchema = z.object({
  id: z.uuid(),
  role_profile_id: z.uuid(),
  generation_status: z.enum(["pending", "generating", "ready", "failed"]),
  learning_status: z.enum(["not_started", "in_progress", "completed"]),
  progress_percent: score.nullable(),
  completed_items: z.number().int().nonnegative(),
  total_items: z.number().int().nonnegative(),
  practice_started: z.boolean(),
  update_state: z.enum(["up_to_date", "updating", "delayed"]),
  pending_result_count: z.number().int().nonnegative(),
  window_size: z.number().int().positive(),
  achieved_threshold: score,
  items: z.array(roadmapItemSchema),
});
export type Roadmap = z.infer<typeof roadmapSchema>;
export type RoadmapItem = z.infer<typeof roadmapItemSchema>;
export const historyRowSchema = z.object({
  interview_id: z.uuid(),
  question_number: z.number().int().positive(),
  result: z.enum(["scored", "skipped"]),
  score,
  question_text: z.string().nullable(),
  evaluated_at: z.iso.datetime({ offset: true }),
  in_current_window: z.boolean(),
});
export type TopicHistoryRow = z.infer<typeof historyRowSchema>;
const historySchema = z.object({
  items: z.array(historyRowSchema),
  pagination: z.object({
    limit: z.number().int().positive(),
    offset: z.number().int().nonnegative(),
    total: z.number().int().nonnegative(),
  }),
});
export const roadmapKeys = {
  all: (userId: string, profileId: string) =>
    ["roadmap", userId, profileId] as const,
  detail: (userId: string, profileId: string) =>
    [...roadmapKeys.all(userId, profileId), "detail"] as const,
  history: (userId: string, profileId: string, itemId: string) =>
    [...roadmapKeys.all(userId, profileId), "history", itemId] as const,
};
const path = (id: string) => `/role-profiles/${encodeURIComponent(id)}/roadmap`;
export async function getRoadmap(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(path(id), { signal });
  return z
    .object({ success: z.literal(true), data: roadmapSchema })
    .parse(response.data).data;
}
export async function prepareRoadmap(
  id: string,
  action: "retry" | "regenerate",
) {
  const response = await apiClient.post<unknown>(
    `${path(id)}/${action}`,
    undefined,
    { timeout: 210_000 },
  );
  return z
    .object({
      success: z.literal(true),
      data: z.object({
        roadmap_id: z.uuid(),
        role_profile_id: z.uuid(),
        generation_status: z.enum(["pending", "generating", "ready", "failed"]),
      }),
    })
    .parse(response.data).data;
}
export async function getTopicHistory(
  profileId: string,
  itemId: string,
  offset: number,
  signal?: AbortSignal,
  limit = 20,
) {
  const response = await apiClient.get<unknown>(
    `${path(profileId)}/items/${encodeURIComponent(itemId)}/history`,
    { params: { limit, offset }, signal },
  );
  return z
    .object({ success: z.literal(true), data: historySchema })
    .parse(response.data).data;
}
// There is no session-to-topic endpoint. Walk the public, paginated history to establish
// which topics actually received evidence; never infer it from the requested focus or overall score.
export async function getSessionTopicEvidence(
  profileId: string,
  itemId: string,
  sessionId: string,
  signal?: AbortSignal,
) {
  const found: TopicHistoryRow[] = [];
  let offset = 0;
  while (true) {
    const page = await getTopicHistory(profileId, itemId, offset, signal, 100);
    found.push(...page.items.filter((row) => row.interview_id === sessionId));
    offset += page.items.length;
    if (offset >= page.pagination.total || page.items.length === 0)
      return found;
  }
}
