import { describe, expect, it } from "vitest";
import {
  avatarDetailEnvelopeSchema,
  avatarMetadataSchema,
  purchaseFormSchema,
} from "@/features/avatar/schemas/avatar-schema";

describe("avatar metadata", () => {
  it("trims and counts code points like the Go validation", () => {
    expect(
      avatarMetadataSchema.parse({ name: "  Mia  ", personality: " Calm " }),
    ).toEqual({ name: "Mia", personality: "Calm" });
    expect(
      avatarMetadataSchema.safeParse({
        name: "😀".repeat(100),
        personality: "x",
      }).success,
    ).toBe(true);
    expect(
      avatarMetadataSchema.safeParse({
        name: "a".repeat(101),
        personality: "x",
      }).success,
    ).toBe(false);
  });

  it.each([
    { name: "   ", personality: "x" },
    { name: "Mia", personality: "" },
    { name: "M\0ia", personality: "x" },
    { name: "Mia", personality: "x".repeat(2001) },
  ])("rejects %j", (value) => {
    expect(avatarMetadataSchema.safeParse(value).success).toBe(false);
  });
});

describe("slot purchase amount", () => {
  it.each([1, 10])("accepts %i", (amount) => {
    expect(purchaseFormSchema.safeParse({ amount }).success).toBe(true);
  });
  it.each([0, 11, 1.5, Number.NaN])("rejects %s", (amount) => {
    expect(purchaseFormSchema.safeParse({ amount }).success).toBe(false);
  });
});

describe("avatar detail", () => {
  const detail = {
    id: "11111111-1111-4111-8111-111111111111",
    name: "Mia",
    personality: "Calm",
    created_at: "2026-10-27T10:00:00Z",
    updated_at: "2026-10-27T10:00:00Z",
    model_access: {
      url: "https://bucket.s3.example.com/avatars/x.vrm?X-Amz-Signature=abc",
      method: "GET",
      headers: {},
      expires_at: "2026-10-27T10:05:00Z",
    },
  };
  it("accepts the presigned grant", () => {
    expect(
      avatarDetailEnvelopeSchema.safeParse({ success: true, data: detail })
        .success,
    ).toBe(true);
  });
  it("rejects a non-GET grant or a script URL", () => {
    for (const model_access of [
      { ...detail.model_access, method: "PUT" },
      { ...detail.model_access, url: "javascript:alert(1)" },
    ])
      expect(
        avatarDetailEnvelopeSchema.safeParse({
          success: true,
          data: { ...detail, model_access },
        }).success,
      ).toBe(false);
  });
});
