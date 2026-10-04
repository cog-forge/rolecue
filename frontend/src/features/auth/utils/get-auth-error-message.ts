type ErrorRecord = Record<string, unknown>;

function asRecord(value: unknown): ErrorRecord | undefined {
  return value && typeof value === "object"
    ? (value as ErrorRecord)
    : undefined;
}

function getErrorCode(error: unknown) {
  const record = asRecord(error);
  const details = asRecord(record?.error);
  return record?.code ?? details?.code;
}

export function isEmailVerificationError(error: unknown) {
  const code = getErrorCode(error);
  const message = asRecord(error)?.message;
  return (
    code === "EMAIL_NOT_VERIFIED" ||
    (typeof message === "string" && message.toLowerCase().includes("verif"))
  );
}

export function getAuthErrorMessage(error: unknown, fallback: string) {
  switch (getErrorCode(error)) {
    case "INVALID_EMAIL_OR_PASSWORD":
      return "Email or password is incorrect.";
    case "USER_ALREADY_EXISTS_USE_ANOTHER_EMAIL":
    case "USER_ALREADY_EXISTS":
      return "An account with this email already exists. Try signing in.";
    case "INVALID_EMAIL":
      return "Please enter a valid email address.";
    case "INVALID_PASSWORD":
      return "Please choose a password with at least 8 characters.";
    case "EMAIL_NOT_VERIFIED":
      return "Please verify your email before signing in.";
    default:
      return fallback;
  }
}
