import { z } from "zod";

export const loginSchema = z.object({
  email: z
    .string()
    .trim()
    .min(1, "Email address is required")
    .email("Please enter a valid email address"),
  password: z.string().min(1, "Password is required"),
});

export const loginResponseSchema = z.object({
  success: z.literal(true),
  data: z.string().min(1),
});

export type LoginCredentials = z.input<typeof loginSchema>;
export type LoginFormValues = z.output<typeof loginSchema>;
