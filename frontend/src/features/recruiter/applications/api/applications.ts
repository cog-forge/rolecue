import { apiClient } from "@/lib/api/client";
import {
  applicationDetailEnvelopeSchema,
  screeningEnvelopeSchema,
} from "../schemas/application-detail-schema";
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
  detail: (userId: string, postingId: string, applicationId: string) =>
    [
      ...applicationKeys.all(userId),
      "detail",
      postingId,
      applicationId,
    ] as const,
};

const applicationPath = (postingId: string, applicationId: string) =>
  `/job-postings/${encodeURIComponent(postingId)}/applications/${encodeURIComponent(applicationId)}`;

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

export async function getApplicationDetail(
  postingId: string,
  applicationId: string,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(
    applicationPath(postingId, applicationId),
    { signal },
  );
  return applicationDetailEnvelopeSchema.parse(response.data).data;
}

export async function approveApplicationCv(
  postingId: string,
  applicationId: string,
) {
  const response = await apiClient.post<unknown>(
    `${applicationPath(postingId, applicationId)}/approve-cv`,
    {},
  );
  return screeningEnvelopeSchema.parse(response.data).data;
}

// The reason is optional; an empty body rejects without one (RejectCVRequest.reject_reason).
export async function rejectApplicationCv(
  postingId: string,
  applicationId: string,
  reason?: string,
) {
  const body = reason ? { reject_reason: reason } : {};
  const response = await apiClient.post<unknown>(
    `${applicationPath(postingId, applicationId)}/reject-cv`,
    body,
  );
  return screeningEnvelopeSchema.parse(response.data).data;
}
