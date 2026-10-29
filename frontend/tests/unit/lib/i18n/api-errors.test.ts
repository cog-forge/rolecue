import { readFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import {
  apiErrorMessage,
  isApiErrorCode,
  readApiError,
} from "@/lib/i18n/api-errors";
import en from "@/lib/i18n/messages/en/errors.json";
import { errorsFor } from "@tests/helpers/intl";

function failure(status: number, code: string, message = "Backend English") {
  return new AxiosError("failed", "ERR_BAD_REQUEST", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

const offline = new AxiosError("Network Error", "ERR_NETWORK");

describe("readApiError", () => {
  it("reads the envelope without trusting its shape", () => {
    expect(readApiError(failure(409, "ALREADY_APPLIED", "raw"))).toEqual({
      status: 409,
      code: "ALREADY_APPLIED",
      message: "raw",
      network: false,
    });
    expect(readApiError(new Error("x")).code).toBeNull();
    expect(readApiError(offline).network).toBe(true);
  });
});

describe("apiErrorMessage", () => {
  it("shows the translation of a mapped code in each language, never the backend text", () => {
    const error = failure(402, "INSUFFICIENT_BALANCE", "wallet too low");
    expect(apiErrorMessage(errorsFor("en"), error)).toBe(
      "You don't have enough coins for this. Add coins and try again.",
    );
    expect(apiErrorMessage(errorsFor("vi"), error)).toBe(
      "Số xu trong ví không đủ. Hãy nạp thêm xu rồi thử lại.",
    );
    expect(apiErrorMessage(errorsFor("en"), error)).not.toContain("wallet");
  });

  it("uses the generic translated line for an unknown code, in each language", () => {
    const error = failure(400, "SOMETHING_NEW", "Backend English");
    expect(apiErrorMessage(errorsFor("en"), error)).toBe(
      "Something went wrong. Try again.",
    );
    expect(apiErrorMessage(errorsFor("vi"), error)).toBe(
      "Đã xảy ra lỗi. Vui lòng thử lại.",
    );
  });

  it("prefers the screen's own copy over the generic line, but not over a specific code", () => {
    const own = "We couldn't save your profile.";
    expect(
      apiErrorMessage(errorsFor("en"), failure(400, "SOMETHING_NEW"), own),
    ).toBe(own);
    expect(
      apiErrorMessage(errorsFor("en"), failure(500, "INTERNAL_ERROR"), own),
    ).toBe(own);
    expect(
      apiErrorMessage(errorsFor("vi"), failure(409, "ALREADY_APPLIED"), own),
    ).toBe("Bạn đã ứng tuyển vào tin tuyển dụng này.");
  });

  it("explains an unreachable server and rate limiting", () => {
    expect(apiErrorMessage(errorsFor("en"), offline)).toBe(
      "We couldn't reach RoleCue. Check your connection and try again.",
    );
    expect(apiErrorMessage(errorsFor("vi"), offline)).toBe(
      "Không kết nối được tới RoleCue. Hãy kiểm tra mạng rồi thử lại.",
    );
    expect(apiErrorMessage(errorsFor("en"), failure(429, "RATE_LIMITED"))).toBe(
      "Too many requests. Please wait a moment before trying again.",
    );
  });

  it("falls back to the generic line for something that is not an API error", () => {
    expect(apiErrorMessage(errorsFor("vi"), new Error("boom"))).toBe(
      "Đã xảy ra lỗi. Vui lòng thử lại.",
    );
  });
});

describe("api error catalog", () => {
  // Tests run from frontend/; the Go API sits beside it in the monorepo.
  const goCodes = resolve(process.cwd(), "../api/pkg/apperror/apperr.go");

  it.skipIf(!existsSync(goCodes))(
    "has a translation for every code the Go API defines",
    () => {
      const defined = [
        ...readFileSync(goCodes, "utf8").matchAll(/Code = "([A-Z_]+)"/g),
      ].map((match) => match[1]);
      expect(defined.length).toBeGreaterThan(50);
      expect(defined.filter((code) => !isApiErrorCode(code))).toEqual([]);
    },
  );

  it("has no empty message in either language", () => {
    for (const locale of ["en", "vi"] as const) {
      const t = errorsFor(locale);
      for (const code of Object.keys(en.api))
        expect(
          t(`api.${code as keyof typeof en.api}`).trim().length,
        ).toBeGreaterThan(0);
    }
  });
});
