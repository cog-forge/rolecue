import { apiClient } from "@/lib/api/client";
import {
  profileEnvelopeSchema,
  type ProfilePatch,
} from "../schemas/profile-schema";

export const profileKeys = {
  me: (userId: string) => ["profile", "me", userId] as const,
};
export async function getMyProfile(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/profile", { signal });
  return profileEnvelopeSchema.parse(response.data).data;
}
export async function updateMyProfile(patch: ProfilePatch) {
  const response = await apiClient.patch<unknown>("/profile", patch);
  return profileEnvelopeSchema.parse(response.data).data;
}
