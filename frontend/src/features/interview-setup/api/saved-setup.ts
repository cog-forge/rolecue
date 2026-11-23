import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import { roomBackgrounds } from "@/features/interview/components/room-backgrounds";
import { interviewSetupKeys } from "./setup-catalog";

// The saved pre-interview setup (KAN-231): avatar, voice and environment, plus a live readiness.
// Contract: api/docs/interview-setup-api.md. A setup is a draft, never an interview.

export const environmentIds = roomBackgrounds.map((room) => room.id);
export type EnvironmentId = (typeof roomBackgrounds)[number]["id"];

const named = z.object({ id: z.uuid(), name: z.string() });
export const blockerSchema = z.object({
  code: z.string(),
  field: z.enum(["avatar", "voice", "environment"]).nullish(),
});
export type SetupBlocker = z.infer<typeof blockerSchema>;
export type BlockerField = NonNullable<SetupBlocker["field"]>;

export const savedSetupSchema = z.object({
  mode: z.enum(["practice", "recruitment"]),
  role_profile_id: z.uuid().nullish(),
  application_id: z.uuid().nullish(),
  target_title: z.string(),
  avatar: named.nullable(),
  voice: named.nullable(),
  // Kept as text: a stale or unknown id is reported as a blocker, never a parse failure.
  environment: z.string().nullable(),
  locked_fields: z.array(z.string()).nullish(),
  saved: z.boolean(),
  updated_at: z.string().nullish(),
  ready: z.boolean(),
  blockers: z.array(blockerSchema),
});
export type SavedSetup = z.infer<typeof savedSetupSchema>;
const envelopeSchema = z.object({
  success: z.literal(true),
  data: savedSetupSchema,
});

export type SetupTarget =
  | { mode: "practice"; roleProfileId: string }
  | { mode: "recruitment"; applicationId: string };

/** A full replace. Recruitment carries the environment only: its avatar and voice are locked. */
export type SetupDraft = {
  avatarId: string | null;
  voiceId: string | null;
  environment: string | null;
};

const targetId = (target: SetupTarget) =>
  target.mode === "practice" ? target.roleProfileId : target.applicationId;
const path = (target: SetupTarget) =>
  `/interview-setups/${target.mode}/${encodeURIComponent(targetId(target))}`;

export const savedSetupKeys = {
  detail: (userId: string, target: SetupTarget) =>
    [
      ...interviewSetupKeys.all(userId),
      "saved",
      target.mode,
      targetId(target),
    ] as const,
};

export async function getSavedSetup(target: SetupTarget, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(path(target), { signal });
  return envelopeSchema.parse(response.data).data;
}

export function saveBody(target: SetupTarget, draft: SetupDraft) {
  return target.mode === "practice"
    ? {
        avatar_id: draft.avatarId,
        voice_id: draft.voiceId,
        environment: draft.environment,
      }
    : { environment: draft.environment };
}

export async function putSavedSetup(target: SetupTarget, draft: SetupDraft) {
  const response = await apiClient.put<unknown>(
    path(target),
    saveBody(target, draft),
  );
  return envelopeSchema.parse(response.data).data;
}

export function useSavedSetup(userId: string, target: SetupTarget) {
  return useQuery({
    queryKey: savedSetupKeys.detail(userId, target),
    queryFn: ({ signal }) => getSavedSetup(target, signal),
    // Readiness is recomputed from live data on every read, so never serve a stale answer.
    staleTime: 0,
    refetchOnMount: "always",
    retry: false,
  });
}

// Saves are serialised per target (the latest choice is always sent last) and the server's answer
// replaces the cached view, so readiness is shown from what was really stored.
export function useSaveSetup(userId: string, target: SetupTarget) {
  const cache = useQueryClient();
  return useMutation({
    scope: { id: `interview-setup-${target.mode}-${targetId(target)}` },
    mutationFn: (draft: SetupDraft) => putSavedSetup(target, draft),
    onSuccess: (view) => storeSetup(cache, userId, target, view),
  });
}

export function storeSetup(
  cache: QueryClient,
  userId: string,
  target: SetupTarget,
  view: SavedSetup,
) {
  cache.setQueryData(savedSetupKeys.detail(userId, target), view);
}

export function draftOf(view: SavedSetup): SetupDraft {
  return {
    avatarId: view.avatar?.id ?? null,
    voiceId: view.voice?.id ?? null,
    environment: view.environment,
  };
}
