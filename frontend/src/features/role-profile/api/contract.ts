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
const text = (min: number, max: number) =>
  z
    .string()
    .trim()
    .refine(
      (value) => [...value].length >= min && [...value].length <= max,
      `Use ${min}–${max} characters.`,
    );
export const requirementSchema = z
  .object({
    name: text(1, 200),
    target: text(1, 2000),
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
        message: "Select a skill category.",
      });
    if (value.kind !== "skill" && value.category)
      context.addIssue({
        code: "custom",
        path: ["category"],
        message: "Only skills have a category.",
      });
  });
export const reviewSchema = z.object({
  title: text(1, 200),
  seniorityLevel: senioritySchema.nullable(),
  requirements: z
    .array(requirementSchema)
    .min(1, "Keep at least one technical requirement.")
    .max(200),
  refinementNote: text(0, 2000),
});
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
export function pdfFileError(file: File) {
  if (
    !file.name.toLowerCase().endsWith(".pdf") ||
    (file.type && file.type !== "application/pdf")
  )
    return "Choose a PDF document.";
  if (file.size === 0) return "This PDF is empty. Choose another document.";
  if (file.size > maxPDFBytes) return "Choose a PDF up to 5 MB.";
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
export const entrySchema = z.object({
  rawText: z.string().refine((value) => {
    const length = [...normalizeRoleText(value)].length;
    return length >= 100 && length <= 20_000;
  }, "Paste 100–20,000 characters of job description text."),
});
export type RoleProfile = z.infer<typeof roleProfileSchema>;
export type RoleReview = z.infer<typeof reviewSchema>;
export type RoleRequirement = z.infer<typeof requirementSchema>;
export type RoleSummary = z.infer<typeof summarySchema>;
export type EntryValues = z.infer<typeof entrySchema>;
export const statusLabels: Record<RoleProfile["status"], string> = {
  draft: "Draft",
  analyzing: "Analyzing",
  review: "Needs review",
  confirmed: "Confirmed",
};
export const categoryLabels: Record<z.infer<typeof categorySchema>, string> = {
  programming_language: "Language",
  framework: "Framework",
  database: "Database",
  tool: "Tool",
  technology: "Technology",
  other: "Technical skill",
};
export function isKnownTarget(target: string) {
  return proficiencyLevels.some((value) => value === target);
}
// Targets saved before proficiency levels existed are free text; keep them as written.
export function isCustomTarget({ name, target }: RoleRequirement) {
  return Boolean(target) && !isKnownTarget(target) && target !== name;
}
export function requirementLabel({ kind, category, type }: RoleRequirement) {
  const classification =
    kind === "skill" && category
      ? categoryLabels[category]
      : kind === "technology"
        ? "Technology"
        : "Domain knowledge";
  if (!type) return classification;
  return `${classification} · ${type === "required" ? "Required" : "Preferred"}`;
}
