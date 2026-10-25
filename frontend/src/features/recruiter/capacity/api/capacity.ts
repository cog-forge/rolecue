import { apiClient } from "@/lib/api/client";
import { capacityEnvelopeSchema } from "../schemas/capacity-schema";

export const capacityKeys = {
  all: (userId: string) => ["recruiter-capacity", userId] as const,
  detail: (userId: string, postingId: string) =>
    [...capacityKeys.all(userId), postingId] as const,
};

const postingPath = (postingId: string) =>
  `/job-postings/${encodeURIComponent(postingId)}`;

export async function getCapacity(postingId: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(
    `${postingPath(postingId)}/interview-capacity`,
    { signal },
  );
  return capacityEnvelopeSchema.parse(response.data).data;
}

// One purchase per posting: the server answers the current state when the same count repeats.
export async function fundCapacity(postingId: string, slots: number) {
  const response = await apiClient.post<unknown>(
    `${postingPath(postingId)}/interview-capacity`,
    { slots },
  );
  return capacityEnvelopeSchema.parse(response.data).data;
}

// The response body is not read: the panel refetches the capacity state after the close.
export async function closeIntake(postingId: string) {
  await apiClient.post(`${postingPath(postingId)}/close-intake`);
}
