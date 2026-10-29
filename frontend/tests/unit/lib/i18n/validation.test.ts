import { describe, expect, it } from "vitest";
import { createLoginSchema } from "@/features/auth/schemas/login-schema";
import { createRegisterSchema } from "@/features/auth/schemas/register-schema";
import { createChangePasswordSchema } from "@/features/account-security/schemas/change-password-schema";
import { validationFor } from "@tests/helpers/intl";

function messages(result: {
  success: boolean;
  error?: { issues: { message: string }[] };
}) {
  return result.error ? result.error.issues.map((issue) => issue.message) : [];
}

describe("validation messages come from the catalog", () => {
  it("translates required and format errors", () => {
    const input = { email: "", password: "" };
    expect(
      messages(createLoginSchema(validationFor("en")).safeParse(input)),
    ).toEqual([
      "Email address is required",
      "Please enter a valid email address",
      "Password is required",
    ]);
    expect(
      messages(createLoginSchema(validationFor("vi")).safeParse(input)),
    ).toEqual([
      "Vui lòng nhập địa chỉ email",
      "Vui lòng nhập địa chỉ email hợp lệ",
      "Vui lòng nhập mật khẩu",
    ]);
  });

  it("translates a length rule with its number", () => {
    const input = {
      email: "a@b.co",
      password: "short",
      confirmPassword: "short",
    };
    expect(
      messages(createRegisterSchema(validationFor("en")).safeParse(input)),
    ).toEqual(["Use at least 8 characters"]);
    expect(
      messages(createRegisterSchema(validationFor("vi")).safeParse(input)),
    ).toEqual(["Mật khẩu cần có ít nhất 8 ký tự"]);
  });

  it("translates a cross-field rule", () => {
    const input = {
      currentPassword: "Current-Secret-1",
      newPassword: "Another-Secret-2",
      confirmPassword: "Different-Secret-3",
      revokeOtherSessions: true,
    };
    expect(
      messages(
        createChangePasswordSchema(validationFor("en")).safeParse(input),
      ),
    ).toEqual(["Passwords do not match"]);
    expect(
      messages(
        createChangePasswordSchema(validationFor("vi")).safeParse(input),
      ),
    ).toEqual(["Mật khẩu nhập lại không khớp"]);
  });

  it("accepts valid input in both languages", () => {
    const input = { email: "a@b.co", password: "passphrase" };
    for (const locale of ["en", "vi"] as const)
      expect(
        createLoginSchema(validationFor(locale)).safeParse(input).success,
      ).toBe(true);
  });
});
