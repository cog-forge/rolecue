import { z } from "zod";

type ValidationKey =
  | "textLength"
  | "entryLength"
  | "categoryRequired"
  | "categoryOnlySkills"
  | "requirementsMin";
// The form's `useTranslations("candidate.roleProfiles.validation")` satisfies this. The numbers are
// passed as strings so that English keeps "2000" and not "2,000".
export type RoleValidation = (
  key: ValidationKey,
  values?: { min: string; max: string },
) => string;

export const roleText = (v: RoleValidation, min: number, max: number) =>
  z
    .string()
    .trim()
    .refine(
      (value) => [...value].length >= min && [...value].length <= max,
      v("textLength", { min: String(min), max: String(max) }),
    );

// Request and response guards: their messages are never shown, so they stay as plain keys.
// Screens validate user input with the translated factories instead.
export const guardMessages: RoleValidation = (key) => key;
