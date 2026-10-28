import { beforeEach, expect, it, vi } from "vitest";
import { listUnfinishedPractice } from "@/features/interview-setup/api/setup-catalog";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));

const profileA = "22222222-2222-4222-8222-222222222222";
const profileB = "22222222-2222-4222-8222-bbbbbbbbbbbb";
const id = (n: number) =>
  `66666666-6666-4666-8666-${String(n).padStart(12, "0")}`;

function row(n: number, status: string) {
  return {
    id: id(n),
    type: "practice",
    started_at: "2026-10-28T08:00:00Z",
    completed_at: status === "in_progress" ? null : "2026-10-28T08:30:00Z",
    evaluation_status: "pending",
    status,
    score: null,
    role_profile_title: "Frontend Engineer",
    job_posting_title: null,
    application_id: null,
  };
}

function detail(n: number, profile: string, startedAt: string) {
  return {
    id: id(n),
    type: "practice",
    role_profile_id: profile,
    application_id: null,
    avatar_id: null,
    voice_id: null,
    started_at: startedAt,
    completed_at: null,
    evaluation_status: "pending",
    question_count: 8,
    answered_count: n,
  };
}

beforeEach(() => {
  mocks.get.mockReset();
});

it("reads each in-progress row once and keeps the newest session per Role Profile", async () => {
  const details: Record<string, ReturnType<typeof detail>> = {
    [id(1)]: detail(1, profileA, "2026-10-28T08:00:00Z"),
    [id(2)]: detail(2, profileA, "2026-10-28T09:00:00+07:00"),
    [id(4)]: detail(4, profileB, "2026-10-27T08:00:00Z"),
  };
  mocks.get.mockImplementation((url: string) =>
    Promise.resolve({
      data:
        url === "/interviews"
          ? {
              success: true,
              data: {
                items: [
                  row(1, "in_progress"),
                  row(2, "in_progress"),
                  row(3, "completed"),
                  row(4, "in_progress"),
                ],
                total: 4,
                page: 1,
                page_size: 100,
              },
            }
          : { success: true, data: details[url.split("/").pop() ?? ""] },
    }),
  );
  const unfinished = await listUnfinishedPractice();
  expect(mocks.get).toHaveBeenCalledWith("/interviews", {
    params: { type: "practice", page: 1, page_size: 100 },
    signal: undefined,
  });
  // The completed row is never read.
  expect(mocks.get).not.toHaveBeenCalledWith(
    `/interviews/${id(3)}`,
    expect.anything(),
  );
  // 08:00Z is newer than 09:00+07:00 (02:00Z), whatever the strings say.
  expect(unfinished.get(profileA)).toMatchObject({
    id: id(1),
    answeredCount: 1,
  });
  expect(unfinished.get(profileB)).toMatchObject({
    id: id(4),
    questionCount: 8,
  });
});
