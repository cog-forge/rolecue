import { isAxiosError } from "axios";
import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import { meSchema } from "@/features/auth/schemas/session-schema";
import { type OnboardingRole } from "../schemas/onboarding-schema";
export async function selectOnboardingRole(role: OnboardingRole) {
  const response = await apiClient.post<unknown>("/onboarding/role", { role });
  return meSchema.parse(response.data).data;
}
const errorSchema = z.object({ error: z.object({ message: z.string() }) });
export function onboardingError(error: unknown) {
  if (isAxiosError(error)) {
    const body = errorSchema.safeParse(error.response?.data);
    if (body.success) return body.data.error.message;
  }
  return "Unable to save your details. Please try again.";
}
