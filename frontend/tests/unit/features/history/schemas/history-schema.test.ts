import { describe, expect, it } from "vitest";
import { historyPageEnvelopeSchema } from "@/features/history/schemas/history-schema";

const item = (over: Record<string, unknown> = {}) => ({
  id: "33333333-3333-4333-8333-333333333333",
  type: "practice",
  started_at: "2026-10-20T08:00:00Z",
  completed_at: "2026-10-20T08:30:00Z",
  evaluation_status: "completed",
  status: "completed",
  score: 72,
  role_profile_title: "Backend Engineer",
  job_posting_title: null,
  application_id: null,
  ...over,
});

const page = (items: unknown[]) => ({
  success: true,
  data: { items, total: items.length, page: 1, page_size: 20 },
});

describe("historyPageEnvelopeSchema", () => {
  it("accepts practice and recruitment rows", () => {
    const result = historyPageEnvelopeSchema.safeParse(
      page([
        item(),
        item({
          id: "44444444-4444-4444-8444-444444444444",
          type: "recruitment",
          status: "in_progress",
          evaluation_status: "pending",
          completed_at: null,
          score: null,
          job_posting_title: "Backend Engineer, Hanoi",
          application_id: "66666666-6666-4666-8666-666666666666",
        }),
      ]),
    );
    expect(result.success).toBe(true);
  });

  it("rejects an unknown interview type", () => {
    expect(
      historyPageEnvelopeSchema.safeParse(page([item({ type: "mock" })]))
        .success,
    ).toBe(false);
  });

  it("rejects a score above 100", () => {
    expect(
      historyPageEnvelopeSchema.safeParse(page([item({ score: 120 })])).success,
    ).toBe(false);
  });

  it("rejects a missing role profile title (contract drift)", () => {
    const withoutTitle: Record<string, unknown> = { ...item() };
    Reflect.deleteProperty(withoutTitle, "role_profile_title");
    expect(
      historyPageEnvelopeSchema.safeParse(page([withoutTitle])).success,
    ).toBe(false);
  });
});
