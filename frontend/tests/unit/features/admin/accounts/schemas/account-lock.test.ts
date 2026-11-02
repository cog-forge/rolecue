import { describe, expect, it } from "vitest";
import { createTranslator } from "next-intl";
import { messagesFor } from "@tests/helpers/intl";
import type { AppLocale } from "@/lib/i18n/config";
import {
  banRequestSchema,
  createLockFormSchema,
  lockRequest,
} from "@/features/admin/accounts/schemas/account-lock";

const validationOf = (locale: AppLocale = "en") =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "admin.accounts.validation",
  });
const v = validationOf();
const userId = "11111111-1111-4111-8111-111111111111";
const now = new Date("2026-10-10T10:00:00Z").getTime();
const values = {
  reason: "  Repeated abuse  ",
  indefinite: false,
  duration: "3600",
  expiresAt: "",
};

describe("account lock policy", () => {
  it("trims the reason and converts a preset to seconds", () => {
    const parsed = createLockFormSchema(v, () => now).parse(values);
    expect(lockRequest(userId, parsed, v, now)).toEqual({
      userId,
      banReason: "Repeated abuse",
      banExpiresIn: 3600,
    });
  });
  it("omits expiry entirely for indefinite locks, ignoring inactive date inputs", () => {
    const parsed = createLockFormSchema(v, () => now).parse({
      ...values,
      indefinite: true,
      duration: "custom",
      expiresAt: "invalid",
    });
    expect(lockRequest(userId, parsed, v, now)).toEqual({
      userId,
      banReason: "Repeated abuse",
    });
  });
  it("converts custom local date/time to remaining seconds at submit", () => {
    const local = new Date(2026, 9, 11, 12, 0);
    const submittedAt = local.getTime() - 90_500;
    const parsed = createLockFormSchema(v, () => submittedAt).parse({
      ...values,
      duration: "custom",
      expiresAt: "2026-10-11T12:00",
    });
    expect(lockRequest(userId, parsed, v, submittedAt).banExpiresIn).toBe(91);
  });
  it.each([
    { ...values, reason: "   " },
    { ...values, reason: "a".repeat(501) },
    { ...values, duration: "" },
    { ...values, duration: "custom", expiresAt: "" },
    { ...values, duration: "custom", expiresAt: "2020-01-01T12:00" },
    { ...values, duration: "custom", expiresAt: "2026-02-30T12:00" },
  ])("rejects invalid form values %j", (input) => {
    expect(createLockFormSchema(v, () => now).safeParse(input).success).toBe(
      false,
    );
  });
  it("rechecks a custom date if it expired while the dialog was open", () => {
    const expires = new Date(2026, 9, 11, 12).getTime();
    expect(() =>
      lockRequest(
        userId,
        { ...values, duration: "custom", expiresAt: "2026-10-11T12:00" },
        v,
        expires + 1,
      ),
    ).toThrow();
  });
  it("reports field errors in the active language", () => {
    const messages = (locale: AppLocale) => {
      const result = createLockFormSchema(
        validationOf(locale),
        () => now,
      ).safeParse({
        reason: " ",
        indefinite: false,
        duration: "",
        expiresAt: "",
      });
      return result.success
        ? []
        : result.error.issues.map((issue) => issue.message);
    };
    expect(messages("en")).toEqual([
      "Enter a reason for locking this account.",
      "Choose a lock duration.",
    ]);
    expect(messages("vi")).toEqual([
      "Nhập lý do khóa tài khoản này.",
      "Hãy chọn thời hạn khóa.",
    ]);
  });
  it("words the reason limit and a past expiry in both languages", () => {
    for (const [locale, reason, expiry] of [
      [
        "en",
        "Use 500 characters or fewer.",
        "Choose a valid date and time in the future.",
      ],
      [
        "vi",
        "Dùng tối đa 500 ký tự.",
        "Hãy chọn ngày và giờ hợp lệ trong tương lai.",
      ],
    ] as const) {
      const schema = createLockFormSchema(validationOf(locale), () => now);
      const tooLong = schema.safeParse({ ...values, reason: "a".repeat(501) });
      const past = schema.safeParse({
        ...values,
        duration: "custom",
        expiresAt: "2020-01-01T12:00",
      });
      expect(tooLong.error?.issues[0]?.message).toBe(reason);
      expect(past.error?.issues[0]?.message).toBe(expiry);
    }
  });
  it.each([0, -1, 0.5, Number.POSITIVE_INFINITY, Number.NaN, 9e15])(
    "rejects an unsafe server duration: %s",
    (banExpiresIn) => {
      expect(
        banRequestSchema.safeParse({ userId, banReason: "Abuse", banExpiresIn })
          .success,
      ).toBe(false);
    },
  );
  it("accepts arbitrary positive custom seconds and missing duration for indefinite", () => {
    expect(
      banRequestSchema.parse({
        userId,
        banReason: " Abuse ",
        banExpiresIn: 12_345,
      }).banReason,
    ).toBe("Abuse");
    expect(
      banRequestSchema.safeParse({ userId, banReason: "Abuse" }).success,
    ).toBe(true);
    expect(
      banRequestSchema.safeParse({ userId: "invalid", banReason: "Abuse" })
        .success,
    ).toBe(false);
    expect(banRequestSchema.safeParse({ userId, banReason: " " }).success).toBe(
      false,
    );
  });
});
