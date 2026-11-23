import { describe, expect, it } from "vitest";
import { roleProfileSchema } from "@/features/role-profile/schemas/role-profile-schema";

describe("Role Profile candidate boundary", () => {
  it("rejects a candidate response containing a hidden bank", () => {
    const profile = {
      id: "11111111-1111-4111-8111-111111111111",
      title: "Engineer",
      rawText: "source",
      hasOriginalDocument: false,
      seniorityLevel: "senior",
      requirements: [],
      refinementNote: "",
      status: "draft",
      createdAt: "2026-10-10T00:00:00Z",
    };
    expect(
      roleProfileSchema.safeParse({ ...profile, status: "draft" }).success,
    ).toBe(true);
    expect(
      roleProfileSchema.safeParse({
        ...profile,
        requirements: [
          {
            name: "Go",
            target: "Concurrency",
            kind: "skill",
            category: "programming_language",
            type: null,
            evidenceText: null,
          },
        ],
      }).success,
    ).toBe(true);
    expect(
      roleProfileSchema.safeParse({
        ...profile,
        questions: ["Hidden question"],
      }).success,
    ).toBe(false);
  });
});
