import { apiClient } from "@/lib/api/client";
import {
  applicationDetailEnvelopeSchema,
  screeningEnvelopeSchema,
} from "../schemas/application-detail-schema";
import { postingPageEnvelopeSchema } from "@/features/recruiter/job-postings/schemas/job-posting-schema";
import { applicationPageEnvelopeSchema } from "../schemas/application-schema";
import {
  interviewReportEnvelopeSchema,
  recordingEnvelopeSchema,
  transcriptEnvelopeSchema,
} from "../schemas/interview-evidence-schema";
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
  postingOptions: (userId: string) =>
    [...applicationKeys.all(userId), "posting-options"] as const,
  detail: (userId: string, postingId: string, applicationId: string) =>
    [
      ...applicationKeys.all(userId),
      "detail",
      postingId,
      applicationId,
    ] as const,
  report: (userId: string, interviewId: string) =>
    [...applicationKeys.all(userId), "report", interviewId] as const,
  recording: (userId: string, postingId: string, applicationId: string) =>
    [
      ...applicationKeys.all(userId),
      "recording",
      postingId,
      applicationId,
    ] as const,
  transcript: (userId: string, postingId: string, applicationId: string) =>
    [
      ...applicationKeys.all(userId),
      "transcript",
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

// The Recruiter's newest postings for the inbox's posting filter. One page of the API maximum is
// enough for a choice list; a posting beyond it can still be reached from its own page.
export async function listPostingOptions(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/job-postings", {
    params: { limit: 100, offset: 0 },
    signal,
  });
  return postingPageEnvelopeSchema.parse(response.data).data.items;
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

// The evaluation of an interview, read on demand by the owning Recruiter.
export async function getInterviewReport(
  interviewId: string,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(
    `/interviews/${encodeURIComponent(interviewId)}/report`,
    { signal },
  );
  return interviewReportEnvelopeSchema.parse(response.data).data;
}

export async function getApplicationRecording(
  postingId: string,
  applicationId: string,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(
    `${applicationPath(postingId, applicationId)}/recording`,
    { signal },
  );
  return recordingEnvelopeSchema.parse(response.data).data;
}

export async function getApplicationTranscript(
  postingId: string,
  applicationId: string,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(
    `${applicationPath(postingId, applicationId)}/transcript`,
    { signal },
  );
  return transcriptEnvelopeSchema.parse(response.data).data;
}

// The final decision. Only the fields the Go DTO accepts are sent; the reason is sent only when given.
// The response is the updated Application Detail.
export async function decideApplication(
  postingId: string,
  applicationId: string,
  decision: "approve" | "reject",
  reason?: string,
) {
  const body = reason ? { decision, reason } : { decision };
  const response = await apiClient.post<unknown>(
    `${applicationPath(postingId, applicationId)}/decision`,
    body,
  );
  return applicationDetailEnvelopeSchema.parse(response.data).data;
}
