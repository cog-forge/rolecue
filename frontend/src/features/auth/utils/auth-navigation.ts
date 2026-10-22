import { routes } from "@/config/routes";

// Only the account-security screen may request a return after reauthentication.
export function securityReturnTo(value: unknown) {
  return value === routes.settings ? routes.settings : undefined;
}

export function signedInDestination(role: unknown, returnTo?: string) {
  const home =
    role === "admin"
      ? routes.admin.dashboard
      : role === "recruiter"
        ? routes.recruiter.dashboard
        : routes.dashboard;
  return `${securityReturnTo(returnTo) ?? home}?auth=signed-in`;
}

// Sign-in link that resumes the security flow after reauthentication.
export function reauthenticationHref(returnTo?: string) {
  const destination = securityReturnTo(returnTo);
  return destination
    ? `${routes.login}?returnTo=${encodeURIComponent(destination)}`
    : routes.login;
}

export function challengeDestination(returnTo?: string) {
  const destination = securityReturnTo(returnTo);
  return destination
    ? `${routes.twoFactor}?returnTo=${encodeURIComponent(destination)}`
    : routes.twoFactor;
}
