import type { ErrorTranslator } from "@/lib/i18n/api-errors";

type ErrorRecord = Record<string, unknown>;

function asRecord(value: unknown): ErrorRecord | undefined {
  return value && typeof value === "object"
    ? (value as ErrorRecord)
    : undefined;
}

export function getErrorCode(error: unknown) {
  const record = asRecord(error);
  const details = asRecord(record?.error);
  return record?.code ?? details?.code;
}

export function isAccountLockedError(error: unknown) {
  return getErrorCode(error) === "BANNED_USER";
}

export function isEmailVerificationError(error: unknown) {
  const code = getErrorCode(error);
  const message = asRecord(error)?.message;
  return (
    code === "EMAIL_NOT_VERIFIED" ||
    (typeof message === "string" && message.toLowerCase().includes("verif"))
  );
}

// Sign-in and sign-up errors by Better Auth code. The server's own text is never shown; a code
// without a translation falls back to the copy the screen passes in.
export function getAuthErrorMessage(
  error: unknown,
  fallback: string,
  t: ErrorTranslator,
) {
  switch (getErrorCode(error)) {
    case "BANNED_USER":
      return t("auth.accountLocked");
    case "INVALID_EMAIL_OR_PASSWORD":
      return t("auth.invalidCredentials");
    case "USER_ALREADY_EXISTS_USE_ANOTHER_EMAIL":
    case "USER_ALREADY_EXISTS":
      return t("auth.emailTaken");
    case "INVALID_EMAIL":
      return t("auth.invalidEmail");
    case "INVALID_PASSWORD":
      return t("auth.weakPassword");
    case "EMAIL_NOT_VERIFIED":
      return t("auth.emailNotVerified");
    default:
      // Some failures only carry Better Auth's English text, so unverified email is also read from it.
      return isEmailVerificationError(error)
        ? t("auth.emailNotVerified")
        : fallback;
  }
}
