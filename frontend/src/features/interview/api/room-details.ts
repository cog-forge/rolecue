import { z } from "zod";
import { apiClient } from "@/lib/api/client";

/** Read-only room context for the saved avatar. No questions, turns or lifecycle mutations. */
export const roomDetailsSchema = z.object({
  id: z.uuid(),
  type: z.enum(["practice", "recruitment"]),
  avatar_id: z.uuid().nullable(),
});
export type RoomDetails = z.infer<typeof roomDetailsSchema>;
const envelope = z.object({
  success: z.literal(true),
  data: roomDetailsSchema,
});
export async function getRoomDetails(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(
    `/interviews/${encodeURIComponent(id)}`,
    { signal },
  );
  return envelope.parse(response.data).data;
}
