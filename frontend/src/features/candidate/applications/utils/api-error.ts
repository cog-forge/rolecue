import { isAxiosError } from "axios";

// Reads the Go envelope { success:false, error:{ code, message } } without trusting its shape.
export function apiError(error: unknown) {
  if (!isAxiosError<{ error?: { code?: unknown; message?: unknown } }>(error))
    return { status: 0, code: null, message: null };
  const body = error.response?.data?.error;
  return {
    status: error.response?.status ?? 0,
    code: typeof body?.code === "string" ? body.code : null,
    message: typeof body?.message === "string" ? body.message : null,
  };
}

export function isAccessDenied(error: unknown) {
  return [401, 403].includes(apiError(error).status);
}

// Codes where sending the same CV again cannot succeed, so the screen offers no retry.
const finalCodes = new Set([
  "ALREADY_APPLIED",
  "INTAKE_NOT_OPEN",
  "JOB_POSTING_NOT_FOUND",
  "FORBIDDEN",
]);

const applyMessages: Record<string, string> = {
  ALREADY_APPLIED: "You already applied to this posting.",
  INTAKE_NOT_OPEN: "This posting is no longer accepting applications.",
  JOB_POSTING_NOT_FOUND: "This job posting is no longer available.",
  FORBIDDEN: "Only candidate accounts can apply to job postings.",
};

// Fixed copy for known codes; a validation error shows the server's own words.
export function applyErrorMessage(error: unknown) {
  const { code, message } = apiError(error);
  if (code !== null && Object.hasOwn(applyMessages, code))
    return applyMessages[code];
  if (code === "VALIDATION_ERROR")
    return message ?? "Choose a PDF file up to 5 MB.";
  return "We couldn't upload your CV. Try again.";
}

export function canRetryApply(error: unknown) {
  const { code } = apiError(error);
  return code === null || !finalCodes.has(code);
}
