import { describe, expect, it } from "vitest";
import {
  getAuthErrorMessage,
  isEmailVerificationError,
} from "@/features/auth/utils/get-auth-error-message";
import { errorsFor } from "@tests/helpers/intl";

describe("getAuthErrorMessage", () => {
  const cases: [string, unknown, string, string][] = [
    [
      "wrong password",
      {
        code: "INVALID_EMAIL_OR_PASSWORD",
        message: "Invalid email or password",
      },
      "Email or password is incorrect.",
      "Email hoặc mật khẩu không đúng.",
    ],
    [
      "unverified email",
      { code: "EMAIL_NOT_VERIFIED", message: "Email not verified" },
      "Please verify your email before signing in.",
      "Vui lòng xác minh email trước khi đăng nhập.",
    ],
    [
      "unverified email without a code",
      { message: "Email not verified" },
      "Please verify your email before signing in.",
      "Vui lòng xác minh email trước khi đăng nhập.",
    ],
    [
      "locked account",
      { code: "BANNED_USER", message: "You have been banned" },
      "Your account has been locked by an administrator. Contact support to request a review. You can sign in again once the lock expires or an administrator unlocks your account.",
      "Tài khoản của bạn đã bị quản trị viên khóa. Hãy liên hệ bộ phận hỗ trợ để được xem xét. Bạn có thể đăng nhập lại khi khóa hết hạn hoặc khi quản trị viên mở khóa tài khoản.",
    ],
  ];

  it.each(cases)("maps %s in both languages", (_name, error, en, vi) => {
    expect(getAuthErrorMessage(error, "fallback", errorsFor("en"))).toBe(en);
    expect(getAuthErrorMessage(error, "fallback", errorsFor("vi"))).toBe(vi);
  });

  it("uses the screen's fallback for an unknown code and never the raw text", () => {
    const error = { code: "SOMETHING_NEW", message: "Raw English" };
    expect(getAuthErrorMessage(error, "fallback", errorsFor("vi"))).toBe(
      "fallback",
    );
  });

  it("recognises an unverified email from its code or its text", () => {
    expect(isEmailVerificationError({ code: "EMAIL_NOT_VERIFIED" })).toBe(true);
    expect(isEmailVerificationError({ message: "Email not verified" })).toBe(
      true,
    );
    expect(isEmailVerificationError({ code: "BANNED_USER" })).toBe(false);
  });
});
