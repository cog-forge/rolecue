import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import { evaluationStatusSchema } from "../transport/protocol";

/** How a session ended (docs/interview-ws-protocol.md §3.1). Null for sessions ended before it was
 * recorded. */
export const endReasonSchema = z.enum(["completed", "left", "abandoned"]);
export type EndReason = z.infer<typeof endReasonSchema>;

const endedSessionSchema = z.object({
  id: z.uuid(),
  completed_at: z.iso.datetime({ offset: true }),
  end_reason: endReasonSchema.nullable(),
  evaluation_status: evaluationStatusSchema,
});
export type EndedSession = z.infer<typeof endedSessionSchema>;
const envelope = z.object({
  success: z.literal(true),
  data: endedSessionSchema,
});

/** Leave is REST, not a socket command: POST /interviews/{id}/complete ends the session through the
 * server's one end path, so it also works while the socket is reconnecting. It is idempotent: a double
 * click or a second tab gets the first ending back. */
export async function leaveInterview(id: string, signal?: AbortSignal) {
  const response = await apiClient.post<unknown>(
    `/interviews/${encodeURIComponent(id)}/complete`,
    undefined,
    { signal },
  );
  return envelope.parse(response.data).data;
}
