import { z } from "zod";
import type { Validation } from "@/lib/i18n/validation";

export function createLoginSchema(v: Validation) {
  return z.object({
    email: z
      .string()
      .trim()
      .min(1, v("emailRequired"))
      .email(v("emailInvalid")),
    password: z.string().min(1, v("passwordRequired")),
  });
}

export type LoginFormValues = z.output<ReturnType<typeof createLoginSchema>>;
