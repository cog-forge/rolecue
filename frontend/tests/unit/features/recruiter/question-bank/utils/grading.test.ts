import { describe, expect, it } from "vitest";
import {
  answerWordCount,
  createGradingSchema,
} from "@/features/recruiter/question-bank/schemas/grading-schema";
describe("expected answer draft", () => {
  it("counts words across Unicode whitespace and enforces both server bounds", () => {
    expect(answerWordCount(" one\n two\tthree ")).toBe(3);
    const schema = createGradingSchema(10, 3, "Too long");
    expect(
      schema.safeParse({ rubric_id: "", expected_answer: "", reviewed: false })
        .success,
    ).toBe(true);
    expect(
      schema.safeParse({
        rubric_id: "",
        expected_answer: "😀".repeat(10),
        reviewed: true,
      }).success,
    ).toBe(true);
    expect(
      schema.safeParse({
        rubric_id: "",
        expected_answer: "a b c d",
        reviewed: true,
      }).success,
    ).toBe(false);
    expect(
      schema.safeParse({
        rubric_id: "",
        expected_answer: "界".repeat(11),
        reviewed: true,
      }).success,
    ).toBe(false);
  });
});
