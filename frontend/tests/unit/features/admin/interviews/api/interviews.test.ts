import { describe, expect, it, vi } from "vitest";
import { interviewFilterSchema } from "@/features/admin/interviews/schemas/interview-filters";
import {
  listAdminInterviews,
  getAdminInterview,
} from "@/features/admin/interviews/api/interviews";

const get = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/client", () => ({ apiClient: { get } }));

const interviewId = "11111111-1111-4111-8111-111111111111";
const candidateId = "22222222-2222-4222-8222-222222222222";

describe("admin interviews API", () => {
  it("sends only the filters that are set, with the contract parameter names", async () => {
    get.mockResolvedValueOnce({
      data: {
        success: true,
        data: { items: [], total: 0, page: 2, page_size: 20 },
      },
    });

    await listAdminInterviews({
      page: 2,
      type: "practice",
      candidateId,
      jobPostingId: null,
    });

    expect(get).toHaveBeenLastCalledWith("/admin/interviews", {
      params: {
        page: 2,
        page_size: 20,
        type: "practice",
        candidate_id: candidateId,
      },
      signal: undefined,
    });
  });

  it("strips every field outside the operational allow-list", async () => {
    get.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          items: [
            {
              id: interviewId,
              type: "practice",
              owner: {
                id: candidateId,
                name: "Alex",
                email: "alex@example.com",
                password_hash: "leak",
              },
              job_posting: null,
              score: null,
              has_report: false,
              has_recording: true,
              record_path: "recordings/leak.webm",
              transcript: "leak",
            },
          ],
          total: 1,
          page: 1,
          page_size: 20,
        },
      },
    });

    const result = await listAdminInterviews({
      page: 1,
      type: null,
      candidateId: null,
      jobPostingId: null,
    });
    const serialized = JSON.stringify(result);

    expect(result.items[0]).not.toHaveProperty("record_path");
    expect(result.items[0]).not.toHaveProperty("transcript");
    expect(result.items[0].owner).not.toHaveProperty("password_hash");
    expect(serialized).not.toContain("recordings/leak.webm");
    expect(serialized).not.toContain("leak");
  });

  it("rejects a detail response that is not an envelope with data", async () => {
    get.mockResolvedValueOnce({ data: { success: true } });

    await expect(getAdminInterview(interviewId)).rejects.toThrow();
  });
});

describe("interview filter schema", () => {
  it("accepts empty optional IDs and full UUIDs", () => {
    expect(
      interviewFilterSchema.safeParse({
        type: "all",
        candidateId: "",
        jobPostingId: candidateId,
      }).success,
    ).toBe(true);
  });

  it("rejects partial IDs and unknown types", () => {
    expect(
      interviewFilterSchema.safeParse({
        type: "all",
        candidateId: "22222222",
        jobPostingId: "",
      }).success,
    ).toBe(false);
    expect(
      interviewFilterSchema.safeParse({
        type: "live",
        candidateId: "",
        jobPostingId: "",
      }).success,
    ).toBe(false);
  });
});
