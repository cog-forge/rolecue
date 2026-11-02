import { isAxiosError } from "axios";
import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import type { SessionCheck } from "./reconnect-policy";

const sessionSchema = z.object({
  success: z.literal(true),
  data: z.object({ completed_at: z.string().nullable() }),
});

/** Reads only whether the session is still open; the live socket stays the source of interview state. */
export async function checkInterviewSession(
  interviewId: string,
  signal?: AbortSignal,
): Promise<SessionCheck> {
  try {
    const response = await apiClient.get<unknown>(
      `/interviews/${encodeURIComponent(interviewId)}`,
      { signal },
    );
    const parsed = sessionSchema.safeParse(response.data);
    if (!parsed.success) return "unreachable";
    return parsed.data.data.completed_at === null ? "active" : "completed";
  } catch (error) {
    const status = isAxiosError(error) ? (error.response?.status ?? 0) : 0;
    if (status === 401 || status === 403) return "denied";
    if (status === 400 || status === 404) return "not-found";
    return "unreachable";
  }
}
