import { apiClient } from "@/lib/api/client";
import {
  APPLICATION_PAGE_SIZE,
  candidateApplicationEnvelopeSchema,
  candidateApplicationPageEnvelopeSchema,
  createdApplicationEnvelopeSchema,
} from "../schemas/candidate-application-schema";

// Scoped to the user: the lists and details belong to one Candidate.
export const candidateApplicationKeys = {
  all: (userId: string) => ["candidate-applications", userId] as const,
  lists: (userId: string) =>
    [...candidateApplicationKeys.all(userId), "list"] as const,
  list: (userId: string, page: number) =>
    [...candidateApplicationKeys.lists(userId), page] as const,
  details: (userId: string) =>
    [...candidateApplicationKeys.all(userId), "detail"] as const,
  detail: (userId: string, id: string) =>
    [...candidateApplicationKeys.details(userId), id] as const,
};

export async function listCandidateApplications(
  page: number,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/applications", {
    params: { page, page_size: APPLICATION_PAGE_SIZE },
    signal,
  });
  return candidateApplicationPageEnvelopeSchema.parse(response.data).data;
}

export async function getCandidateApplication(
  id: string,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(
    `/applications/${encodeURIComponent(id)}`,
    { signal },
  );
  return candidateApplicationEnvelopeSchema.parse(response.data).data;
}

// One PDF CV as the single "file" part. onProgress receives whole percentages of the upload.
// The browser sets the multipart boundary when the body is FormData.
export async function uploadApplicationCv(
  postingId: string,
  file: File,
  onProgress?: (percent: number) => void,
) {
  const form = new FormData();
  form.append("file", file);
  const response = await apiClient.post<unknown>(
    `/job-board/${encodeURIComponent(postingId)}/applications`,
    form,
    {
      onUploadProgress: (event) => {
        if (onProgress && event.total)
          onProgress(Math.round((event.loaded * 100) / event.total));
      },
    },
  );
  return createdApplicationEnvelopeSchema.parse(response.data).data;
}
