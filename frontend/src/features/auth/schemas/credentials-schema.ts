import { z } from "zod";
import type { Validation } from "@/lib/i18n/validation";

export function createCurrentPasswordSchema(v: Validation) {
  return z.object({
    password: z
      .string()
      .min(1, v("currentPasswordRequired"))
      .max(128, v("passwordTooLong")),
  });
}

export function createTotpVerificationSchema(v: Validation) {
  return z.object({
    code: z.string().regex(/^\d{6}$/, v("totpCode")),
    trustDevice: z.boolean(),
  });
}

export function createBackupVerificationSchema(v: Validation) {
  return z.object({
    code: z.string().trim().min(1, v("backupCodeRequired")),
    trustDevice: z.boolean(),
  });
}

export const totpSetupSchema = z.object({
  method: z.literal("totp"),
  totpURI: z.string().startsWith("otpauth://totp/"),
  backupCodes: z.array(z.string()).min(1),
});

export type CurrentPasswordValues = z.infer<
  ReturnType<typeof createCurrentPasswordSchema>
>;
export type VerificationValues = z.infer<
  ReturnType<typeof createTotpVerificationSchema>
>;
export type TotpSetup = z.infer<typeof totpSetupSchema>;
