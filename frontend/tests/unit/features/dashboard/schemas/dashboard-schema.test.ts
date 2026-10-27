import { describe, expect, it } from "vitest";
import {
  interviewHistoryPageEnvelopeSchema,
  roleProfilePageEnvelopeSchema,
} from "@/features/dashboard/schemas/dashboard-schema";

const interview = {
  id: "33333333-3333-4333-8333-333333333333",
  type: "practice",
  started_at: "2026-10-21T09:00:00Z",
  completed_at: null,
  evaluation_status: "pending",
  status: "in_progress",
  score: null,
  role_profile_title: "Go Backend Engineer",
  job_posting_title: null,
  application_id: null,
};

describe("dashboard wire schemas", () => {
  it("accepts a Role Profile page with a nullable seniority", () => {
    const parsed = roleProfilePageEnvelopeSchema.parse({
      success: true,
      data: {
        items: [
          {
            id: "22222222-2222-4222-8222-222222222222",
            title: "Go Backend Engineer",
            seniorityLevel: null,
            status: "draft",
            createdAt: "2026-10-20T08:00:00Z",
          },
        ],
        pagination: { limit: 3, offset: 0, total: 1 },
      },
    });
    expect(parsed.data.items[0].status).toBe("draft");
  });

  it("rejects a Role Profile status the API does not send", () => {
    expect(() =>
      roleProfilePageEnvelopeSchema.parse({
        success: true,
        data: {
          items: [
            {
              id: "22222222-2222-4222-8222-222222222222",
              title: "Go Backend Engineer",
              seniorityLevel: "mid",
              status: "archived",
              createdAt: "2026-10-20T08:00:00Z",
            },
          ],
          pagination: { limit: 3, offset: 0, total: 1 },
        },
      }),
    ).toThrow();
  });

  it("accepts an unfinished practice interview in the history page", () => {
    const parsed = interviewHistoryPageEnvelopeSchema.parse({
      success: true,
      data: { items: [interview], total: 1, page: 1, page_size: 3 },
    });
    expect(parsed.data.items[0].score).toBeNull();
  });

  it("rejects a score outside 0..100", () => {
    expect(() =>
      interviewHistoryPageEnvelopeSchema.parse({
        success: true,
        data: {
          items: [{ ...interview, score: 101 }],
          total: 1,
          page: 1,
          page_size: 3,
        },
      }),
    ).toThrow();
  });
});
