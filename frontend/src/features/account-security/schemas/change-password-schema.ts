import { z } from "zod";

// Same length rule as sign-up, reset and the Better Auth server (minPasswordLength 8).
export const changePasswordSchema = z
  .object({
    currentPassword: z
      .string()
      .min(1, "Enter your current password")
      .max(128, "Password is too long"),
    newPassword: z
      .string()
      .min(8, "Use at least 8 characters")
      .max(128, "Password is too long"),
    confirmPassword: z.string().min(1, "Please confirm your new password"),
    revokeOtherSessions: z.boolean(),
  })
  .refine((values) => values.newPassword === values.confirmPassword, {
    path: ["confirmPassword"],
    message: "Passwords do not match",
  })
  // Better Auth does not reject reusing the current password, so the form does.
  .refine(
    (values) =>
      !values.currentPassword || values.newPassword !== values.currentPassword,
    {
      path: ["newPassword"],
      message: "Choose a password different from your current one",
    },
  );

export type ChangePasswordValues = z.output<typeof changePasswordSchema>;
