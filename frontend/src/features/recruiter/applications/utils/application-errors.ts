"use client";

import { useCallback } from "react";
import { useTranslations } from "next-intl";
import { apiError } from "@/features/recruiter/job-postings/utils/api-error";
import { apiErrorMessage, type ErrorTranslator } from "@/lib/i18n/api-errors";

export type ApplicationErrorTranslator = ReturnType<
  typeof useTranslations<"recruiter.applications.errors">
>;

// Codes with their own wording on these screens (catalog: recruiter.applications.errors).
const applicationCodes = [
  "APPLICATION_NOT_PENDING",
  "INTAKE_NOT_OPEN",
  "INTERVIEW_CAPACITY_FULL",
  "INTERVIEW_CAPACITY_NOT_FUNDED",
  "APPLICATION_NOT_FOUND",
  "JOB_POSTING_NOT_FOUND",
  "APPLICATION_NOT_INTERVIEWED",
  "FINAL_REVIEW_LOCKED",
  "INTERVIEW_NOT_COMPLETED",
] as const;

function isApplicationCode(
  code: string,
): code is (typeof applicationCodes)[number] {
  return (applicationCodes as readonly string[]).includes(code);
}

// Fixed copy for the screening actions and detail reads, in plain Recruiter language; any other
// code uses the shared translated message for it, then the caller's fallback. The server's own
// text is never shown. `t` is the shared error catalog, `applicationT` this screen's copy.
export function applicationErrorMessage(
  error: unknown,
  t: ErrorTranslator,
  applicationT: ApplicationErrorTranslator,
  fallback?: string,
) {
  const { code } = apiError(error);
  if (code !== null && isApplicationCode(code)) return applicationT(code);
  return apiErrorMessage(t, error, fallback);
}

// `message(error, fallback)` for a failed screening, decision or evidence call, in the active language.
export function useApplicationErrorMessage() {
  const t = useTranslations("errors");
  const applicationT = useTranslations("recruiter.applications.errors");
  return useCallback(
    (error: unknown, fallback?: string) =>
      applicationErrorMessage(error, t, applicationT, fallback),
    [t, applicationT],
  );
}
