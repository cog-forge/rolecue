import { z } from "zod";
import {
  guardMessages,
  roleText,
  type RoleValidation,
} from "./role-validation";

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

export function createRequirementSchema(v: RoleValidation) {
  return z
    .object({
      name: roleText(v, 1, 200),
      target: roleText(v, 1, 2000),
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
export const requirementSchema = createRequirementSchema(guardMessages);

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

export type RoleProfile = z.infer<typeof roleProfileSchema>;
export type RoleRequirement = z.infer<typeof requirementSchema>;
export type RoleSummary = z.infer<typeof summarySchema>;
