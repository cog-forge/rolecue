import { z } from "zod";

export const profileEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    id: z.uuid(),
    full_name: z.string(),
    email: z.email(),
    image: z.string().nullable(),
    role: z.enum(["candidate", "recruiter", "admin"]),
    email_verified: z.boolean(),
    company_name: z.string().nullable(),
    company_website: z.string().nullable(),
    created_at: z.iso.datetime({ offset: true }),
    updated_at: z.iso.datetime({ offset: true }),
  }),
});
export type Profile = z.infer<typeof profileEnvelopeSchema>["data"];
export type ProfileValues = {
  full_name: string;
  image: string;
  company_name: string;
  company_website: string;
};
export type ProfilePatch = Partial<{
  full_name: string;
  image: string | null;
  company_name: string | null;
  company_website: string | null;
}>;

export function profileValues(profile: Profile): ProfileValues {
  return {
    full_name: profile.full_name,
    image: profile.image ?? "",
    company_name: profile.company_name ?? "",
    company_website: profile.company_website ?? "",
  };
}

const length = (value: string) => Array.from(value).length;
function httpsUrl(value: string) {
  if (length(value) > 2048) return false;
  try {
    const url = new URL(value);
    return (
      url.protocol === "https:" &&
      !!url.hostname &&
      !url.username &&
      !url.password
    );
  } catch {
    return false;
  }
}

export function createProfileFormSchema(profile: Profile) {
  return z
    .object({
      full_name: z.string(),
      image: z.string(),
      company_name: z.string(),
      company_website: z.string(),
    })
    .superRefine((values, ctx) => {
      if (!values.full_name.trim() || length(values.full_name.trim()) > 100)
        ctx.addIssue({
          code: "custom",
          path: ["full_name"],
          message: "Enter a display name between 1 and 100 characters.",
        });
      if (
        values.image !== (profile.image ?? "") &&
        values.image.trim() &&
        !httpsUrl(values.image.trim())
      )
        ctx.addIssue({
          code: "custom",
          path: ["image"],
          message:
            "Use an HTTPS image URL without credentials, up to 2048 characters.",
        });
      if (profile.role !== "recruiter") return;
      if (length(values.company_name.trim()) > 200)
        ctx.addIssue({
          code: "custom",
          path: ["company_name"],
          message: "Company name must be 200 characters or fewer.",
        });
      if (
        values.company_website !== (profile.company_website ?? "") &&
        values.company_website.trim() &&
        !httpsUrl(values.company_website.trim())
      )
        ctx.addIssue({
          code: "custom",
          path: ["company_website"],
          message:
            "Use an HTTPS website URL without credentials, up to 2048 characters.",
        });
    });
}

export function profilePatch(
  values: ProfileValues,
  profile: Profile,
): ProfilePatch {
  const current = profileValues(profile);
  const patch: ProfilePatch = {};
  if (values.full_name !== current.full_name)
    patch.full_name = values.full_name.trim();
  if (values.image !== current.image) patch.image = values.image.trim() || null;
  if (profile.role === "recruiter") {
    if (values.company_name !== current.company_name)
      patch.company_name = values.company_name.trim() || null;
    if (values.company_website !== current.company_website)
      patch.company_website = values.company_website.trim() || null;
  }
  return patch;
}
