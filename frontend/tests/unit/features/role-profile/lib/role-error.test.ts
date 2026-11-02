import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import { createTranslator } from "next-intl";
import { errorsFor, messagesFor } from "@tests/helpers/intl";
import { roleErrorMessage } from "@/features/role-profile/lib/role-error";

function failure(status: number, code: string) {
  return new AxiosError("x", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
    data: { success: false, error: { code, message: "Server English text" } },
  });
}
const roleT = (locale: "en" | "vi") =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "candidate.roleProfiles.errors",
  });

describe("roleErrorMessage", () => {
  it("maps codes to the active language and never shows the server text", () => {
    const vi = (error: unknown) =>
      roleErrorMessage(roleT("vi"), errorsFor("vi"), error);
    expect(vi(failure(503, "PDF_UNAVAILABLE"))).toContain(
      "Hiện không thể đọc PDF",
    );
    expect(vi(failure(502, "EXTRACTION_FAILED"))).toContain(
      "Phân tích không thành công",
    );
    expect(vi(failure(409, "ROLE_PROFILE_CONFLICT"))).toContain(
      "chưa sẵn sàng",
    );
    expect(vi(failure(409, "SOMETHING_NEW"))).toContain("chưa sẵn sàng");
    expect(vi(failure(400, "INVALID_PDF"))).not.toContain(
      "Server English text",
    );
    expect(vi(failure(400, "INVALID_PDF"))).toBe(
      errorsFor("vi")("api.INVALID_PDF"),
    );
    expect(vi(new Error("anything"))).toBe(
      "Chúng tôi không thể hoàn tất bước đó. Vui lòng thử lại.",
    );
    expect(vi(new AxiosError("c", "ERR_CANCELED"))).toContain("Đã hủy");
  });

  it("keeps the English fallbacks", () => {
    const en = (error: unknown) =>
      roleErrorMessage(roleT("en"), errorsFor("en"), error);
    expect(en(new Error("anything"))).toBe(
      "We couldn't complete that step. Please try again.",
    );
    expect(en(failure(409, "SOMETHING_NEW"))).toBe(
      "This job description is not ready for that action. Refresh and try again.",
    );
    expect(en(new AxiosError("c", "ERR_CANCELED"))).toBe(
      "Canceled. You can retry or open your saved draft.",
    );
  });
});
