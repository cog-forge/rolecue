import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { historyPageEnvelopeSchema } from "@/features/history/schemas/history-schema";
import { apiClient } from "@/lib/api/client";
import { getInterviewSession } from "./start-session";

// Scoped to the user: setup reads the Candidate's own catalog and sessions.
export const interviewSetupKeys = {
  all: (userId: string) => ["interview-setup", userId] as const,
  voices: (userId: string) =>
    [...interviewSetupKeys.all(userId), "voices"] as const,
  unfinished: (userId: string) =>
    [...interviewSetupKeys.all(userId), "unfinished-practice"] as const,
};

const voicePageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(z.object({ id: z.uuid(), name: z.string() })),
    pagination: z.object({ total: z.number().int().nonnegative() }),
  }),
});
export type Voice = z.infer<
  typeof voicePageEnvelopeSchema
>["data"]["items"][number];

// Only active voices are listed; 100 is the API's page cap and far above the catalog's size.
export async function listActiveVoices(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/voices", {
    params: { limit: 100, offset: 0 },
    signal,
  });
  return voicePageEnvelopeSchema.parse(response.data).data.items;
}

export type UnfinishedPractice = {
  id: string;
  roleProfileId: string;
  startedAt: string;
  answeredCount: number;
  questionCount: number;
};

// History rows carry the Role Profile's title, not its id, so each in-progress row is read once to
// learn which profile it belongs to. There are normally none or one or two. The newest wins, as on
// the server. A session past the first 100 rows is missed here, but the server still returns it.
export async function listUnfinishedPractice(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/interviews", {
    params: { type: "practice", page: 1, page_size: 100 },
    signal,
  });
  const rows = historyPageEnvelopeSchema
    .parse(response.data)
    .data.items.filter((row) => row.status === "in_progress");
  const sessions = await Promise.all(
    rows.map((row) => getInterviewSession(row.id, signal)),
  );
  const byProfile = new Map<string, UnfinishedPractice>();
  for (const session of sessions) {
    if (!session.role_profile_id || session.completed_at) continue;
    const current = byProfile.get(session.role_profile_id);
    if (
      current &&
      Date.parse(current.startedAt) >= Date.parse(session.started_at)
    )
      continue;
    byProfile.set(session.role_profile_id, {
      id: session.id,
      roleProfileId: session.role_profile_id,
      startedAt: session.started_at,
      answeredCount: session.answered_count,
      questionCount: session.question_count,
    });
  }
  return byProfile;
}

export function useUnfinishedPractice(userId: string) {
  return useQuery({
    queryKey: interviewSetupKeys.unfinished(userId),
    queryFn: ({ signal }) => listUnfinishedPractice(signal),
    // A session can finish in another tab; setup must not offer a stale resume.
    refetchOnMount: "always",
    staleTime: 0,
    retry: false,
  });
}
