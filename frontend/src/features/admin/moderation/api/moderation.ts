import { apiClient } from "@/lib/api/client";
import {
  postingPageEnvelopeSchema,
  reviewPostingEnvelopeSchema,
} from "../schemas/moderation-schema";
import { queueApiParams, type QueueFilters } from "../utils/queue-filters";

export const moderationKeys = {
  all: (userId: string) => ["admin-moderation", userId] as const,
  lists: (userId: string) => [...moderationKeys.all(userId), "list"] as const,
  list: (userId: string, filters: QueueFilters) =>
    [...moderationKeys.lists(userId), filters] as const,
  detail: (userId: string, id: string) =>
    [...moderationKeys.all(userId), "detail", id] as const,
};

const postingPath = (id: string) =>
  `/admin/job-postings/${encodeURIComponent(id)}`;

export async function listPostings(filters: QueueFilters, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/admin/job-postings", {
    params: queueApiParams(filters),
    signal,
  });
  return postingPageEnvelopeSchema.parse(response.data).data;
}

export async function getPosting(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(postingPath(id), { signal });
  return reviewPostingEnvelopeSchema.parse(response.data).data;
}

export async function approvePosting(id: string) {
  const response = await apiClient.post<unknown>(`${postingPath(id)}/approve`);
  return reviewPostingEnvelopeSchema.parse(response.data).data;
}

export async function rejectPosting(id: string, rejectReason: string) {
  const response = await apiClient.post<unknown>(`${postingPath(id)}/reject`, {
    reject_reason: rejectReason,
  });
  return reviewPostingEnvelopeSchema.parse(response.data).data;
}
