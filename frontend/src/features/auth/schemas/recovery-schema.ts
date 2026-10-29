import { z } from "zod";
import type { Validation } from "@/lib/i18n/validation";

export function createForgotPasswordSchema(v: Validation) {
  return z.object({
    email: z
      .string()
      .trim()
      .min(1, v("emailRequired"))
      .email(v("emailInvalid")),
  });
}

export function createResetPasswordSchema(v: Validation) {
  return z
    .object({
      password: z
        .string()
        .min(8, v("passwordMin", { min: 8 }))
        .max(128, v("passwordTooLong")),
      confirmPassword: z.string().min(1, v("confirmPasswordRequired")),
    })
    .refine((values) => values.password === values.confirmPassword, {
      path: ["confirmPassword"],
      message: v("passwordsMismatch"),
    });
}

export type ForgotPasswordValues = z.output<
  ReturnType<typeof createForgotPasswordSchema>
>;
export type ResetPasswordValues = z.output<
  ReturnType<typeof createResetPasswordSchema>
>;
