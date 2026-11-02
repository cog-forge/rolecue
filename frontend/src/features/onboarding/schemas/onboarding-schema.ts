import { z } from "zod";
import {
  createProfileFormSchema,
  isHttpsProfileUrl,
  normalizeHttpsProfileUrlInput,
  type Profile,
} from "@/features/profile/schemas/profile-schema";
export const roleSelectionSchema = z.object({
  role: z.enum(["candidate", "recruiter"]),
});
export type OnboardingRole = z.infer<typeof roleSelectionSchema>["role"];
export function createOnboardingSchema(
  profile: Profile,
  messages?: {
    name: string;
    image: string;
    companyLength: string;
    website: string;
    companyRequired: string;
    websiteRequired: string;
    companyWebsiteInvalid: string;
  },
) {
  return createProfileFormSchema(profile, messages)
    .safeExtend({
      company_website: z
        .string()
        .transform((value) =>
          profile.role === "recruiter"
            ? normalizeHttpsProfileUrlInput(value)
            : value,
        ),
    })
    .superRefine((values, ctx) => {
      if (profile.role !== "recruiter") return;
      if (!values.company_name.trim())
        ctx.addIssue({
          code: "custom",
          path: ["company_name"],
          message: messages?.companyRequired ?? "Enter your company name.",
        });
      if (!values.company_website.trim())
        ctx.addIssue({
          code: "custom",
          path: ["company_website"],
          message: messages?.websiteRequired ?? "Enter your company website.",
        });
      if (
        values.company_website.trim() &&
        !isHttpsProfileUrl(values.company_website.trim())
      )
        ctx.addIssue({
          code: "custom",
          path: ["company_website"],
          message:
            messages?.companyWebsiteInvalid ??
            "Use a full HTTPS company website.",
        });
    });
}
