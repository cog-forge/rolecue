import {
  PRACTICE_SESSION_COINS,
  type StartFailure,
} from "../api/start-session";

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

// Matched by code. The only server text read is VALIDATION_ERROR's, to tell a removed avatar from a
// retired voice; anything else gets the generic setup message.
export function describeStartFailure(
  failure: StartFailure,
  context: Context,
): StartProblem {
  if (!failure.confirmed)
    return {
      message:
        "We couldn't confirm your interview started. Try again. You won't be charged twice.",
      action: "retry",
    };
  const practice = context.type === "practice";
  switch (failure.code) {
    case "INSUFFICIENT_BALANCE":
      return {
        message:
          context.balance === undefined
            ? `You need ${PRACTICE_SESSION_COINS} coins to start a practice interview.`
            : `You need ${PRACTICE_SESSION_COINS} coins to start a practice interview. Your balance is ${context.balance}.`,
        action: "add-coins",
      };
    case "ROLE_PROFILE_REQUIRED":
      return {
        message: "Confirm this Role Profile before starting.",
        action: "role-profile",
      };
    case "JD_NOT_FOUND":
      return {
        message: "We couldn't find this Role Profile. Choose another one.",
        action: "setup",
      };
    case "QUESTION_BANK_NOT_READY":
      return practice
        ? {
            message:
              "Your interview questions are still being prepared. Try again in a minute.",
            action: "retry",
          }
        : {
            message: "This interview isn't ready yet. Try again later.",
            action: "retry",
          };
    case "VALIDATION_ERROR":
      if (failure.message === "avatar not found")
        return {
          message:
            "This avatar is no longer in your inventory. Choose another.",
          action: "interviewer",
        };
      if (failure.message === "voice not found")
        return {
          message: "This voice is no longer available. Choose another.",
          action: "interviewer",
        };
      return {
        message:
          "Something in your setup isn't valid. Review your choices and try again.",
        action: practice ? "setup" : "application",
      };
    case "APPLICATION_NOT_FOUND":
    case "JOB_POSTING_NOT_FOUND":
      return {
        message: "We couldn't find this application.",
        action: "applications",
      };
    case "APPLICATION_NOT_ELIGIBLE":
      return {
        message:
          "This application can't start an interview now. The deadline may have passed.",
        action: "application",
      };
    case "INTERVIEW_CAPACITY_NOT_FUNDED":
      return {
        message:
          "The company hasn't opened interview slots for this posting yet. Try again later.",
        action: "application",
      };
    case "INTERVIEW_CAPACITY_FULL":
      return {
        message: "All interview slots for this posting are taken.",
        action: "application",
      };
    case "INTERVIEW_COMPLETED":
      return {
        message: practice
          ? "This interview has already finished. You can start a new one."
          : "This interview has already finished.",
        action: practice ? "start-new" : "application",
      };
    case "INTERVIEW_NOT_FOUND":
      // On a resume the session is gone; on a practice start the server found another
      // Candidate's session for that Role Profile id, which is the same as not found for you.
      if (context.resume)
        return practice
          ? {
              message:
                "We couldn't find this interview. You can start a new one.",
              action: "start-new",
            }
          : {
              message: "We couldn't find this interview.",
              action: "application",
            };
      return practice
        ? {
            message: "We couldn't find this Role Profile. Choose another one.",
            action: "setup",
          }
        : {
            message: "We couldn't find this application.",
            action: "applications",
          };
    case "FORBIDDEN":
    case "INVALID_TOKEN":
    case "USER_INACTIVE":
      return {
        message:
          "Only candidate accounts can start interviews. Sign in again if this looks wrong.",
        action: "sign-in",
      };
  }
}
