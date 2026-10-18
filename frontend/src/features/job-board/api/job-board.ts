import { apiClient } from "@/lib/api/client";
import {
  boardPageEnvelopeSchema,
  boardPostingEnvelopeSchema,
} from "../schemas/job-board-schema";
import { boardApiParams, type BoardFilters } from "../utils/board-filters";

// Scoped to the user: the detail says whether this Candidate already applied.
export const jobBoardKeys = {
  all: (userId: string) => ["job-board", userId] as const,
  lists: (userId: string) => [...jobBoardKeys.all(userId), "list"] as const,
  list: (userId: string, filters: BoardFilters) =>
    [...jobBoardKeys.lists(userId), filters] as const,
  details: (userId: string) => [...jobBoardKeys.all(userId), "detail"] as const,
  detail: (userId: string, id: string) =>
    [...jobBoardKeys.details(userId), id] as const,
};

export async function getJobBoard(filters: BoardFilters, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/job-board", {
    params: boardApiParams(filters),
    signal,
  });
  return boardPageEnvelopeSchema.parse(response.data).data;
}

export async function getJobBoardPosting(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(
    `/job-board/${encodeURIComponent(id)}`,
    { signal },
  );
  return boardPostingEnvelopeSchema.parse(response.data).data;
}
