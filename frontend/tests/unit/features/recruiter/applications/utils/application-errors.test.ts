import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import { createTranslator } from "next-intl";
import { errorsFor, messagesFor } from "@tests/helpers/intl";
import type { AppLocale } from "@/lib/i18n/config";
import { applicationErrorMessage } from "@/features/recruiter/applications/utils/application-errors";

function apiFailure(status: number, code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function messageFor(
  error: unknown,
  locale: AppLocale = "en",
  fallback?: string,
) {
  const applicationT = createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "recruiter.applications.errors",
  });
  return applicationErrorMessage(
    error,
    errorsFor(locale),
    applicationT,
    fallback,
  );
}

describe("applicationErrorMessage", () => {
  it.each([
    [
      "APPLICATION_NOT_PENDING",
      "This application has already been decided. Refresh to see its current status.",
    ],
    ["INTAKE_NOT_OPEN", "This posting is no longer accepting applications."],
    [
      "INTERVIEW_CAPACITY_FULL",
      "All interview slots for this posting are already taken.",
    ],
    [
      "INTERVIEW_CAPACITY_NOT_FUNDED",
      "Interview capacity is not funded for this posting.",
    ],
    ["APPLICATION_NOT_FOUND", "This application could not be found."],
    ["JOB_POSTING_NOT_FOUND", "This job posting could not be found."],
    [
      "APPLICATION_NOT_INTERVIEWED",
      "This application is not waiting for a final decision. Refresh to see its current status.",
    ],
    ["FINAL_REVIEW_LOCKED", "Final decisions are not open yet."],
    [
      "INTERVIEW_NOT_COMPLETED",
      "The transcript is available once the interview is completed.",
    ],
  ])("maps %s to its message", (code, expected) => {
    expect(messageFor(apiFailure(409, code, "server text"))).toBe(expected);
  });

  it("maps the same codes to Vietnamese", () => {
    expect(
      messageFor(apiFailure(409, "INTAKE_NOT_OPEN", "server text"), "vi"),
    ).toBe("Tin tuyển dụng này không còn nhận đơn ứng tuyển.");
    expect(
      messageFor(apiFailure(409, "FINAL_REVIEW_LOCKED", "server text"), "vi"),
    ).toBe("Chưa thể đưa ra quyết định cuối cùng.");
  });

  it("uses the shared translated message for a code the shared catalog knows", () => {
    expect(messageFor(apiFailure(403, "FORBIDDEN", "server text"))).toBe(
      "You don't have permission to do that.",
    );
    expect(messageFor(apiFailure(403, "FORBIDDEN", "server text"), "vi")).toBe(
      errorsFor("vi")("api.FORBIDDEN"),
    );
  });

  it("never shows the server message for an unknown code", () => {
    const failure = apiFailure(500, "SOMETHING_NEW", "Server says no");
    expect(messageFor(failure)).toBe("Something went wrong. Try again.");
    expect(messageFor(failure, "en", "Could not approve")).toBe(
      "Could not approve",
    );
    expect(messageFor(failure, "vi")).not.toContain("Server says no");
  });

  it("prefers the caller's wording over a non-specific code", () => {
    expect(
      messageFor(
        apiFailure(500, "INTERNAL_ERROR", "boom"),
        "vi",
        "Không thể duyệt CV này.",
      ),
    ).toBe("Không thể duyệt CV này.");
  });

  it("uses the caller fallback when the error carries no envelope", () => {
    expect(messageFor(new Error("network down"), "en", "Try again")).toBe(
      "Try again",
    );
    expect(messageFor(new Error("network down"))).toBe(
      "Something went wrong. Try again.",
    );
  });
});
