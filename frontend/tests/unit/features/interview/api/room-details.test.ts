import { beforeEach, expect, it, vi } from "vitest";
import { getRoomDetails } from "@/features/interview/api/room-details";
const get = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/client", () => ({ apiClient: { get } }));
beforeEach(() => get.mockReset());
const id = "aaaaaaaa-aaaa-4aaa-8aaa-000000000053";
it("reads only room identity and avatar, discarding legacy answer/progress payloads", async () => {
  get.mockResolvedValue({
    data: {
      success: true,
      data: {
        id,
        type: "recruitment",
        avatar_id: null,
        current_question: { content: "Private question" },
        turns: [{ answer: "Private answer" }],
        answered_count: 2,
        question_count: 8,
        completed_at: "2026-11-03T08:20:00Z",
      },
    },
  });
  const abort = new AbortController();
  expect(await getRoomDetails(id, abort.signal)).toEqual({
    id,
    type: "recruitment",
    avatar_id: null,
    role_profile_id: null,
    application_id: null,
    completed_at: "2026-11-03T08:20:00Z",
  });
  expect(get).toHaveBeenCalledExactlyOnceWith(`/interviews/${id}`, {
    signal: abort.signal,
  });
});
it("rejects an invalid avatar reference rather than requesting it", async () => {
  get.mockResolvedValue({
    data: {
      success: true,
      data: { id, type: "practice", avatar_id: "invalid" },
    },
  });
  await expect(getRoomDetails(id)).rejects.toThrow();
});
it("keeps the setup a session was started from", async () => {
  const profile = "bbbbbbbb-bbbb-4bbb-8bbb-000000000001";
  get.mockResolvedValue({
    data: {
      success: true,
      data: { id, type: "practice", avatar_id: null, role_profile_id: profile },
    },
  });
  expect(await getRoomDetails(id)).toMatchObject({
    role_profile_id: profile,
    application_id: null,
  });
});
