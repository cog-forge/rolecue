import { z } from "zod";

export const meSchema = z.object({
  success: z.literal(true),
  data: z.object({
    id: z.uuid(),
    email: z.email(),
    full_name: z.string(),
    role: z.enum(["candidate", "recruiter", "admin"]),
    email_verified: z.boolean(),
    is_locked: z.boolean(),
    image: z.string().nullable().optional(),
    onboarding_role_selected: z.boolean(),
    onboarding_completed: z.boolean(),
  }),
});
export type SessionUser = z.infer<typeof meSchema>["data"];
