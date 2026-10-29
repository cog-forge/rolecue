import { isAxiosError } from "axios";
import type { useTranslations } from "next-intl";
import errors from "./messages/en/errors.json";

export type ErrorTranslator = ReturnType<typeof useTranslations<"errors">>;

export type ApiErrorCode = keyof typeof errors.api;

export function isApiErrorCode(code: unknown): code is ApiErrorCode {
  return typeof code === "string" && Object.hasOwn(errors.api, code);
}

type Envelope = { error?: { code?: unknown; message?: unknown } };

// Reads the Go envelope { success:false, error:{ code, message } } without trusting its shape.
// `message` is the backend's English text: keep it for logs, never show it when a code is mapped.
export function readApiError(error: unknown) {
  if (!isAxiosError<Envelope>(error))
    return { status: 0, code: null, message: null, network: false };
  const body = error.response?.data?.error;
  return {
    status: error.response?.status ?? 0,
    code: typeof body?.code === "string" ? body.code : null,
    message: typeof body?.message === "string" ? body.message : null,
    network: !error.response,
  };
}

// Codes that say nothing about the action; a screen's own wording for the failure reads better.
const nonSpecificCodes = new Set<string>([
  "INTERNAL_ERROR",
  "VALIDATION_ERROR",
]);

// Translated text for an API failure, chosen by the stable error code only:
// 1. a code the catalog knows (errors.api.<CODE>), unless it is non-specific and the caller
//    passed its own translated copy for this action,
// 2. the caller's own copy, if it passed one,
// 3. "can't reach RoleCue" when there was no response, 4. rate limiting, 5. a generic line.
export function apiErrorMessage(
  t: ErrorTranslator,
  error: unknown,
  fallback?: string,
) {
  const { code, status, network } = readApiError(error);
  if (isApiErrorCode(code) && !(fallback && nonSpecificCodes.has(code)))
    return t(`api.${code}`);
  if (fallback) return fallback;
  if (network) return t("network");
  if (status === 429) return t("tooManyRequests");
  return t("generic");
}
