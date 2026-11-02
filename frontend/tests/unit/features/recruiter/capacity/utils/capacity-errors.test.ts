import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import { createTranslator } from "next-intl";
import { errorsFor, messagesFor } from "@tests/helpers/intl";
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

const capacityFor = (locale: "en" | "vi") =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "recruiter.capacity",
  });
const te = errorsFor("en");
const tc = capacityFor("en");

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
    expect(capacityErrorMessage(failure(code, "server text"), te, tc)).toBe(
      copy,
    );
  });

  it("words the screen's codes in Vietnamese", () => {
    expect(
      capacityErrorMessage(
        failure("INSUFFICIENT_BALANCE", "server text"),
        errorsFor("vi"),
        capacityFor("vi"),
      ),
    ).toBe("Ví của bạn không đủ xu cho số lượt này. Chưa có khoản nào bị trừ.");
  });

  it("never shows the server's text: a catalogued code gets its shared translation", () => {
    expect(
      capacityErrorMessage(
        failure("INTERVIEW_CAPACITY_FULL", "Server says no."),
        te,
        tc,
      ),
    ).toBe("All interview slots for this posting are taken.");
    expect(
      capacityErrorMessage(
        failure("INTERVIEW_CAPACITY_FULL", "Server says no."),
        errorsFor("vi"),
        capacityFor("vi"),
      ),
    ).toBe("Các lượt phỏng vấn của tin tuyển dụng này đã được dùng hết.");
  });

  it("falls back to the screen's copy for a code nobody mapped", () => {
    expect(
      capacityErrorMessage(
        failure("SOMETHING_ELSE", "Server says no."),
        te,
        tc,
      ),
    ).toBe("Something went wrong. Try again.");
    expect(
      capacityErrorMessage(
        failure("SOMETHING_ELSE", "Server says no."),
        errorsFor("vi"),
        capacityFor("vi"),
      ),
    ).toBe("Đã xảy ra lỗi. Hãy thử lại.");
  });

  it("uses the fallback when there is no server envelope", () => {
    expect(
      capacityErrorMessage(new Error("network"), te, tc, "Try again later."),
    ).toBe("Try again later.");
  });
});
