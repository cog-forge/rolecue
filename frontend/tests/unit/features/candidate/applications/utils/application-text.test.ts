import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import {
  canRetryApply,
  applyErrorMessage,
} from "@/features/candidate/applications/utils/api-error";
import {
  isDeadlinePassed,
  timeLeftText,
} from "@/features/candidate/applications/utils/application-text";
import { applicationHref } from "@/features/candidate/applications/utils/application-links";

const now = Date.parse("2026-10-25T08:00:00Z");

function apiFailure(status: number, code: string, message?: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

describe("timeLeftText", () => {
  it.each([
    ["2026-10-25T11:20:00Z", "3 h 20 min left"],
    ["2026-10-25T08:12:00Z", "12 min left"],
    ["2026-10-25T08:00:30Z", "Less than a minute left"],
    ["2026-10-25T07:59:00Z", "Deadline passed"],
  ])("reads %s as %s", (deadline, text) => {
    expect(timeLeftText(deadline, now)).toBe(text);
  });

  it("marks a deadline at exactly now as passed", () => {
    expect(isDeadlinePassed("2026-10-25T08:00:00Z", now)).toBe(true);
    expect(isDeadlinePassed("2026-10-25T08:00:01Z", now)).toBe(false);
  });
});

describe("application error copy", () => {
  it("maps known codes to fixed copy and keeps retries for transient failures", () => {
    expect(applyErrorMessage(apiFailure(409, "ALREADY_APPLIED"))).toBe(
      "You already applied to this posting.",
    );
    expect(canRetryApply(apiFailure(409, "ALREADY_APPLIED"))).toBe(false);
    expect(canRetryApply(apiFailure(409, "INTAKE_NOT_OPEN"))).toBe(false);
    expect(canRetryApply(apiFailure(500, "INTERNAL_ERROR"))).toBe(true);
    expect(canRetryApply(new Error("network"))).toBe(true);
  });

  it("uses the server's words for a validation error and a generic line otherwise", () => {
    expect(
      applyErrorMessage(apiFailure(400, "VALIDATION_ERROR", "Not a PDF.")),
    ).toBe("Not a PDF.");
    expect(applyErrorMessage(new Error("boom"))).toBe(
      "We couldn't upload your CV. Try again.",
    );
  });
});

describe("applicationHref", () => {
  it("builds the detail path under the applications route and encodes the id", () => {
    expect(applicationHref("a/b c")).toBe("/applications/a%2Fb%20c");
  });
});
