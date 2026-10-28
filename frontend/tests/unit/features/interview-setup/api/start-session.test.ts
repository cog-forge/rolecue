import { beforeEach, describe, expect, it, vi } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import {
  createInterviewSession,
  resumeInterviewSession,
  startFailure,
} from "@/features/interview-setup/api/start-session";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));

const profileId = "22222222-2222-4222-8222-222222222222";
const applicationId = "33333333-3333-4333-8333-333333333333";
const avatarId = "44444444-4444-4444-8444-444444444444";
const voiceId = "55555555-5555-4555-8555-555555555555";
const sessionId = "66666666-6666-4666-8666-666666666666";
const key = "77777777-7777-4777-8777-777777777777";

function session(over: Record<string, unknown> = {}) {
  return {
    id: sessionId,
    type: "practice",
    role_profile_id: profileId,
    application_id: null,
    avatar_id: null,
    voice_id: null,
    started_at: "2026-10-28T08:00:00Z",
    completed_at: null,
    evaluation_status: "pending",
    question_count: 8,
    answered_count: 0,
    current_question: { position: 1, content: "Tell me about a hard bug." },
    turns: [],
    ...over,
  };
}

function apiFailure(status: number, data: unknown) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data,
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

beforeEach(() => {
  mocks.post.mockReset();
});

describe("createInterviewSession", () => {
  it("sends a practice start with its own avatar, voice and the attempt's key", async () => {
    mocks.post.mockResolvedValue({
      status: 201,
      data: { success: true, data: session({ avatar_id: avatarId }) },
    });
    const result = await createInterviewSession(
      { type: "practice", roleProfileId: profileId, avatarId, voiceId },
      key,
    );
    expect(mocks.post).toHaveBeenCalledWith(
      "/interviews",
      {
        type: "practice",
        role_profile_id: profileId,
        avatar_id: avatarId,
        voice_id: voiceId,
      },
      { headers: { "Idempotency-Key": key } },
    );
    expect(result.created).toBe(true);
    expect(result.session.id).toBe(sessionId);
    // The room's data (question, turns) is not carried through setup.
    expect(result.session).not.toHaveProperty("current_question");
  });

  it("omits an unset avatar and voice instead of sending empty values", async () => {
    mocks.post.mockResolvedValue({
      status: 201,
      data: { success: true, data: session() },
    });
    await createInterviewSession(
      { type: "practice", roleProfileId: profileId },
      key,
    );
    expect(mocks.post.mock.calls[0][1]).toEqual({
      type: "practice",
      role_profile_id: profileId,
    });
  });

  it("never sends an avatar or voice for recruitment", async () => {
    mocks.post.mockResolvedValue({
      status: 201,
      data: {
        success: true,
        data: session({
          type: "recruitment",
          role_profile_id: null,
          application_id: applicationId,
          turns: undefined,
        }),
      },
    });
    await createInterviewSession({ type: "recruitment", applicationId }, key);
    expect(mocks.post.mock.calls[0][1]).toEqual({
      type: "recruitment",
      application_id: applicationId,
    });
  });

  it("reports a 200 as an existing session", async () => {
    mocks.post.mockResolvedValue({
      status: 200,
      data: { success: true, data: session({ answered_count: 3 }) },
    });
    const result = await createInterviewSession(
      { type: "practice", roleProfileId: profileId },
      key,
    );
    expect(result.created).toBe(false);
    expect(result.session.answered_count).toBe(3);
  });

  it("rejects a success body that does not match the session contract", async () => {
    mocks.post.mockResolvedValue({
      status: 201,
      data: { success: true, data: { id: "not-a-uuid" } },
    });
    await expect(
      createInterviewSession(
        { type: "practice", roleProfileId: profileId },
        key,
      ),
    ).rejects.toThrow();
  });
});

it("resumes by session id with no body", async () => {
  mocks.post.mockResolvedValue({
    status: 200,
    data: { success: true, data: session() },
  });
  await expect(resumeInterviewSession(sessionId)).resolves.toMatchObject({
    id: sessionId,
  });
  expect(mocks.post).toHaveBeenCalledWith(`/interviews/${sessionId}/resume`);
});

describe("startFailure", () => {
  it("confirms a known code from the error envelope", () => {
    expect(
      startFailure(
        apiFailure(422, {
          success: false,
          error: { code: "INSUFFICIENT_BALANCE", message: "insufficient" },
        }),
      ),
    ).toEqual({
      confirmed: true,
      code: "INSUFFICIENT_BALANCE",
      message: "insufficient",
    });
  });

  it.each([
    [
      "a 500 with an internal code",
      apiFailure(500, {
        success: false,
        error: { code: "INTERNAL_ERROR", message: "x" },
      }),
    ],
    [
      "a proxy error without an envelope",
      apiFailure(503, { error: "Upstream service unavailable" }),
    ],
    ["a dropped connection", new AxiosError("Network Error", "ERR_NETWORK")],
    ["a body that failed validation", new Error("ZodError")],
  ])("treats %s as ambiguous", (_, error) => {
    expect(startFailure(error)).toEqual({ confirmed: false });
  });
});
