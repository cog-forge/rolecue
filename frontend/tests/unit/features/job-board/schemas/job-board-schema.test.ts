import { describe, expect, it } from "vitest";
import {
  boardPageEnvelopeSchema,
  boardPostingEnvelopeSchema,
} from "@/features/job-board/schemas/job-board-schema";

const posting = {
  id: "33333333-3333-4333-8333-333333333333",
  title: "Go Backend Engineer",
  company_name: "Acme",
  location: "Hanoi",
  employment_type: "Full-time",
  salary_min: 1000,
  salary_max: 2000,
  seniority_level: "mid",
  created_at: "2026-10-12T08:00:00Z",
};
const detail = {
  ...posting,
  company_website: "https://acme.example.com",
  interview_duration: 30,
  raw_text: "We build things.",
  requirements: [
    { name: "Go", target: "programming_language", type: "required" },
  ],
  already_applied: false,
};

describe("job board schemas", () => {
  it("parses a page exactly as Go sends it", () => {
    const parsed = boardPageEnvelopeSchema.parse({
      success: true,
      data: {
        items: [posting],
        pagination: { limit: 20, offset: 0, total: 1 },
      },
    });
    expect(parsed.data.items[0].title).toBe("Go Backend Engineer");
  });

  it("accepts null optional fields", () => {
    const parsed = boardPageEnvelopeSchema.parse({
      success: true,
      data: {
        items: [
          {
            ...posting,
            company_name: null,
            location: null,
            employment_type: null,
            salary_min: null,
            salary_max: null,
          },
        ],
        pagination: { limit: 20, offset: 0, total: 1 },
      },
    });
    expect(parsed.data.items[0].company_name).toBeNull();
  });

  it("parses a detail and keeps an unknown requirement target", () => {
    const parsed = boardPostingEnvelopeSchema.parse({
      success: true,
      data: {
        ...detail,
        requirements: [
          { name: "AWS", target: "cloud_platform", type: "preferred" },
        ],
      },
    });
    expect(parsed.data.requirements[0].target).toBe("cloud_platform");
    expect(parsed.data.already_applied).toBe(false);
  });

  it("drops fields Candidates must never see", () => {
    const parsed = boardPostingEnvelopeSchema.parse({
      success: true,
      data: {
        ...detail,
        interview_slot: 10,
        core_question_count: 50,
        evaluation_weights: [{ metric: "x", weight: 1 }],
      },
    });
    expect(parsed.data).not.toHaveProperty("interview_slot");
    expect(parsed.data).not.toHaveProperty("core_question_count");
    expect(parsed.data).not.toHaveProperty("evaluation_weights");
  });

  it.each([
    ["an unknown level", { seniority_level: "principal" }],
    ["a missing already_applied", { already_applied: undefined }],
    ["a bad id", { id: "nope" }],
    [
      "a bad requirement type",
      { requirements: [{ name: "Go", target: "x", type: "bonus" }] },
    ],
  ])("rejects a drifted detail: %s", (_name, change) => {
    expect(
      boardPostingEnvelopeSchema.safeParse({
        success: true,
        data: { ...detail, ...change },
      }).success,
    ).toBe(false);
  });
});
