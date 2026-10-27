import { apiClient } from "@/lib/api/client";
import {
  interviewHistoryPageEnvelopeSchema,
  roleProfilePageEnvelopeSchema,
} from "../schemas/dashboard-schema";

// Dashboard-scoped keys. Cards share no data with the feature screens, so each card owns its key.
export const dashboardKeys = {
  all: (userId: string) => ["dashboard", userId] as const,
  roleProfiles: (userId: string) =>
    [...dashboardKeys.all(userId), "role-profiles"] as const,
  practice: (userId: string) =>
    [...dashboardKeys.all(userId), "practice-interviews"] as const,
};

export async function listDashboardRoleProfiles(
  limit: number,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/role-profiles", {
    params: { limit, offset: 0 },
    signal,
  });
  return roleProfilePageEnvelopeSchema.parse(response.data).data;
}

export async function listPracticeInterviews(
  pageSize: number,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/interviews", {
    params: { type: "practice", page: 1, page_size: pageSize },
    signal,
  });
  return interviewHistoryPageEnvelopeSchema.parse(response.data).data;
}
