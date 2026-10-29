import { describe, expect, it } from "vitest";
import {
  createBackupVerificationSchema,
  createCurrentPasswordSchema,
  createTotpVerificationSchema,
  totpSetupSchema,
} from "@/features/auth/schemas/credentials-schema";
import { validationFor } from "@tests/helpers/intl";

const v = validationFor("en");
const backupVerificationSchema = createBackupVerificationSchema(v);
const currentPasswordSchema = createCurrentPasswordSchema(v);
const totpVerificationSchema = createTotpVerificationSchema(v);

describe("credentials schemas", () => {
  it("requires the current password", () => {
    expect(currentPasswordSchema.safeParse({ password: "" }).success).toBe(
      false,
    );
    expect(
      currentPasswordSchema.safeParse({ password: "secret" }).success,
    ).toBe(true);
  });

  it.each(["12345", "1234567", "12a456", " 123456"])(
    "rejects authenticator code %j",
    (code) => {
      expect(
        totpVerificationSchema.safeParse({ code, trustDevice: false }).success,
      ).toBe(false);
    },
  );

  it("accepts a six-digit code and a trimmed backup code", () => {
    expect(
      totpVerificationSchema.safeParse({ code: "012345", trustDevice: true })
        .success,
    ).toBe(true);
    expect(
      backupVerificationSchema.parse({
        code: "  ABCDE-12345 ",
        trustDevice: false,
      }).code,
    ).toBe("ABCDE-12345");
    expect(
      backupVerificationSchema.safeParse({ code: "   ", trustDevice: false })
        .success,
    ).toBe(false);
  });

  it("accepts only a TOTP setup with backup codes", () => {
    const setup = {
      method: "totp",
      totpURI: "otpauth://totp/RoleCue:test?secret=ABC",
      backupCodes: ["ABCDE-12345"],
    };
    expect(totpSetupSchema.safeParse(setup).success).toBe(true);
    expect(
      totpSetupSchema.safeParse({ ...setup, totpURI: "https://example.com" })
        .success,
    ).toBe(false);
    expect(
      totpSetupSchema.safeParse({ ...setup, backupCodes: [] }).success,
    ).toBe(false);
  });
});
