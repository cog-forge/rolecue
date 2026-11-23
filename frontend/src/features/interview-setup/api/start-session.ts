import { isAxiosError } from "axios";
import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import type { SetupBlocker } from "./saved-setup";

// The practice price in coins. The Go service fixes it as domain.PracticeInterviewCoins and no
// endpoint returns it (Product Decision 15); affordability still comes from GET /wallet.
export const PRACTICE_SESSION_COINS = 5;

// Mirrors domain.Session (snake_case). Only the fields setup reads are validated; the rest
// (current question, turns) belong to the interview room and are stripped here.
export const interviewSessionSchema = z.object({
  id: z.uuid(),
  type: z.enum(["practice", "recruitment"]),
  role_profile_id: z.uuid().nullable(),
  focus_item_id: z
    .uuid()
    .nullish()
    .transform((value) => value ?? null),
  resumed: z.boolean().optional(),
  application_id: z.uuid().nullable(),
  avatar_id: z.uuid().nullable(),
  voice_id: z.uuid().nullable(),
  started_at: z.iso.datetime({ offset: true }),
  completed_at: z.iso.datetime({ offset: true }).nullable(),
  question_count: z.number().int().min(0),
  answered_count: z.number().int().min(0),
});
export type InterviewSession = z.infer<typeof interviewSessionSchema>;

const sessionEnvelopeSchema = z.object({
  success: z.literal(true),
  data: interviewSessionSchema,
});

// Recruitment never names its own avatar or voice: the posting fixes them and the API refuses both.
export type StartRequest =
  | {
      type: "practice";
      roleProfileId: string;
      focusItemId?: string;
      avatarId?: string;
      voiceId?: string;
    }
  | { type: "recruitment"; applicationId: string };

function startBody(request: StartRequest) {
  if (request.type === "recruitment")
    return { type: request.type, application_id: request.applicationId };
  return {
    type: request.type,
    role_profile_id: request.roleProfileId,
    ...(request.focusItemId ? { focus_item_id: request.focusItemId } : {}),
    ...(request.avatarId ? { avatar_id: request.avatarId } : {}),
    ...(request.voiceId ? { voice_id: request.voiceId } : {}),
  };
}

// 201 creates a session; 200 returns one that already existed (an unfinished practice session of
// the same Role Profile, the Application's recruitment session, or a replay of the same key).
export async function createInterviewSession(
  request: StartRequest,
  idempotencyKey: string,
) {
  const response = await apiClient.post<unknown>(
    "/interviews",
    startBody(request),
    { headers: { "Idempotency-Key": idempotencyKey }, timeout: 210_000 },
  );
  const session = sessionEnvelopeSchema.parse(response.data).data;
  return {
    session,
    created: response.status === 201 && session.resumed !== true,
  };
}

// No side effects and no charge; a completed session is refused with INTERVIEW_COMPLETED.
export async function resumeInterviewSession(id: string) {
  const response = await apiClient.post<unknown>(
    `/interviews/${encodeURIComponent(id)}/resume`,
  );
  return sessionEnvelopeSchema.parse(response.data).data;
}

export async function getInterviewSession(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(
    `/interviews/${encodeURIComponent(id)}`,
    { signal },
  );
  return sessionEnvelopeSchema.parse(response.data).data;
}

const errorEnvelopeSchema = z.object({
  success: z.literal(false),
  error: z.object({ code: z.string(), message: z.string() }),
});

// Codes that prove the start did not create a session. Anything else (no response, a timeout, a
// 5xx, an unknown code, a 2xx body that fails validation) may follow a commit, so a retry must
// reuse the same Idempotency-Key.
export const confirmedStartCodes = [
  "VALIDATION_ERROR",
  "FORBIDDEN",
  "INVALID_TOKEN",
  "USER_INACTIVE",
  "ROLE_PROFILE_REQUIRED",
  "JD_NOT_FOUND",
  "QUESTION_BANK_NOT_READY",
  "ROADMAP_NOT_READY",
  "QUESTION_BANK_COVERAGE_INSUFFICIENT",
  "QUESTION_GENERATION_FAILED",
  "INVALID_QUESTION_GENERATION_OUTPUT",
  "LLM_BUDGET_EXCEEDED",
  "INTERVIEW_START_INTENT_CONFLICT",
  "INSUFFICIENT_BALANCE",
  "APPLICATION_NOT_FOUND",
  "APPLICATION_NOT_ELIGIBLE",
  "JOB_POSTING_NOT_FOUND",
  "INTERVIEW_CAPACITY_NOT_FUNDED",
  "INTERVIEW_CAPACITY_FULL",
  "INTERVIEW_NOT_FOUND",
  "INTERVIEW_COMPLETED",
  // The saved setup is not ready: nothing was created. Its blockers come with the error.
  "SETUP_NOT_READY",
] as const;
export type StartCode = (typeof confirmedStartCodes)[number];

export type StartFailure =
  { confirmed: true; code: StartCode; message: string } | { confirmed: false };

function isStartCode(code: string): code is StartCode {
  return (confirmedStartCodes as readonly string[]).includes(code);
}

export function startFailure(error: unknown): StartFailure {
  if (!isAxiosError(error)) return { confirmed: false };
  const body = errorEnvelopeSchema.safeParse(error.response?.data);
  if (!body.success) return { confirmed: false };
  const { code, message } = body.data.error;
  return isStartCode(code)
    ? { confirmed: true, code, message }
    : { confirmed: false };
}

// The blocker shape of the setup API. Declared here, not imported, so this module never depends on
// saved-setup at load time (that module reaches back here through the setup catalog).
const startBlockerSchema = z.object({
  code: z.string(),
  field: z.enum(["avatar", "voice", "environment"]).nullish(),
});

const notReadySchema = z.object({
  error: z.object({
    details: z.object({ blockers: z.array(startBlockerSchema) }).nullish(),
  }),
});

// The blockers a SETUP_NOT_READY answer carries (error.details.blockers, the shape of the setup
// API's own blockers). A missing or malformed details is an empty list, never a failure.
export function startBlockers(error: unknown): SetupBlocker[] {
  if (!isAxiosError(error)) return [];
  const body = notReadySchema.safeParse(error.response?.data);
  return body.success ? (body.data.error.details?.blockers ?? []) : [];
}
