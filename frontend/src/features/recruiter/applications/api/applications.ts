import { apiClient } from "@/lib/api/client";
import { applicationPageEnvelopeSchema } from "../schemas/application-schema";
import {
  applicationApiParams,
  type ApplicationFilters,
} from "../utils/application-filters";

export const applicationKeys = {
  all: (userId: string) => ["recruiter-applications", userId] as const,
  posting: (userId: string, postingId: string, filters: ApplicationFilters) =>
    [...applicationKeys.all(userId), "posting", postingId, filters] as const,
  own: (userId: string, filters: ApplicationFilters) =>
    [...applicationKeys.all(userId), "own", filters] as const,
};

export async function listPostingApplications(
  postingId: string,
  filters: ApplicationFilters,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(
    `/job-postings/${encodeURIComponent(postingId)}/applications`,
    { params: applicationApiParams(filters, false), signal },
  );
  return applicationPageEnvelopeSchema.parse(response.data).data;
}

export async function listOwnApplications(
  filters: ApplicationFilters,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/recruiter/applications", {
    params: applicationApiParams(filters, true),
    signal,
  });
  return applicationPageEnvelopeSchema.parse(response.data).data;
}
