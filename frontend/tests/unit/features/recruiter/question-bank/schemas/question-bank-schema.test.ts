import { describe, expect, it } from "vitest";
import {
  bankEnvelopeSchema,
  questionFormSchema,
  questionLength,
} from "@/features/recruiter/question-bank/schemas/question-bank-schema";

const bank = {
  questions: [{ id: "44444444-4444-4444-8444-444444444444", content: "Explain Go channels." }],
  posting_status: "pending",
  editable: true,
  max_question_length: 1000,
  generation_size: 10,
};

describe("question bank schema", () => {
  it("accepts the Go list contract", () => {
    expect(bankEnvelopeSchema.parse({ success: true, data: bank }).data.editable).toBe(true);
  });

  it("rejects contract drift", () => {
    expect(() =>
      bankEnvelopeSchema.parse({ success: true, data: { ...bank, posting_status: "draft" } }),
    ).toThrow();
    expect(() =>
      bankEnvelopeSchema.parse({ success: true, data: { questions: bank.questions } }),
    ).toThrow();
  });

  it("counts code points after trimming, like Go runes", () => {
    expect(questionLength("  😀ab  ")).toBe(3);
  });

  it("uses the server limit", () => {
    const schema = questionFormSchema(5);
    expect(schema.safeParse({ content: "   " }).success).toBe(false);
    expect(schema.safeParse({ content: " 12345 " }).success).toBe(true);
    const tooLong = schema.safeParse({ content: "123456" });
    expect(tooLong.success).toBe(false);
    expect(tooLong.error?.issues[0]?.message).toBe("Keep the question to 5 characters or fewer.");
  });
});
