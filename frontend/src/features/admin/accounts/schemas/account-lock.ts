import { z } from "zod";

export const lockDurations = [
  { value: "3600", label: "1 hour" },
  { value: "86400", label: "24 hours" },
  { value: "604800", label: "7 days" },
  { value: "2592000", label: "30 days" },
] as const;

const reasonSchema = z
  .string()
  .trim()
  .min(1, "Enter a reason for locking this account.")
  .max(500, "Use 500 characters or fewer.");
export const targetRequestSchema = z.object({
  userId: z.uuid().transform((id) => id.toLowerCase()),
});
export const banRequestSchema = targetRequestSchema.extend({
  banReason: reasonSchema,
  banExpiresIn: z
    .number()
    .int()
    .positive()
    .refine(
      (seconds) =>
        Number.isFinite(new Date(Date.now() + seconds * 1000).getTime()),
      "Choose a valid lock duration.",
    )
    .optional(),
});

function localExpiry(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return Number.NaN;
  const date = new Date(value);
  const [year, month, day, hour, minute] = value.split(/[-T:]/).map(Number);
  // Reject normalized impossible dates and local times skipped by DST.
  if (
    date.getFullYear() !== year ||
    date.getMonth() + 1 !== month ||
    date.getDate() !== day ||
    date.getHours() !== hour ||
    date.getMinutes() !== minute
  )
    return Number.NaN;
  return date.getTime();
}

export function createLockFormSchema(now = Date.now) {
  return z
    .object({
      reason: reasonSchema,
      indefinite: z.boolean(),
      duration: z.enum(["", "3600", "86400", "604800", "2592000", "custom"]),
      expiresAt: z.string(),
    })
    .superRefine((values, ctx) => {
      if (values.indefinite) return;
      if (!values.duration)
        ctx.addIssue({
          code: "custom",
          path: ["duration"],
          message: "Choose a lock duration.",
        });
      if (
        values.duration === "custom" &&
        !(localExpiry(values.expiresAt) > now())
      )
        ctx.addIssue({
          code: "custom",
          path: ["expiresAt"],
          message: "Choose a valid date and time in the future.",
        });
    });
}

export type LockFormValues = z.infer<ReturnType<typeof createLockFormSchema>>;

export function lockRequest(
  userId: string,
  values: LockFormValues,
  now = Date.now(),
) {
  const parsed = createLockFormSchema(() => now).parse(values);
  return banRequestSchema.parse({
    userId,
    banReason: parsed.reason,
    ...(parsed.indefinite
      ? {}
      : {
          banExpiresIn:
            parsed.duration === "custom"
              ? Math.ceil((localExpiry(parsed.expiresAt) - now) / 1000)
              : Number(parsed.duration),
        }),
  });
}
