import type { useTranslations } from "next-intl";
import {
  PRACTICE_SESSION_COINS,
  type StartFailure,
} from "../api/start-session";

export type StartErrorTranslator = ReturnType<
  typeof useTranslations<"candidate.setup.startErrors">
>;

// What the start panel offers next to the message. "retry" keeps the same Idempotency-Key only
// when the failure was ambiguous; every confirmed failure starts a new attempt.
export type StartAction =
  | "retry"
  | "add-coins"
  | "role-profile"
  | "setup"
  | "interviewer"
  | "application"
  | "applications"
  | "start-new"
  | "sign-in";

export type StartProblem = { message: string; action: StartAction | null };

type Context = {
  type: "practice" | "recruitment";
  resume?: boolean;
  balance?: number;
};

// Matched by code; the wording lives in candidate.setup.startErrors. The only server text read is
// VALIDATION_ERROR's, to tell a removed avatar from a retired voice. It is compared, never shown.
export function describeStartFailure(
  failure: StartFailure,
  context: Context,
  t: StartErrorTranslator,
): StartProblem {
  if (!failure.confirmed) return { message: t("unconfirmed"), action: "retry" };
  const practice = context.type === "practice";
  switch (failure.code) {
    case "INSUFFICIENT_BALANCE":
      return {
        message:
          context.balance === undefined
            ? t("insufficientBalance", { cost: PRACTICE_SESSION_COINS })
            : t("insufficientBalanceWithBalance", {
                cost: PRACTICE_SESSION_COINS,
                balance: String(context.balance),
              }),
        action: "add-coins",
      };
    case "ROLE_PROFILE_REQUIRED":
      return { message: t("roleProfileRequired"), action: "role-profile" };
    case "JD_NOT_FOUND":
      return { message: t("profileNotFound"), action: "setup" };
    case "QUESTION_BANK_NOT_READY":
      return {
        message: practice
          ? t("questionBankPractice")
          : t("questionBankRecruitment"),
        action: "retry",
      };
    case "VALIDATION_ERROR":
      if (failure.message === "avatar not found")
        return { message: t("avatarGone"), action: "interviewer" };
      if (failure.message === "voice not found")
        return { message: t("voiceGone"), action: "interviewer" };
      return {
        message: t("invalid"),
        action: practice ? "setup" : "application",
      };
    case "APPLICATION_NOT_FOUND":
    case "JOB_POSTING_NOT_FOUND":
      return { message: t("applicationNotFound"), action: "applications" };
    case "APPLICATION_NOT_ELIGIBLE":
      return { message: t("notEligible"), action: "application" };
    case "INTERVIEW_CAPACITY_NOT_FUNDED":
      return { message: t("capacityNotFunded"), action: "application" };
    case "INTERVIEW_CAPACITY_FULL":
      return { message: t("capacityFull"), action: "application" };
    case "INTERVIEW_COMPLETED":
      return {
        message: practice ? t("completedPractice") : t("completedRecruitment"),
        action: practice ? "start-new" : "application",
      };
    case "INTERVIEW_NOT_FOUND":
      // On a resume the session is gone; on a practice start the server found another
      // Candidate's session for that Role Profile id, which is the same as not found for you.
      if (context.resume)
        return practice
          ? { message: t("interviewNotFoundPractice"), action: "start-new" }
          : {
              message: t("interviewNotFoundRecruitment"),
              action: "application",
            };
      return practice
        ? { message: t("profileNotFound"), action: "setup" }
        : { message: t("applicationNotFound"), action: "applications" };
    case "FORBIDDEN":
    case "INVALID_TOKEN":
    case "USER_INACTIVE":
      return { message: t("denied"), action: "sign-in" };
  }
}
