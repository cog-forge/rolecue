import { z } from "zod";
import { apiClient } from "@/lib/api/client";

/** Read-only room context for the saved avatar. No questions, turns or lifecycle mutations.
 * completed_at tells a recruitment room whether to record or to finish the upload. */
export const roomDetailsSchema = z.object({
  id: z.uuid(),
  type: z.enum(["practice", "recruitment"]),
  avatar_id: z.uuid().nullable(),
  completed_at: z.iso
    .datetime({ offset: true })
    .nullish()
    .transform((value) => value ?? null),
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
