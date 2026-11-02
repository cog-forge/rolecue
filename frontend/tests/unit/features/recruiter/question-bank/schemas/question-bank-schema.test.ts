import { describe, expect, it } from "vitest";
import { createTranslator } from "next-intl";
import { messagesFor } from "@tests/helpers/intl";
import type { AppLocale } from "@/lib/i18n/config";
import {
  bankEnvelopeSchema,
  createQuestionFormSchema,
  questionLength,
} from "@/features/recruiter/question-bank/schemas/question-bank-schema";

const bank = {
  questions: [
    {
      id: "44444444-4444-4444-8444-444444444444",
      content: "Explain Go channels.",
    },
  ],
  posting_status: "pending",
  editable: true,
  max_question_length: 1000,
  review_submitted: false,
  generation_size: 10,
};

const validationFor = (locale: AppLocale = "en") =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "recruiter.questionBank.validation",
  });

describe("question bank schema", () => {
  it("accepts the Go list contract", () => {
    expect(
      bankEnvelopeSchema.parse({ success: true, data: bank }).data.editable,
    ).toBe(true);
  });

  it("rejects contract drift", () => {
    expect(() =>
      bankEnvelopeSchema.parse({
        success: true,
        data: { ...bank, posting_status: "draft" },
      }),
    ).toThrow();
    expect(() =>
      bankEnvelopeSchema.parse({
        success: true,
        data: { questions: bank.questions },
      }),
    ).toThrow();
  });

  it("counts code points after trimming, like Go runes", () => {
    expect(questionLength("  😀ab  ")).toBe(3);
  });

  it("uses the server limit", () => {
    const schema = createQuestionFormSchema(5, validationFor());
    expect(schema.safeParse({ content: "   " }).success).toBe(false);
    expect(schema.safeParse({ content: " 12345 " }).success).toBe(true);
    const tooLong = schema.safeParse({ content: "123456" });
    expect(tooLong.success).toBe(false);
    expect(tooLong.error?.issues[0]?.message).toBe(
      "Keep the question to 5 characters or fewer.",
    );
  });

  it("words the limit in Vietnamese and groups digits like the language", () => {
    const schema = createQuestionFormSchema(1000, validationFor("vi"));
    const tooLong = schema.safeParse({ content: "a".repeat(1001) });
    expect(tooLong.error?.issues[0]?.message).toBe(
      "Câu hỏi tối đa 1.000 ký tự.",
    );
    expect(schema.safeParse({ content: " " }).error?.issues[0]?.message).toBe(
      "Hãy viết câu hỏi trước.",
    );
    const english = createQuestionFormSchema(1000, validationFor());
    expect(
      english.safeParse({ content: "a".repeat(1001) }).error?.issues[0]
        ?.message,
    ).toBe("Keep the question to 1,000 characters or fewer.");
  });
});
