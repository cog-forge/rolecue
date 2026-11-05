import { AxiosError, type AxiosResponse } from "axios";
import { describe, expect, it } from "vitest";
import {
  adminMfaSetupHref,
  isAdminMfaRequired,
} from "@/features/auth/utils/admin-mfa";
import { apiErrorMessage } from "@/lib/i18n/api-errors";
import { errorsFor } from "@tests/helpers/intl";

function apiError(status: number, code: string) {
  return new AxiosError("failed", "ERR_BAD_REQUEST", undefined, undefined, {
    status,
    data: { success: false, error: { code, message: "English text" } },
  } as AxiosResponse);
}

describe("admin two-factor requirement", () => {
  it("recognises only the 403 admin MFA code", () => {
    expect(isAdminMfaRequired(apiError(403, "ADMIN_MFA_REQUIRED"))).toBe(true);
    expect(isAdminMfaRequired(apiError(403, "FORBIDDEN"))).toBe(false);
    expect(isAdminMfaRequired(apiError(401, "ADMIN_MFA_REQUIRED"))).toBe(false);
    expect(isAdminMfaRequired(new Error("x"))).toBe(false);
  });

  it("sends the admin to account security unless already there", () => {
    expect(adminMfaSetupHref("/admin/dashboard")).toBe(
      "/settings?mfa=required",
    );
    expect(adminMfaSetupHref("/settings")).toBeNull();
  });

  it("translates the code in both languages", () => {
    const error = apiError(403, "ADMIN_MFA_REQUIRED");
    expect(apiErrorMessage(errorsFor("en"), error)).toContain(
      "two-factor authentication",
    );
    expect(apiErrorMessage(errorsFor("vi"), error)).toContain("xác thực hai");
  });
});
