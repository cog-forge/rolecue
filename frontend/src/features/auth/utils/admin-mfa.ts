import { routes } from "@/config/routes";
import { readApiError } from "@/lib/i18n/api-errors";

// The API rejects an Admin session without two-factor authentication when ADMIN_REQUIRE_MFA is on (KAN-265).
export const ADMIN_MFA_REQUIRED = "ADMIN_MFA_REQUIRED";

export function isAdminMfaRequired(error: unknown) {
  const { status, code } = readApiError(error);
  return status === 403 && code === ADMIN_MFA_REQUIRED;
}

// Account security (authenticator setup) is where an Admin enrolls; null when already there.
export function adminMfaSetupHref(currentPath: string) {
  return currentPath.startsWith(routes.settings)
    ? null
    : `${routes.settings}?mfa=required`;
}
