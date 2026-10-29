import type { BoardPosting, Level } from "../schemas/job-board-schema";

const numberFormat = new Intl.NumberFormat("en");
const relativeFormat = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
const dateFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});

export const levelText: Record<Level, string> = {
  intern: "Intern",
  junior: "Junior",
  mid: "Mid-level",
  senior: "Senior",
  lead: "Lead",
};

const targetText: Record<string, string> = {
  programming_language: "Programming language",
  framework: "Framework",
  database: "Database",
  tool: "Tool",
  technology: "Technology",
  technologies: "Technology",
  domain_knowledge: "Domain knowledge",
  other: "Other skill",
};

// An unknown target still reads as words ("cloud_platform" → "Cloud platform").
export function targetLabel(target: string) {
  const known = targetText[target];
  if (known) return known;
  const words = target.replaceAll("_", " ").trim();
  return words ? words[0].toUpperCase() + words.slice(1) : "Skill";
}

// The API stores salary as plain integers with no currency, so numbers are shown as stored.
export function salaryText(
  posting: Pick<BoardPosting, "salary_min" | "salary_max">,
) {
  const { salary_min: min, salary_max: max } = posting;
  if (min !== null && max !== null)
    return min === max
      ? numberFormat.format(min)
      : `${numberFormat.format(min)} – ${numberFormat.format(max)}`;
  if (min !== null) return `From ${numberFormat.format(min)}`;
  if (max !== null) return `Up to ${numberFormat.format(max)}`;
  return null;
}

const DAY = 86_400_000;

// "Today", "Yesterday", "3 days ago"; older than four weeks shows the date.
export function postedText(createdAt: string, now = Date.now()) {
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
  if (days <= 0) return "Posted today";
  if (days < 28) return `Posted ${relativeFormat.format(-days, "day")}`;
  return `Posted ${dateFormat.format(new Date(createdAt))}`;
}

export function fullDate(createdAt: string) {
  return dateFormat.format(new Date(createdAt));
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
) {
  return posting.published_at
    ? postedText(posting.published_at)
    : `Created ${fullDate(posting.created_at)}`;
}
