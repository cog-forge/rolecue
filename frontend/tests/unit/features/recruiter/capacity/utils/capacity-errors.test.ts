import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import { capacityErrorMessage } from "@/features/recruiter/capacity/utils/capacity-errors";

function failure(code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status: 409,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

describe("capacityErrorMessage", () => {
  it.each([
    [
      "INSUFFICIENT_BALANCE",
      "Your wallet does not have enough coins for this many slots. Nothing was charged.",
    ],
    [
      "INTERVIEW_CAPACITY_ALREADY_FUNDED",
      "Interview slots are already funded for this posting. The current count is shown below.",
    ],
    [
      "INTAKE_NOT_OPEN",
      "Intake is no longer open for this posting, so this cannot be changed.",
    ],
  ])("maps %s to plain copy", (code, copy) => {
    expect(capacityErrorMessage(failure(code, "server text"))).toBe(copy);
  });

  it("shows the server's message for an unmapped code", () => {
    expect(
      capacityErrorMessage(failure("SOMETHING_ELSE", "Server says no.")),
    ).toBe("Server says no.");
  });

  it("uses the fallback when there is no server envelope", () => {
    expect(capacityErrorMessage(new Error("network"), "Try again later.")).toBe(
      "Try again later.",
    );
  });
});
