import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import { errorsFor } from "@tests/helpers/intl";
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

const te = errorsFor("en");

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
    expect(capacityErrorMessage(failure(code, "server text"), te)).toBe(copy);
  });

  it("never shows the server's text: a catalogued code gets its shared translation", () => {
    expect(
      capacityErrorMessage(
        failure("INTERVIEW_CAPACITY_FULL", "Server says no."),
        te,
      ),
    ).toBe("All interview slots for this posting are taken.");
    expect(
      capacityErrorMessage(
        failure("INTERVIEW_CAPACITY_FULL", "Server says no."),
        errorsFor("vi"),
      ),
    ).toBe("Các lượt phỏng vấn của tin tuyển dụng này đã được dùng hết.");
  });

  it("falls back to the screen's copy for a code nobody mapped", () => {
    expect(
      capacityErrorMessage(failure("SOMETHING_ELSE", "Server says no."), te),
    ).toBe("Something went wrong. Try again.");
  });

  it("uses the fallback when there is no server envelope", () => {
    expect(
      capacityErrorMessage(new Error("network"), te, "Try again later."),
    ).toBe("Try again later.");
  });
});
