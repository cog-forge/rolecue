import { z } from "zod";
import type { ErrorTranslator } from "@/lib/i18n/api-errors";
import { getErrorCode } from "./get-auth-error-message";

const errorSchema = z.object({ status: z.number().optional() });

export function securityErrorCode(error: unknown) {
  const code = getErrorCode(error);
  return typeof code === "string" ? code : undefined;
}

// Account-security errors by Better Auth code, in the active language. The server's text is never shown.
export function securityErrorMessage(error: unknown, t: ErrorTranslator) {
  switch (securityErrorCode(error)) {
    case "INVALID_PASSWORD":
      return t("auth.wrongCurrentPassword");
    case "INVALID_CODE":
      return t("auth.invalidCode");
    case "INVALID_BACKUP_CODE":
      return t("auth.invalidBackupCode");
    case "INVALID_TWO_FACTOR_COOKIE":
    case "TOO_MANY_ATTEMPTS_REQUEST_NEW_CODE":
      return t("auth.verificationExpired");
    case "ACCOUNT_TEMPORARILY_LOCKED":
      return t("auth.temporarilyLocked");
    case "BANNED_USER":
      return t("auth.accountLocked");
    case "SESSION_NOT_FRESH":
      return t("auth.sessionNotFresh");
    case "UNAUTHORIZED":
    case "INVALID_SESSION":
      return t("auth.sessionExpired");
    case "TOTP_ALREADY_ENABLED":
      return t("auth.totpAlreadyEnabled");
    case "TOTP_NOT_ENABLED":
    case "TWO_FACTOR_NOT_ENABLED":
    case "BACKUP_CODES_NOT_ENABLED":
      return t("auth.totpChanged");
    case "TOTP_NOT_CONFIGURED":
      return t("auth.totpUnavailable");
    case "FAILED_TO_UNLINK_LAST_ACCOUNT":
      return t("auth.lastSignInMethod");
    case "SOCIAL_ACCOUNT_ALREADY_LINKED":
      return t("auth.socialAlreadyLinked");
    case "LINKING_NOT_ALLOWED":
    case "LINKING_DIFFERENT_EMAILS_NOT_ALLOWED":
      return t("auth.linkingNotAllowed");
    default: {
      const parsed = errorSchema.safeParse(error);
      return parsed.success && parsed.data.status === 429
        ? t("auth.tooManyRequests")
        : t("auth.generic");
    }
  }
}

export function securityAccessDenied(error: unknown) {
  return ["UNAUTHORIZED", "INVALID_SESSION", "BANNED_USER"].includes(
    securityErrorCode(error) ?? "",
  );
}

export function securityOutcomeUncertain(error: unknown) {
  const parsed = errorSchema.safeParse(error);
  return (
    !securityErrorCode(error) ||
    ["INTERNAL_SERVER_ERROR", "INTERNAL_ERROR", "UNKNOWN_ERROR"].includes(
      securityErrorCode(error) ?? "",
    ) ||
    (parsed.success && (parsed.data.status ?? 0) >= 500)
  );
}

// Preserve the SDK's error codes; checking result.error is required even for
// successful HTTP transport. Never replace an error with empty credentials data.
export function securityData<T>(result: {
  data: T | null;
  error: unknown;
}): NonNullable<T> {
  if (result.error) throw result.error;
  if (result.data == null) {
    throw Object.assign(new Error("Authentication data is unavailable"), {
      code: "UNAUTHORIZED",
    });
  }
  return result.data;
}
