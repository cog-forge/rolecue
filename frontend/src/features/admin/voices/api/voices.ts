import { apiClient } from "@/lib/api/client";
import {
  removeVoiceEnvelopeSchema,
  voiceEnvelopeSchema,
  voicePageEnvelopeSchema,
} from "../schemas/voice-schema";
import { voiceApiParams, type VoiceFilters } from "../utils/voice-filters";

export const voiceKeys = {
  all: (userId: string) => ["admin-voices", userId] as const,
  lists: (userId: string) => [...voiceKeys.all(userId), "list"] as const,
  list: (userId: string, filters: VoiceFilters) =>
    [...voiceKeys.lists(userId), filters] as const,
};

export async function listVoices(filters: VoiceFilters, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/admin/voices", {
    params: voiceApiParams(filters),
    signal,
  });
  return voicePageEnvelopeSchema.parse(response.data).data;
}

export async function removeVoice(id: string) {
  const response = await apiClient.delete<unknown>(`/admin/voices/${encodeURIComponent(id)}`);
  return removeVoiceEnvelopeSchema.parse(response.data).data;
}

// Safe to repeat: reactivating an active voice changes nothing. A deleted voice is a 404.
export async function activateVoice(id: string) {
  const response = await apiClient.patch<unknown>(`/admin/voices/${encodeURIComponent(id)}/activate`);
  return voiceEnvelopeSchema.parse(response.data).data;
}
