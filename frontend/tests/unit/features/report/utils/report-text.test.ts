import { describe, expect, it } from "vitest";
import {
  reportStatusLabel,
  reportTypeLabel,
  scoreText,
} from "@/features/report/utils/report-text";

describe("report text", () => {
  it("labels every status the API can return", () => {
    expect(reportStatusLabel).toEqual({
      in_progress: "In progress",
      pending: "Evaluating",
      completed: "Completed",
      failed: "Evaluation failed",
    });
  });

  it("labels both interview types", () => {
    expect(reportTypeLabel.practice).toBe("Practice interview");
    expect(reportTypeLabel.recruitment).toBe("Recruitment interview");
  });

  it("shows the score out of 100, or says none exists yet", () => {
    expect(scoreText(72)).toBe("72 / 100");
    expect(scoreText(0)).toBe("0 / 100");
    expect(scoreText(null)).toBe("No score yet");
  });
});
