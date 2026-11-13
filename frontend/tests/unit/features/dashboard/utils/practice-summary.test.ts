import { describe, expect, it } from "vitest";
import {
  hasCompleteScore,
  recentPractice,
} from "@/features/dashboard/utils/practice-summary";
import type { InterviewHistoryItem } from "@/features/dashboard/schemas/dashboard-schema";

const session: InterviewHistoryItem = {
  id: "11111111-1111-4111-8111-111111111111",
  type: "practice",
  started_at: "2026-11-01T08:00:00Z",
  completed_at: "2026-11-01T08:30:00Z",
  evaluation_status: "completed",
  status: "completed",
  score: 0,
  score_outcome: "complete",
  role_profile_title: "Backend Engineer",
  job_posting_title: null,
  application_id: null,
};
describe("practice summary", () => {
  it("keeps a real zero and legacy complete scores, while excluding incomplete evaluations", () => {
    expect(hasCompleteScore(session)).toBe(true);
    expect(hasCompleteScore({ ...session, score_outcome: null })).toBe(true);
    for (const item of [
      { ...session, score_outcome: "partial" as const },
      { ...session, score_outcome: "withheld" as const },
      { ...session, score: null },
      { ...session, status: "pending" as const },
    ])
      expect(hasCompleteScore(item)).toBe(false);
  });
  it("orders only practice sessions newest first without mutating the API data", () => {
    const newer = { ...session, id: "new", started_at: "2026-11-02T08:00:00Z" };
    const items = [session, { ...newer, type: "recruitment" as const }, newer];
    expect(recentPractice(items).map((item) => item.id)).toEqual([
      "new",
      session.id,
    ]);
    expect(items[0]).toBe(session);
  });
});
