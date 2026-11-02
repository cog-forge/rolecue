import { readApiError } from "@/lib/i18n/api-errors";

export function isAccessDenied(error: unknown) {
  return [401, 403].includes(readApiError(error).status);
}
