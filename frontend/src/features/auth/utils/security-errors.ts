import { z } from "zod";
import { accountLockedMessage, getErrorCode } from "./get-auth-error-message";

const errorSchema = z.object({ status: z.number().optional() });

export function securityErrorCode(error: unknown) {
  const code = getErrorCode(error);
  return typeof code === "string" ? code : undefined;
}

export function securityErrorMessage(error: unknown) {
  switch (securityErrorCode(error)) {
    case "INVALID_PASSWORD":
      return "Your current password is incorrect. Please try again.";
    case "INVALID_CODE":
      return "That code is invalid. Enter a new code from your authenticator.";
    case "INVALID_BACKUP_CODE":
      return "That backup code is invalid or has already been used.";
    case "INVALID_TWO_FACTOR_COOKIE":
    case "TOO_MANY_ATTEMPTS_REQUEST_NEW_CODE":
      return "This verification has expired. Sign in again to start a new verification.";
    case "ACCOUNT_TEMPORARILY_LOCKED":
      return "Too many incorrect codes. Your account is temporarily locked for verification. Please try again later.";
    case "BANNED_USER":
      return accountLockedMessage;
    case "SESSION_NOT_FRESH":
      return "Sign in again to continue with this security action.";
    case "UNAUTHORIZED":
    case "INVALID_SESSION":
      return "Your session has expired. Sign in again to continue.";
    case "TOTP_ALREADY_ENABLED":
      return "An authenticator is already enabled. Refresh your security settings.";
    case "TOTP_NOT_ENABLED":
    case "TWO_FACTOR_NOT_ENABLED":
    case "BACKUP_CODES_NOT_ENABLED":
      return "Your authenticator settings have changed. Refresh this page or sign in again to continue.";
    case "TOTP_NOT_CONFIGURED":
      return "Authenticator verification is unavailable. Try a saved backup code or sign in again.";
    case "FAILED_TO_UNLINK_LAST_ACCOUNT":
      return "Keep at least one sign-in method connected to your account.";
    case "SOCIAL_ACCOUNT_ALREADY_LINKED":
      return "This social account is already connected to another account.";
    case "LINKING_NOT_ALLOWED":
    case "LINKING_DIFFERENT_EMAILS_NOT_ALLOWED":
      return "Use a verified social account with the same email address as your RoleCue account.";
    default: {
      const parsed = errorSchema.safeParse(error);
      return parsed.success && parsed.data.status === 429
        ? "Too many requests. Please wait a moment before trying again."
        : "We couldn't complete this request. Check your connection and refresh before trying again.";
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
