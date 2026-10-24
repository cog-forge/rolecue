import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
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
  ])("maps %s to its message", (code, expected) => {
    expect(applicationErrorMessage(apiFailure(409, code, "server text"))).toBe(
      expected,
    );
  });

  it("falls back to the server message for an unknown code", () => {
    expect(
      applicationErrorMessage(
        apiFailure(500, "SOMETHING_NEW", "Server says no"),
      ),
    ).toBe("Server says no");
  });

  it("uses the caller fallback when the error carries no envelope", () => {
    expect(
      applicationErrorMessage(new Error("network down"), "Try again"),
    ).toBe("Try again");
    expect(applicationErrorMessage(new Error("network down"))).toBe(
      "Something went wrong. Try again.",
    );
  });
});
