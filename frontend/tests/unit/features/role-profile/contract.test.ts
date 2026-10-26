import { describe, expect, it } from "vitest";
import {
  entrySchema,
  roleProfileSchema,
  pdfFileError,
} from "@/features/role-profile/api/contract";

describe("Role Profile input and candidate boundary", () => {
  it("rejects empty, oversized and non-PDF uploads", () => {
    expect(
      pdfFileError(new File([], "jd.pdf", { type: "application/pdf" })),
    ).toContain("empty");
    expect(
      pdfFileError(
        new File([new Uint8Array(5 * 1024 * 1024 + 1)], "jd.pdf", {
          type: "application/pdf",
        }),
      ),
    ).toContain("5 MB");
    expect(
      pdfFileError(new File(["JD"], "jd.txt", { type: "text/plain" })),
    ).toContain("PDF");
    expect(
      pdfFileError(new File(["JD"], "jd.pdf", { type: "application/pdf" })),
    ).toBeNull();
  });
  it("counts Unicode characters after normalization", () => {
    expect(entrySchema.safeParse({ rawText: "😀".repeat(99) }).success).toBe(
      false,
    );
    expect(
      entrySchema.safeParse({ rawText: `  ${"😀".repeat(100)}\r\n` }).success,
    ).toBe(true);
  });
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
