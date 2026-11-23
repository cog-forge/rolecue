import { routes } from "@/config/routes";
import type { SetupBlocker, SetupTarget } from "../api/saved-setup";
import { setupHref, type SetupParams } from "./setup-params";

// Every blocker code the setup API can answer, so a new code fails the build here, not silently.
export const practiceBlockerCodes = [
  "role_profile_not_confirmed",
  "question_bank_not_ready",
  "avatar_required",
  "avatar_unavailable",
  "voice_required",
  "voice_unavailable",
  "environment_required",
  "environment_unsupported",
] as const;
export const recruitmentBlockerCodes = [
  "application_not_eligible",
  "interview_deadline_passed",
  "interview_already_started",
  "locked_avatar_unavailable",
  "locked_voice_unavailable",
  "environment_required",
  "environment_unsupported",
] as const;
export type BlockerCode =
  | (typeof practiceBlockerCodes)[number]
  | (typeof recruitmentBlockerCodes)[number];

export function isBlockerCode(code: string): code is BlockerCode {
  return (
    (practiceBlockerCodes as readonly string[]).includes(code) ||
    (recruitmentBlockerCodes as readonly string[]).includes(code)
  );
}

export const ENVIRONMENT_ANCHOR = "setup-environment";

/** Where a blocker is fixed: the step of its field, else the Role Profile or the Application. */
export function blockerHref(
  blocker: SetupBlocker,
  target: SetupTarget,
  params: SetupParams,
): { href: string; kind: "step" | "profile" | "application" | "here" } {
  if (target.mode === "recruitment") {
    if (blocker.field === "environment")
      return { href: `#${ENVIRONMENT_ANCHOR}`, kind: "here" };
    return {
      href: routes.application(target.applicationId),
      kind: "application",
    };
  }
  if (blocker.field)
    return {
      href: `${setupHref(routes.interviews.new.interviewer, params)}#${blocker.field === "environment" ? ENVIRONMENT_ANCHOR : `setup-${blocker.field}`}`,
      kind: "step",
    };
  return {
    href: routes.roleProfiles.detail(target.roleProfileId),
    kind: "profile",
  };
}
