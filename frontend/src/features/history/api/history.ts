import { apiClient } from "@/lib/api/client";
import {
  HISTORY_PAGE_SIZE,
  historyPageEnvelopeSchema,
} from "../schemas/history-schema";
import type { HistoryFilters } from "../utils/history-params";

// Scoped to the user: the history belongs to one Candidate.
export const historyKeys = {
  all: (userId: string) => ["interview-history", userId] as const,
  lists: (userId: string) => [...historyKeys.all(userId), "list"] as const,
  list: (userId: string, filters: HistoryFilters) =>
    [...historyKeys.lists(userId), filters] as const,
};

export async function listInterviewHistory(
  filters: HistoryFilters,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/interviews", {
    params: {
      ...(filters.type ? { type: filters.type } : {}),
      page: filters.page,
      page_size: HISTORY_PAGE_SIZE,
    },
    signal,
  });
  return historyPageEnvelopeSchema.parse(response.data).data;
}
