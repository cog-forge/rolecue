import type { useTranslations } from "next-intl";
import type { Formatters } from "@/lib/i18n/formatters";

export type ReportTranslator = ReturnType<
  typeof useTranslations<"candidate.report">
>;

export function formatInterviewTime(iso: string, fmt: Formatters) {
  return fmt.date(iso, "localDateTime");
}

export function scoreText(
  score: number | null,
  fmt: Formatters,
  t: ReportTranslator,
) {
  return score === null
    ? t("noScore")
    : t("score", { score: fmt.number(score) });
}
