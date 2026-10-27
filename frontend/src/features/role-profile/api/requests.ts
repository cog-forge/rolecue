import axios, { isAxiosError } from "axios";
import { apiClient } from "@/lib/api/client";
import {
  documentEnvelopeSchema,
  entrySchema,
  listEnvelopeSchema,
  profileEnvelopeSchema,
  reviewSchema,
  type RoleReview,
  type UploadTicket,
  uploadEnvelopeSchema,
} from "./contract";

export const ROLE_PROFILES_PAGE_SIZE = 20;

const path = (id: string) => `/role-profiles/${encodeURIComponent(id)}`;
export async function listRoleProfiles(offset: number, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/role-profiles", {
    params: { limit: ROLE_PROFILES_PAGE_SIZE, offset },
    signal,
  });
  return listEnvelopeSchema.parse(response.data).data;
}
export async function getRoleProfile(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(path(id), { signal });
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function createRoleProfile(rawText: string) {
  const response = await apiClient.post<unknown>(
    "/role-profiles",
    entrySchema.parse({ rawText }),
  );
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function createPDFUpload(file: File, signal: AbortSignal) {
  const response = await apiClient.post<unknown>(
    "/role-profiles/uploads",
    {
      fileName: file.name,
      sizeBytes: file.size,
      contentType: "application/pdf",
    },
    { signal, timeout: 15_000 },
  );
  return uploadEnvelopeSchema.parse(response.data).data;
}
export async function completePDFUpload(id: string, signal: AbortSignal) {
  const response = await apiClient.post<unknown>(
    `/role-profiles/uploads/${encodeURIComponent(id)}/complete`,
    undefined,
    { signal, timeout: 60_000 },
  );
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function getOriginalDocument(id: string) {
  const response = await apiClient.get<unknown>(`${path(id)}/document`, {
    timeout: 15_000,
  });
  const request = documentEnvelopeSchema.parse(response.data).data;
  if (request.Method !== "GET") throw new Error("Invalid document method");
  return request.URL;
}
export async function saveRoleProfile(id: string, review: RoleReview) {
  const response = await apiClient.put<unknown>(
    path(id),
    reviewSchema.parse(review),
  );
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function confirmRoleProfile(id: string) {
  const response = await apiClient.post<unknown>(
    `${path(id)}/confirm`,
    {},
    { timeout: 150_000 },
  );
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function analyzeRoleProfile(id: string, signal?: AbortSignal) {
  const response = await apiClient.post<unknown>(
    `${path(id)}/analyze`,
    undefined,
    { timeout: 150_000, signal },
  );
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function deleteRoleProfile(id: string) {
  await apiClient.delete(path(id));
}
export async function uploadPDF(
  ticket: UploadTicket,
  file: File,
  signal: AbortSignal,
  progress: (percent: number) => void,
) {
  if (ticket.request.Method !== "PUT") throw new Error("Invalid upload method");
  // Isolated storage transport: never attach the API client's cookies/interceptors.
  await axios.request({
    url: ticket.request.URL,
    method: "PUT",
    headers: ticket.request.Headers,
    data: file,
    withCredentials: false,
    signal,
    timeout: 120_000,
    onUploadProgress: (event) =>
      progress(Math.min(100, Math.round((event.loaded / file.size) * 100))),
  });
}
export function roleError(error: unknown) {
  if (isAxiosError(error)) {
    if (error.code === "ERR_CANCELED")
      return "Canceled. You can retry or open your saved draft.";
    const payload: unknown = error.response?.data;
    if (payload && typeof payload === "object" && "error" in payload) {
      const detail = payload.error;
      if (
        detail &&
        typeof detail === "object" &&
        "message" in detail &&
        typeof detail.message === "string"
      )
        return detail.message;
    }
    if (error.response?.status === 409)
      return "This job description is not ready for that action. Refresh and try again.";
  }
  return "We couldn't complete that step. Please try again.";
}
