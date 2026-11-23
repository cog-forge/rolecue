import {
  proficiencyLevels,
  type RoleRequirement,
} from "../schemas/role-profile-schema";

export function isKnownTarget(target: string) {
  return proficiencyLevels.some((value) => value === target);
}
// Targets saved before proficiency levels existed are free text; keep them as written.
export function isCustomTarget({ name, target }: RoleRequirement) {
  return Boolean(target) && !isKnownTarget(target) && target !== name;
}
