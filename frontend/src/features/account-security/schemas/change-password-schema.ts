import { z } from "zod";
import type { Validation } from "@/lib/i18n/validation";

// Same length rule as sign-up, reset and the Better Auth server (minPasswordLength 8).
export function createChangePasswordSchema(v: Validation) {
  return (
    z
      .object({
        currentPassword: z
          .string()
          .min(1, v("enterCurrentPassword"))
          .max(128, v("passwordTooLong")),
        newPassword: z
          .string()
          .min(8, v("passwordMin", { min: 8 }))
          .max(128, v("passwordTooLong")),
        confirmPassword: z.string().min(1, v("confirmNewPasswordRequired")),
        revokeOtherSessions: z.boolean(),
      })
      .refine((values) => values.newPassword === values.confirmPassword, {
        path: ["confirmPassword"],
        message: v("passwordsMismatch"),
      })
      // Better Auth does not reject reusing the current password, so the form does.
      .refine(
        (values) =>
          !values.currentPassword ||
          values.newPassword !== values.currentPassword,
        {
          path: ["newPassword"],
          message: v("passwordSameAsCurrent"),
        },
      )
  );
}

export type ChangePasswordValues = z.output<
  ReturnType<typeof createChangePasswordSchema>
>;
