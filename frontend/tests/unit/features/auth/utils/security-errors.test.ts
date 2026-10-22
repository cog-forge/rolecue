import { describe, expect, it } from "vitest";
import { accountLockedMessage } from "@/features/auth/utils/get-auth-error-message";
import {
  securityAccessDenied,
  securityData,
  securityErrorCode,
  securityErrorMessage,
  securityOutcomeUncertain,
} from "@/features/auth/utils/security-errors";

describe("security errors", () => {
  it("reads top-level and nested Better Auth codes", () => {
    expect(securityErrorCode({ code: "INVALID_CODE" })).toBe("INVALID_CODE");
    expect(securityErrorCode({ error: { code: "INVALID_CODE" } })).toBe(
      "INVALID_CODE",
    );
    expect(securityErrorCode({ code: 42 })).toBeUndefined();
    expect(securityErrorCode(null)).toBeUndefined();
  });

  it("maps known codes and never shows server text", () => {
    expect(
      securityErrorMessage({ code: "INVALID_BACKUP_CODE", message: "raw" }),
    ).toBe("That backup code is invalid or has already been used.");
    expect(securityErrorMessage({ code: "BANNED_USER" })).toBe(
      accountLockedMessage,
    );
    expect(securityErrorMessage({ status: 429, message: "raw" })).toBe(
      "Too many requests. Please wait a moment before trying again.",
    );
    expect(securityErrorMessage({ code: "SOMETHING_NEW" })).not.toContain(
      "SOMETHING_NEW",
    );
  });

  it("separates denied access from uncertain outcomes", () => {
    expect(securityAccessDenied({ code: "INVALID_SESSION" })).toBe(true);
    expect(securityAccessDenied({ code: "INVALID_CODE" })).toBe(false);
    expect(securityOutcomeUncertain(new Error("offline"))).toBe(true);
    expect(
      securityOutcomeUncertain({ code: "INVALID_CODE", status: 503 }),
    ).toBe(true);
    expect(
      securityOutcomeUncertain({ code: "INVALID_CODE", status: 400 }),
    ).toBe(false);
  });

  it("rethrows SDK errors and rejects empty data", () => {
    const error = { code: "INVALID_CODE" };
    expect(() => securityData({ data: null, error })).toThrow();
    expect(() => securityData({ data: null, error: null })).toThrow(
      "Authentication data is unavailable",
    );
    expect(securityData({ data: { ok: true }, error: null })).toEqual({
      ok: true,
    });
  });
});
