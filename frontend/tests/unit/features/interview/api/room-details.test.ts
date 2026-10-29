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
      },
    },
  });
  const abort = new AbortController();
  expect(await getRoomDetails(id, abort.signal)).toEqual({
    id,
    type: "recruitment",
    avatar_id: null,
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
