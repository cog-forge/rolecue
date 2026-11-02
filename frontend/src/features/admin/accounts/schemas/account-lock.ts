import { z } from "zod";
import type { useTranslations } from "next-intl";

// `admin.accounts.validation` translator; the form schema is built from it so field errors are
// already in the active language.
export type LockValidation = ReturnType<
  typeof useTranslations<"admin.accounts.validation">
>;

// `labelKey` names the `admin.accounts.fields.durations.*` entry that shows the preset.
export const lockDurations = [
  { value: "3600", labelKey: "hour1" },
  { value: "86400", labelKey: "hours24" },
  { value: "604800", labelKey: "days7" },
  { value: "2592000", labelKey: "days30" },
] as const;

const MAX_REASON_LENGTH = 500;

// The request contract sent to the API. Its checks guard the payload and are never shown, so they
// carry no copy; the form schema below owns the messages the Admin reads.
const requestReasonSchema = z.string().trim().min(1).max(MAX_REASON_LENGTH);
export const targetRequestSchema = z.object({
  userId: z.uuid().transform((id) => id.toLowerCase()),
});
export const banRequestSchema = targetRequestSchema.extend({
  banReason: requestReasonSchema,
  banExpiresIn: z
    .number()
    .int()
    .positive()
    .refine((seconds) =>
      Number.isFinite(new Date(Date.now() + seconds * 1000).getTime()),
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

export function createLockFormSchema(v: LockValidation, now = Date.now) {
  return z
    .object({
      reason: z
        .string()
        .trim()
        .min(1, v("reasonRequired"))
        .max(MAX_REASON_LENGTH, v("reasonMax")),
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
          message: v("durationRequired"),
        });
      if (
        values.duration === "custom" &&
        !(localExpiry(values.expiresAt) > now())
      )
        ctx.addIssue({
          code: "custom",
          path: ["expiresAt"],
          message: v("expiryInvalid"),
        });
    });
}

export type LockFormValues = z.infer<ReturnType<typeof createLockFormSchema>>;

export function lockRequest(
  userId: string,
  values: LockFormValues,
  v: LockValidation,
  now = Date.now(),
) {
  const parsed = createLockFormSchema(v, () => now).parse(values);
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
