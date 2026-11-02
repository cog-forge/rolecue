import { isAxiosError } from "axios";
import type { useTranslations } from "next-intl";
import { apiErrorMessage, type ErrorTranslator } from "@/lib/i18n/api-errors";

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

export type ApplyErrorTranslator = ReturnType<
  typeof useTranslations<"candidate.applications.applyErrors">
>;

// Codes with their own wording on this screen (catalog: candidate.applications.applyErrors).
const applyCodes = [
  "ALREADY_APPLIED",
  "INTAKE_NOT_OPEN",
  "JOB_POSTING_NOT_FOUND",
  "FORBIDDEN",
] as const;

function isApplyCode(code: string): code is (typeof applyCodes)[number] {
  return (applyCodes as readonly string[]).includes(code);
}

// Fixed copy for known codes; any other code uses the shared translated message for it. The
// server's own text is never shown. `t` is the shared error catalog, `applyT` this screen's copy.
export function applyErrorMessage(
  error: unknown,
  t: ErrorTranslator,
  applyT: ApplyErrorTranslator,
) {
  const { code } = apiError(error);
  if (code !== null && isApplyCode(code)) return applyT(code);
  if (code === "VALIDATION_ERROR") return applyT("validation");
  return apiErrorMessage(t, error, applyT("upload"));
}

export function canRetryApply(error: unknown) {
  const { code } = apiError(error);
  return code === null || !finalCodes.has(code);
}
