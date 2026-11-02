import type { useTranslations } from "next-intl";
import { apiError } from "@/features/recruiter/job-postings/utils/api-error";
import { apiErrorMessage, type ErrorTranslator } from "@/lib/i18n/api-errors";

export type CapacityErrorTranslator = ReturnType<
  typeof useTranslations<"recruiter.capacity">
>;

// Codes with their own wording on the capacity and close-intake calls, in plain Recruiter language
// (catalog: recruiter.capacity.errors). This is the only place these codes are turned into copy.
const capacityCodes = [
  "INSUFFICIENT_BALANCE",
  "INTERVIEW_CAPACITY_ALREADY_FUNDED",
  "INTAKE_NOT_OPEN",
] as const;

function isCapacityCode(code: string): code is (typeof capacityCodes)[number] {
  return (capacityCodes as readonly string[]).includes(code);
}

// Screen copy for known codes; any other code uses the shared translated message for it, then the
// caller's fallback. The server's own text is never shown.
export function capacityErrorMessage(
  error: unknown,
  t: ErrorTranslator,
  capacityT: CapacityErrorTranslator,
  fallback: string = capacityT("errors.fallback"),
) {
  const { code } = apiError(error);
  if (code !== null && isCapacityCode(code)) return capacityT(`errors.${code}`);
  return apiErrorMessage(t, error, fallback);
}
