import { z } from "zod";
import { apiClient } from "@/lib/api/client";

const envelope = z.object({
  success: z.literal(true),
  data: z.object({
    name: z.string(),
    voice_name: z.string(),
    model_access: z.object({
      url: z.url(),
      method: z.literal("GET"),
      headers: z.record(z.string(), z.string()),
      expires_at: z.iso.datetime({ offset: true }),
    }),
  }),
});

export const companyInterviewerKey = (
  userId: string,
  id: string,
  session = false,
) =>
  [
    "company-interviewer",
    userId,
    session ? "session" : "application",
    id,
  ] as const;

export async function getCompanyInterviewer(
  id: string,
  signal?: AbortSignal,
  session = false,
) {
  const response = await apiClient.get<unknown>(
    `/${session ? "interviews" : "applications"}/${encodeURIComponent(id)}/interviewer`,
    { signal },
  );
  return envelope.parse(response.data).data;
}
