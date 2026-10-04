import { z } from "zod";

export const registerSchema = z
  .object({
    email: z
      .string()
      .trim()
      .min(1, "Email address is required")
      .email("Please enter a valid email address"),
    password: z.string().min(8, "Use at least 8 characters").max(128),
    confirmPassword: z.string().min(1, "Please confirm your password"),
  })
  .refine((values) => values.password === values.confirmPassword, {
    path: ["confirmPassword"],
    message: "Passwords do not match",
  });

export type RegisterFormValues = z.output<typeof registerSchema>;
