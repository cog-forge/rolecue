import type { Roadmap } from "@/features/roadmap/api/roadmap";
export const profileId = "22222222-2222-4222-8222-222222222222";
export const sessionId = "66666666-6666-4666-8666-666666666666";
export const itemId = "33333333-3333-4333-8333-333333333333";
export const secondItemId = "44444444-4444-4444-8444-444444444444";
export const thirdItemId = "55555555-5555-4555-8555-555555555555";
export function roadmapFixture(overrides: Partial<Roadmap> = {}): Roadmap {
  return {
    id: "77777777-7777-4777-8777-777777777777",
    role_profile_id: profileId,
    generation_status: "ready",
    learning_status: "in_progress",
    progress_percent: 33.33,
    completed_items: 1,
    total_items: 3,
    practice_started: true,
    update_state: "up_to_date",
    pending_result_count: 0,
    window_size: 5,
    achieved_threshold: 80,
    items: [
      {
        id: itemId,
        title: "SQL & data modelling",
        description: "Indexes, joins, transactions and query performance.",
        position: 1,
        score: 84.6,
        evidence_question_count: 5,
        learning_status: "achieved",
        last_evaluated_at: "2026-11-13T05:10:00Z",
      },
      {
        id: secondItemId,
        title: "Go concurrency",
        description: "Goroutines, channels and safe concurrent code.",
        position: 2,
        score: 72,
        evidence_question_count: 3,
        learning_status: "in_progress",
        last_evaluated_at: "2026-11-13T05:10:00Z",
      },
      {
        id: thirdItemId,
        title: "System design",
        description: "Trade-offs, reliability and service boundaries.",
        position: 3,
        score: null,
        evidence_question_count: 0,
        learning_status: "not_started",
        last_evaluated_at: null,
      },
    ],
    ...overrides,
  };
}
export const profileFixture = {
  id: profileId,
  title: "Backend Engineer",
  hasOriginalDocument: false,
  rawText: "Go, PostgreSQL and distributed systems.",
  seniorityLevel: "mid",
  requirements: [],
  refinementNote: "",
  status: "confirmed",
  createdAt: "2026-11-13T00:00:00Z",
};
export const sessionFixture = {
  id: sessionId,
  type: "practice",
  role_profile_id: profileId,
  application_id: null,
  focus_item_id: null,
  avatar_id: null,
  voice_id: null,
  started_at: "2026-11-13T04:30:00Z",
  completed_at: null,
  question_count: 8,
  answered_count: 0,
};
