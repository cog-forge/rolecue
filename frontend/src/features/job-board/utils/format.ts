import type { useTranslations } from "next-intl";
import type { Formatters } from "@/lib/i18n/formatters";
import type { BoardPosting, Level } from "../schemas/job-board-schema";

export type JobBoardTranslator = ReturnType<
  typeof useTranslations<"candidate.jobBoard">
>;

export function levelText(level: Level, t: JobBoardTranslator) {
  return t(`level.${level}`);
}

// Several API targets share one label; anything else is shown as words.
const targetKeys = {
  programming_language: "programming_language",
  framework: "framework",
  database: "database",
  tool: "tool",
  technology: "technology",
  technologies: "technology",
  domain_knowledge: "domain_knowledge",
  other: "other",
} as const;

// An unknown target still reads as words ("cloud_platform" → "Cloud platform").
export function targetLabel(target: string, t: JobBoardTranslator) {
  if (Object.hasOwn(targetKeys, target))
    return t(`target.${targetKeys[target as keyof typeof targetKeys]}`);
  const words = target.replaceAll("_", " ").trim();
  return words ? words[0].toUpperCase() + words.slice(1) : t("target.skill");
}

// The API stores salary as plain integers with no currency, so numbers are shown as stored.
export function salaryText(
  posting: Pick<BoardPosting, "salary_min" | "salary_max">,
  fmt: Formatters,
  t: JobBoardTranslator,
) {
  const { salary_min: min, salary_max: max } = posting;
  if (min !== null && max !== null)
    return min === max
      ? fmt.number(min)
      : `${fmt.number(min)} – ${fmt.number(max)}`;
  if (min !== null) return t("salary.from", { amount: fmt.number(min) });
  if (max !== null) return t("salary.upTo", { amount: fmt.number(max) });
  return null;
}

const DAY = 86_400_000;

// "Today", "Yesterday", "3 days ago"; older than four weeks shows the date.
export function postedText(
  createdAt: string,
  fmt: Formatters,
  t: JobBoardTranslator,
  now = Date.now(),
) {
  const published = new Date(createdAt);
  const current = new Date(now);
  const publicationDay = Date.UTC(
    published.getUTCFullYear(),
    published.getUTCMonth(),
    published.getUTCDate(),
  );
  const today = Date.UTC(
    current.getUTCFullYear(),
    current.getUTCMonth(),
    current.getUTCDate(),
  );
  const days = Math.round((today - publicationDay) / DAY);
  if (days <= 0) return t("posted.today");
  if (days < 28)
    return t("posted.relative", { when: fmt.relativeUnit(-days, "day") });
  return t("posted.date", { date: fmt.date(createdAt, "calendarDay") });
}

export function fullDate(createdAt: string, fmt: Formatters) {
  return fmt.date(createdAt, "calendarDay");
}

// UTC is explicit in the badge; historical rows without a publication timestamp are not new.
export function publicationDay(now = new Date()) {
  const from = new Date(now);
  from.setUTCHours(0, 0, 0, 0);
  return {
    from: from.toISOString(),
    to: new Date(from.getTime() + DAY).toISOString(),
  };
}
export function publicationText(
  posting: Pick<BoardPosting, "published_at" | "created_at">,
  fmt: Formatters,
  t: JobBoardTranslator,
) {
  return posting.published_at
    ? postedText(posting.published_at, fmt, t)
    : t("posted.created", { date: fullDate(posting.created_at, fmt) });
}
