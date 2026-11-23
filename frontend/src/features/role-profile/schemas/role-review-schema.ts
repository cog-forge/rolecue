import { z } from "zod";
import {
  createRequirementSchema,
  senioritySchema,
} from "./role-profile-schema";
import {
  guardMessages,
  roleText,
  type RoleValidation,
} from "./role-validation";

export function createReviewSchema(v: RoleValidation) {
  return z.object({
    title: roleText(v, 1, 200),
    seniorityLevel: senioritySchema.nullable(),
    requirements: z
      .array(createRequirementSchema(v))
      .min(1, v("requirementsMin"))
      .max(200),
    refinementNote: roleText(v, 0, 2000),
  });
}
export const reviewSchema = createReviewSchema(guardMessages);

export type RoleReview = z.infer<typeof reviewSchema>;
