import { describe, expect, it } from "vitest";
import {
  confirmedStartCodes,
  type StartCode,
} from "@/features/interview-setup/api/start-session";
import { describeStartFailure } from "@/features/interview-setup/lib/start-errors";

const practice = { type: "practice" as const };
const recruitment = { type: "recruitment" as const };
const confirmed = (code: StartCode, message = "") => ({
  confirmed: true as const,
  code,
  message,
});

describe("describeStartFailure", () => {
  it("offers a retry that cannot double-charge when the outcome is unknown", () => {
    expect(describeStartFailure({ confirmed: false }, practice)).toEqual({
      message:
        "We couldn't confirm your interview started. Try again. You won't be charged twice.",
      action: "retry",
    });
  });

  it("states the price and the real balance when coins are short", () => {
    expect(
      describeStartFailure(confirmed("INSUFFICIENT_BALANCE"), {
        ...practice,
        balance: 3,
      }),
    ).toEqual({
      message:
        "You need 5 coins to start a practice interview. Your balance is 3.",
      action: "add-coins",
    });
  });

  it("tells a removed avatar from a retired voice, and falls back for other validation errors", () => {
    expect(
      describeStartFailure(
        confirmed("VALIDATION_ERROR", "avatar not found"),
        practice,
      ).action,
    ).toBe("interviewer");
    expect(
      describeStartFailure(
        confirmed("VALIDATION_ERROR", "voice not found"),
        practice,
      ).message,
    ).toBe("This voice is no longer available. Choose another.");
    expect(
      describeStartFailure(
        confirmed("VALIDATION_ERROR", "role_profile_id is required"),
        practice,
      ),
    ).toEqual({
      message:
        "Something in your setup isn't valid. Review your choices and try again.",
      action: "setup",
    });
  });

  it("maps the recruitment preconditions back to the application", () => {
    for (const code of [
      "APPLICATION_NOT_ELIGIBLE",
      "INTERVIEW_CAPACITY_NOT_FUNDED",
      "INTERVIEW_CAPACITY_FULL",
    ] as const)
      expect(describeStartFailure(confirmed(code), recruitment).action).toBe(
        "application",
      );
    expect(
      describeStartFailure(confirmed("INTERVIEW_CAPACITY_FULL"), recruitment)
        .message,
    ).toBe("All interview slots for this posting are taken.");
  });

  it("offers a new start when the session to resume has finished or gone", () => {
    expect(
      describeStartFailure(confirmed("INTERVIEW_COMPLETED"), {
        ...practice,
        resume: true,
      }).action,
    ).toBe("start-new");
    expect(
      describeStartFailure(confirmed("INTERVIEW_NOT_FOUND"), {
        ...practice,
        resume: true,
      }).action,
    ).toBe("start-new");
    // On a start, a foreign profile's session is just a profile you don't have.
    expect(
      describeStartFailure(confirmed("INTERVIEW_NOT_FOUND"), practice),
    ).toEqual({
      message: "We couldn't find this Role Profile. Choose another one.",
      action: "setup",
    });
  });

  it("has a message for every confirmed code, for both interview types", () => {
    for (const code of confirmedStartCodes)
      for (const context of [practice, recruitment]) {
        const problem = describeStartFailure(confirmed(code), context);
        expect(problem.message.length, code).toBeGreaterThan(10);
      }
  });
});
