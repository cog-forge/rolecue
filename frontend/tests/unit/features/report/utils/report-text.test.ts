import { describe, expect, it } from "vitest";
import { createTranslator } from "next-intl";
import { scoreText } from "@/features/report/utils/report-text";
import type { AppLocale } from "@/lib/i18n/config";
import { formattersFor, messagesFor } from "@tests/helpers/intl";

const tFor = (locale: AppLocale) =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "candidate.report",
  });

describe("report text", () => {
  it("labels every status and type the API can return", () => {
    const t = tFor("en");
    expect(t("status.in_progress")).toBe("In progress");
    expect(t("status.pending")).toBe("Evaluating");
    expect(t("status.completed")).toBe("Completed");
    expect(t("status.failed")).toBe("Evaluation failed");
    expect(t("type.practice")).toBe("Practice interview");
    expect(t("type.recruitment")).toBe("Recruitment interview");
  });

  it("shows the score out of 100, or says none exists yet", () => {
    const t = tFor("en");
    const fmt = formattersFor("en");
    expect(scoreText(72, fmt, t)).toBe("72.00 / 100");
    expect(scoreText(0, fmt, t)).toBe("0.00 / 100");
    expect(scoreText(62.5, fmt, t)).toBe("62.50 / 100");
    expect(scoreText(null, fmt, t)).toBe("No score yet");
  });

  it("says the same in Vietnamese", () => {
    const t = tFor("vi");
    const fmt = formattersFor("vi");
    expect(scoreText(72, fmt, t)).toBe("72,00 / 100");
    expect(scoreText(null, fmt, t)).toBe("Chưa có điểm");
    expect(t("status.pending")).toBe("Đang đánh giá");
  });
});
