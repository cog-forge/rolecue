import type { useTranslations } from "next-intl";
import type { RoleRequirement } from "../api/contract";

export type RoleLabelTranslator = ReturnType<
  typeof useTranslations<"candidate.roleProfiles.labels">
>;

const statusKeys = {
  draft: "status.draft",
  analyzing: "status.analyzing",
  review: "status.review",
  confirmed: "status.confirmed",
} as const;
const seniorityKeys = {
  intern: "seniority.intern",
  junior: "seniority.junior",
  mid: "seniority.mid",
  senior: "seniority.senior",
  lead: "seniority.lead",
} as const;
const categoryKeys = {
  programming_language: "category.programming_language",
  framework: "category.framework",
  database: "category.database",
  tool: "category.tool",
  technology: "category.technology",
  other: "category.other",
} as const;
const kindKeys = {
  skill: "kind.skill",
  technology: "kind.technology",
  domain_knowledge: "kind.domain_knowledge",
} as const;
const priorityKeys = {
  unspecified: "priority.unspecified",
  required: "priority.required",
  preferred: "priority.preferred",
} as const;
// Targets are stored as these English words; only their label is translated.
const proficiencyKeys = {
  Unspecified: "proficiency.Unspecified",
  New: "proficiency.New",
  Familiar: "proficiency.Familiar",
  Proficient: "proficiency.Proficient",
  Expert: "proficiency.Expert",
} as const;

function has<T extends object>(
  map: T,
  value: string,
): value is keyof T & string {
  return Object.hasOwn(map, value);
}

// Backend enums are chrome labels; an unknown value is shown as received.
export function statusLabel(t: RoleLabelTranslator, status: string) {
  return has(statusKeys, status) ? t(statusKeys[status]) : status;
}
export function seniorityLabel(t: RoleLabelTranslator, level: string) {
  return has(seniorityKeys, level) ? t(seniorityKeys[level]) : level;
}
export function categoryLabel(t: RoleLabelTranslator, category: string) {
  return has(categoryKeys, category) ? t(categoryKeys[category]) : category;
}
export function kindLabel(t: RoleLabelTranslator, kind: string) {
  return has(kindKeys, kind) ? t(kindKeys[kind]) : kind;
}
export function priorityLabel(t: RoleLabelTranslator, priority: string) {
  return has(priorityKeys, priority) ? t(priorityKeys[priority]) : priority;
}
export function proficiencyLabel(t: RoleLabelTranslator, level: string) {
  return has(proficiencyKeys, level) ? t(proficiencyKeys[level]) : level;
}

export function requirementLabel(
  t: RoleLabelTranslator,
  { kind, category, type }: RoleRequirement,
) {
  const classification =
    kind === "skill" && category
      ? categoryLabel(t, category)
      : kind === "technology"
        ? kindLabel(t, "technology")
        : kindLabel(t, "domain_knowledge");
  if (!type) return classification;
  return t("classified", {
    classification,
    priority: priorityLabel(t, type),
  });
}
