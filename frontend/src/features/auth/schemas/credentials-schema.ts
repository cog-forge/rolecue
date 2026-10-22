import { z } from "zod";

export const currentPasswordSchema = z.object({
  password: z.string().min(1, "Current password is required").max(128),
});

export const totpVerificationSchema = z.object({
  code: z
    .string()
    .regex(/^\d{6}$/, "Enter the 6-digit code from your authenticator"),
  trustDevice: z.boolean(),
});

export const backupVerificationSchema = z.object({
  code: z.string().trim().min(1, "Enter a backup code"),
  trustDevice: z.boolean(),
});

export const totpSetupSchema = z.object({
  method: z.literal("totp"),
  totpURI: z.string().startsWith("otpauth://totp/"),
  backupCodes: z.array(z.string()).min(1),
});

export type CurrentPasswordValues = z.infer<typeof currentPasswordSchema>;
export type VerificationValues = z.infer<typeof totpVerificationSchema>;
export type TotpSetup = z.infer<typeof totpSetupSchema>;
