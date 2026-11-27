import { apiClient } from "@/lib/api/client";
import { z } from "zod";
import {
  putStorageFile,
  signedStorageRequestSchema,
} from "@/lib/api/signed-storage-request";
import {
  profileEnvelopeSchema,
  type ProfilePatch,
} from "../schemas/profile-schema";

export const profileKeys = {
  me: (userId: string) => ["profile", "me", userId] as const,
};

const photoUploadEnvelope = z.object({
  success: z.literal(true),
  data: z.object({ upload_id: z.uuid(), request: signedStorageRequestSchema }),
});

export type ProfilePhotoUpload = {
  file: File;
  onProgress: (percent: number) => void;
};

export type ProfileUpdate = { patch: ProfilePatch; photo?: ProfilePhotoUpload };
export async function getMyProfile(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/profile", { signal });
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function updateMyProfile({ patch, photo }: ProfileUpdate) {
  let payload = patch;
  if (photo) {
    const { file, onProgress } = photo;
    onProgress(0);
    const response = await apiClient.post<unknown>(
      "/profile/photo-uploads",
      { content_type: file.type, size_bytes: file.size },
      { timeout: 15_000 },
    );
    const { upload_id, request } = photoUploadEnvelope.parse(
      response.data,
    ).data;
    try {
      await putStorageFile(request, file, onProgress);
    } catch (cause) {
      // Keep the draft available for retry when object storage rejects the PUT.
      throw new Error("The profile photo upload failed.", { cause });
    }
    onProgress(100);
    payload = { ...patch, photo_upload_id: upload_id };
  }
  const response = photo
    ? await apiClient.patch<unknown>("/profile", payload, { timeout: 35_000 })
    : await apiClient.patch<unknown>("/profile", payload);
  return profileEnvelopeSchema.parse(response.data).data;
}
