import { createTranslator } from "next-intl";
import { describe, expect, it } from "vitest";
import { messagesFor } from "@tests/helpers/intl";
import {
  confirmedStartCodes,
  type StartCode,
} from "@/features/interview-setup/api/start-session";
import { describeStartFailure } from "@/features/interview-setup/lib/start-errors";

const translator = (locale: "en" | "vi") =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "candidate.setup.startErrors",
  });
const describeFailure = (
  failure: Parameters<typeof describeStartFailure>[0],
  context: Parameters<typeof describeStartFailure>[1],
  locale: "en" | "vi" = "en",
) => describeStartFailure(failure, context, translator(locale));

const practice = { type: "practice" as const };
const recruitment = { type: "recruitment" as const };
const confirmed = (code: StartCode, message = "") => ({
  confirmed: true as const,
  code,
  message,
});

describe("describeStartFailure", () => {
  it("offers a retry that cannot double-charge when the outcome is unknown", () => {
    expect(describeFailure({ confirmed: false }, practice)).toEqual({
      message:
        "We couldn't confirm your interview started. Try again. You won't be charged twice.",
      action: "retry",
    });
  });

  it("states the price and the real balance when coins are short", () => {
    expect(
      describeFailure(confirmed("INSUFFICIENT_BALANCE"), {
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
      describeFailure(
        confirmed("VALIDATION_ERROR", "avatar not found"),
        practice,
      ).action,
    ).toBe("interviewer");
    expect(
      describeFailure(
        confirmed("VALIDATION_ERROR", "voice not found"),
        practice,
      ).message,
    ).toBe("This voice is no longer available. Choose another.");
    expect(
      describeFailure(
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
      expect(describeFailure(confirmed(code), recruitment).action).toBe(
        "application",
      );
    expect(
      describeFailure(confirmed("INTERVIEW_CAPACITY_FULL"), recruitment)
        .message,
    ).toBe("All interview slots for this posting are taken.");
  });

  it("offers a new start when the session to resume has finished or gone", () => {
    expect(
      describeFailure(confirmed("INTERVIEW_COMPLETED"), {
        ...practice,
        resume: true,
      }).action,
    ).toBe("start-new");
    expect(
      describeFailure(confirmed("INTERVIEW_NOT_FOUND"), {
        ...practice,
        resume: true,
      }).action,
    ).toBe("start-new");
    // On a start, a foreign profile's session is just a profile you don't have.
    expect(describeFailure(confirmed("INTERVIEW_NOT_FOUND"), practice)).toEqual(
      {
        message: "We couldn't find this Role Profile. Choose another one.",
        action: "setup",
      },
    );
  });

  it("has a message for every confirmed code, for both interview types", () => {
    for (const code of confirmedStartCodes)
      for (const context of [practice, recruitment]) {
        const problem = describeFailure(confirmed(code), context);
        expect(problem.message.length, code).toBeGreaterThan(10);
      }
  });

  it("speaks Vietnamese, with the price and balance as numbers, and keeps every code covered", () => {
    expect(describeFailure({ confirmed: false }, practice, "vi").message).toBe(
      "Chúng tôi không thể xác nhận cuộc phỏng vấn của bạn đã bắt đầu. Hãy thử lại. Bạn sẽ không bị trừ xu hai lần.",
    );
    expect(
      describeFailure(
        confirmed("INSUFFICIENT_BALANCE"),
        { ...practice, balance: 3 },
        "vi",
      ),
    ).toEqual({
      message:
        "Bạn cần 5 xu để bắt đầu một cuộc phỏng vấn luyện tập. Số dư của bạn là 3.",
      action: "add-coins",
    });
    expect(
      describeFailure(confirmed("INTERVIEW_CAPACITY_FULL"), recruitment, "vi")
        .message,
    ).toBe("Tất cả lượt phỏng vấn của tin tuyển dụng này đã được sử dụng hết.");
    expect(
      describeFailure(
        confirmed("VALIDATION_ERROR", "voice not found"),
        practice,
        "vi",
      ).message,
    ).toBe("Giọng nói này không còn khả dụng. Hãy chọn giọng khác.");
    for (const code of confirmedStartCodes)
      for (const context of [practice, recruitment]) {
        const problem = describeFailure(confirmed(code), context, "vi");
        expect(problem.message, code).not.toBe(
          describeFailure(confirmed(code), context).message,
        );
      }
  });
});
