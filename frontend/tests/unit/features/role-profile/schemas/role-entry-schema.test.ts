import { describe, expect, it } from "vitest";
import { entrySchema } from "@/features/role-profile/schemas/role-entry-schema";

describe("Role Profile entry input", () => {
  it("counts Unicode characters after normalization", () => {
    expect(entrySchema.safeParse({ rawText: "😀".repeat(99) }).success).toBe(
      false,
    );
    expect(
      entrySchema.safeParse({ rawText: `  ${"😀".repeat(100)}\r\n` }).success,
    ).toBe(true);
  });
});
