import axios from "axios";
import { z } from "zod";
import { apiClient } from "@/lib/api/client";

// Mirrors domain.RecordingStatus and domain.RecordingUploadGrant (KAN-84/KAN-125). The Candidate only
// ever sees the state of the recording, never its object or a read link.
export const recordStatuses = [
  "none",
  "uploading",
  "processing",
  "ready",
  "failed",
] as const;
export type RecordStatus = (typeof recordStatuses)[number];

const statusEnvelope = z.object({
  success: z.literal(true),
  data: z.object({ record_status: z.enum(recordStatuses) }),
});
const grantEnvelope = z.object({
  success: z.literal(true),
  data: z.object({
    upload_url: z.url(),
    method: z.literal("PUT"),
    headers: z.record(z.string(), z.string()),
    expires_at: z.iso.datetime({ offset: true }),
  }),
});
export type RecordingUploadGrant = z.infer<typeof grantEnvelope>["data"];

const path = (interviewId: string) =>
  `/interviews/${encodeURIComponent(interviewId)}/recording`;

export async function getRecordingStatus(
  interviewId: string,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(path(interviewId), { signal });
  return statusEnvelope.parse(response.data).data.record_status;
}

/** The grant binds the exact byte size and media type, and is valid for 15 minutes. */
export async function createRecordingUpload(
  interviewId: string,
  upload: { contentType: string; size: number },
) {
  const response = await apiClient.post<unknown>(path(interviewId), {
    content_type: upload.contentType,
    size: upload.size,
  });
  return grantEnvelope.parse(response.data).data;
}

/** ready when the object arrived; RECORDING_NOT_FOUND (and record_status failed) when it did not. */
export async function completeRecordingUpload(interviewId: string) {
  const response = await apiClient.post<unknown>(
    `${path(interviewId)}/complete`,
  );
  return statusEnvelope.parse(response.data).data.record_status;
}

/**
 * The same completion, sent while the page goes away during an upload, so a recording whose upload
 * did not finish ends failed instead of uploading forever. keepalive lets it outlive the page; it has
 * no body and no custom header, so the browser sends it without a preflight.
 */
export function completeRecordingUploadOnExit(interviewId: string) {
  void apiClient
    .post(`${path(interviewId)}/complete`, undefined, {
      adapter: "fetch",
      fetchOptions: { keepalive: true },
    })
    .catch(() => undefined);
}

/**
 * PUT straight to storage with the grant's signed headers. The plain axios instance sends no RoleCue
 * cookie to storage, and reports upload progress.
 */
export async function putRecording(
  grant: RecordingUploadGrant,
  blob: Blob,
  onProgress: (fraction: number) => void,
  signal?: AbortSignal,
) {
  await axios.request({
    url: grant.upload_url,
    method: grant.method,
    data: blob,
    headers: grant.headers,
    withCredentials: false,
    signal,
    onUploadProgress: (event) => {
      if (event.total) onProgress(event.loaded / event.total);
    },
  });
}
