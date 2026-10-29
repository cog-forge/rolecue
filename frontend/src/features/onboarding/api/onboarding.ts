import { apiClient } from "@/lib/api/client";
import { meSchema } from "@/features/auth/schemas/session-schema";
import { type OnboardingRole } from "../schemas/onboarding-schema";
import { apiErrorMessage, type ErrorTranslator } from "@/lib/i18n/api-errors";
export async function selectOnboardingRole(role: OnboardingRole) {
  const response = await apiClient.post<unknown>("/onboarding/role", { role });
  return meSchema.parse(response.data).data;
}
export function onboardingError(error: unknown, t: ErrorTranslator) {
  return apiErrorMessage(
    t,
    error,
    "Unable to save your details. Please try again.",
  );
}
