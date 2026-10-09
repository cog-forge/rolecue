import { z } from "zod";
import {
  createProfileFormSchema,
  isHttpsProfileUrl,
  type Profile,
} from "@/features/profile/schemas/profile-schema";
export const roleSelectionSchema = z.object({
  role: z.enum(["candidate", "recruiter"]),
});
export type OnboardingRole = z.infer<typeof roleSelectionSchema>["role"];
export function createOnboardingSchema(profile: Profile) {
  return createProfileFormSchema(profile).superRefine((values, ctx) => {
    if (profile.role !== "recruiter") return;
    if (!values.company_name.trim())
      ctx.addIssue({
        code: "custom",
        path: ["company_name"],
        message: "Enter your company name.",
      });
    if (!values.company_website.trim())
      ctx.addIssue({
        code: "custom",
        path: ["company_website"],
        message: "Enter your company website.",
      });
    if (
      values.company_website.trim() &&
      !isHttpsProfileUrl(values.company_website.trim())
    )
      ctx.addIssue({
        code: "custom",
        path: ["company_website"],
        message: "Use a full HTTPS company website.",
      });
  });
}
