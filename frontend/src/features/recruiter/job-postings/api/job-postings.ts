import { apiClient } from "@/lib/api/client";
import {
  avatarPageEnvelopeSchema,
  documentEnvelopeSchema,
  documentLinkEnvelopeSchema,
  postingEnvelopeSchema,
  postingPageEnvelopeSchema,
  suggestionEnvelopeSchema,
  voicePageEnvelopeSchema,
  type CreatePostingBody,
  type PostingPatch,
} from "../schemas/job-posting-schema";
import { listApiParams, type ListFilters } from "../utils/list-filters";

export const jobPostingKeys = {
  all: (userId: string) => ["recruiter-job-postings", userId] as const,
  lists: (userId: string) => [...jobPostingKeys.all(userId), "list"] as const,
  list: (userId: string, filters: ListFilters) =>
    [...jobPostingKeys.lists(userId), filters] as const,
  detail: (userId: string, id: string) =>
    [...jobPostingKeys.all(userId), "detail", id] as const,
  document: (userId: string, id: string) =>
    [...jobPostingKeys.detail(userId, id), "document"] as const,
  voices: (userId: string) =>
    [...jobPostingKeys.all(userId), "voices"] as const,
  avatars: (userId: string) =>
    [...jobPostingKeys.all(userId), "avatars"] as const,
};

const postingPath = (id: string) => `/job-postings/${encodeURIComponent(id)}`;
// The pickers show one page of up to 100; the API caps a page there.
const pickerPage = { limit: 100, offset: 0 } as const;

export async function listPostings(filters: ListFilters, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/job-postings", {
    params: listApiParams(filters),
    signal,
  });
  return postingPageEnvelopeSchema.parse(response.data).data;
}

export async function getPosting(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(postingPath(id), { signal });
  return postingEnvelopeSchema.parse(response.data).data;
}

export async function createPosting(body: CreatePostingBody) {
  const response = await apiClient.post<unknown>("/job-postings", body);
  return postingEnvelopeSchema.parse(response.data).data;
}

export async function updatePosting(id: string, patch: PostingPatch) {
  const response = await apiClient.patch<unknown>(postingPath(id), patch);
  return postingEnvelopeSchema.parse(response.data).data;
}

export async function resubmitPosting(id: string) {
  const response = await apiClient.post<unknown>(`${postingPath(id)}/resubmit`);
  return postingEnvelopeSchema.parse(response.data).data;
}

// The browser sets the multipart boundary when the body is FormData.
export async function uploadDocument(file: File) {
  const form = new FormData();
  form.append("file", file);
  const response = await apiClient.post<unknown>(
    "/job-postings/documents",
    form,
  );
  return documentEnvelopeSchema.parse(response.data).data;
}

export async function getDocumentLink(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(`${postingPath(id)}/document`, {
    signal,
  });
  return documentLinkEnvelopeSchema.parse(response.data).data;
}

export async function extractRequirements(rawText: string) {
  const response = await apiClient.post<unknown>("/job-postings/extract", {
    raw_text: rawText,
  });
  return suggestionEnvelopeSchema.parse(response.data).data;
}

export async function listVoices(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/voices", {
    params: pickerPage,
    signal,
  });
  return voicePageEnvelopeSchema.parse(response.data).data;
}

export async function listAvatars(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/inventory/avatars", {
    params: pickerPage,
    signal,
  });
  return avatarPageEnvelopeSchema.parse(response.data).data;
}
