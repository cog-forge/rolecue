import { isAxiosError } from "axios";
import type { useTranslations } from "next-intl";
import { apiErrorMessage, readApiError } from "@/lib/i18n/api-errors";
import type { ErrorTranslator } from "@/lib/i18n/api-errors";

export type RoleErrorTranslator = ReturnType<
  typeof useTranslations<"candidate.roleProfiles.errors">
>;

// Screen-specific wording for the codes this flow hits most; the server's `message` is never shown.
export function roleErrorMessage(
  t: RoleErrorTranslator,
  errors: ErrorTranslator,
  error: unknown,
) {
  if (isAxiosError(error) && error.code === "ERR_CANCELED")
    return t("canceled");
  const { code, status } = readApiError(error);
  if (code === "PDF_UNAVAILABLE") return t("pdfUnavailable");
  if (code === "EXTRACTION_FAILED") return t("extractionFailed");
  if (code === "ROLE_PROFILE_CONFLICT") return t("notReady");
  return apiErrorMessage(
    errors,
    error,
    status === 409 ? t("notReady") : t("generic"),
  );
}
