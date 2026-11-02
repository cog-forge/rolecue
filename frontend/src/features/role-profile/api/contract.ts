import { z } from "zod";

export const senioritySchema = z.enum([
  "intern",
  "junior",
  "mid",
  "senior",
  "lead",
]);
export const categorySchema = z.enum([
  "programming_language",
  "framework",
  "database",
  "tool",
  "technology",
  "other",
]);
export const statusSchema = z.enum([
  "draft",
  "analyzing",
  "review",
  "confirmed",
]);
export const proficiencyLevels = [
  "Unspecified",
  "New",
  "Familiar",
  "Proficient",
  "Expert",
] as const;

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
const text = (v: RoleValidation, min: number, max: number) =>
  z
    .string()
    .trim()
    .refine(
      (value) => [...value].length >= min && [...value].length <= max,
      v("textLength", { min: String(min), max: String(max) }),
    );
export function createRequirementSchema(v: RoleValidation) {
  return z
    .object({
      name: text(v, 1, 200),
      target: text(v, 1, 2000),
      kind: z.enum(["skill", "technology", "domain_knowledge"]),
      category: categorySchema.nullable(),
      type: z.enum(["required", "preferred"]).nullable(),
      evidenceText: z.string().nullable(),
    })
    .strict()
    .superRefine((value, context) => {
      if (value.kind === "skill" && !value.category)
        context.addIssue({
          code: "custom",
          path: ["category"],
          message: v("categoryRequired"),
        });
      if (value.kind !== "skill" && value.category)
        context.addIssue({
          code: "custom",
          path: ["category"],
          message: v("categoryOnlySkills"),
        });
    });
}
export function createReviewSchema(v: RoleValidation) {
  return z.object({
    title: text(v, 1, 200),
    seniorityLevel: senioritySchema.nullable(),
    requirements: z
      .array(createRequirementSchema(v))
      .min(1, v("requirementsMin"))
      .max(200),
    refinementNote: text(v, 0, 2000),
  });
}
// Request and response guards: their messages are never shown, so they stay as plain keys.
// Screens validate user input with the translated factories above.
const guardMessages: RoleValidation = (key) => key;
export const requirementSchema = createRequirementSchema(guardMessages);
export const reviewSchema = createReviewSchema(guardMessages);
export const roleProfileSchema = z
  .object({
    title: z.string(),
    seniorityLevel: senioritySchema.nullable(),
    requirements: z.array(requirementSchema),
    refinementNote: z.string(),
    id: z.uuid(),
    rawText: z.string(),
    hasOriginalDocument: z.boolean(),
    status: statusSchema,
    createdAt: z.string(),
  })
  .strict();
export const profileEnvelopeSchema = z.object({
  success: z.literal(true),
  data: roleProfileSchema,
});

export const maxPDFBytes = 5 * 1024 * 1024;
export const signedRequestSchema = z
  .object({
    URL: z
      .url()
      .refine((value) => ["http:", "https:"].includes(new URL(value).protocol)),
    Method: z.enum(["PUT", "GET"]),
    Headers: z.record(z.string(), z.string()),
    ExpiresAt: z.string(),
  })
  .strict();
export const uploadTicketSchema = z
  .object({
    uploadId: z.uuid(),
    request: signedRequestSchema,
  })
  .strict();
export const uploadEnvelopeSchema = z.object({
  success: z.literal(true),
  data: uploadTicketSchema,
});
export const documentEnvelopeSchema = z.object({
  success: z.literal(true),
  data: signedRequestSchema,
});
export type UploadTicket = z.infer<typeof uploadTicketSchema>;
export type PdfErrorTranslator = (
  key: "notPdf" | "empty" | "tooLarge",
) => string;
export function pdfFileError(file: File, t: PdfErrorTranslator) {
  if (
    !file.name.toLowerCase().endsWith(".pdf") ||
    (file.type && file.type !== "application/pdf")
  )
    return t("notPdf");
  if (file.size === 0) return t("empty");
  if (file.size > maxPDFBytes) return t("tooLarge");
  return null;
}
export const summarySchema = z
  .object({
    id: z.uuid(),
    title: z.string(),
    seniorityLevel: senioritySchema.nullable(),
    status: statusSchema,
    createdAt: z.string(),
  })
  .strict();
export const listEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(summarySchema),
    pagination: z.object({
      limit: z.number(),
      offset: z.number(),
      total: z.number(),
    }),
  }),
});
export function normalizeRoleText(raw: string) {
  const lines = raw
    .replace(/\r\n?/g, "\n")
    .split("\n")
    .map((line) => line.trim().replace(/[^\S\n]+/gu, " "));
  return lines
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}
export function createEntrySchema(v: RoleValidation) {
  return z.object({
    rawText: z.string().refine((value) => {
      const length = [...normalizeRoleText(value)].length;
      return length >= 100 && length <= 20_000;
    }, v("entryLength")),
  });
}
export const entrySchema = createEntrySchema(guardMessages);
export type RoleProfile = z.infer<typeof roleProfileSchema>;
export type RoleReview = z.infer<typeof reviewSchema>;
export type RoleRequirement = z.infer<typeof requirementSchema>;
export type RoleSummary = z.infer<typeof summarySchema>;
export type EntryValues = z.infer<typeof entrySchema>;
export function isKnownTarget(target: string) {
  return proficiencyLevels.some((value) => value === target);
}
// Targets saved before proficiency levels existed are free text; keep them as written.
export function isCustomTarget({ name, target }: RoleRequirement) {
  return Boolean(target) && !isKnownTarget(target) && target !== name;
}
