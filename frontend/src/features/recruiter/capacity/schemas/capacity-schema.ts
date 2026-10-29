import { z } from "zod";
import { postingStatusSchema } from "@/features/recruiter/job-postings/schemas/job-posting-schema";
import { defaultLocale } from "@/lib/i18n/config";
import { formatNumber } from "@/lib/i18n/formatters";

// Mirrors the Go CapacityState contract (KAN-128, snake_case). The submitted-CV count is not part
// of it: submissions never count as capacity.
export const capacityStateSchema = z.object({
  interview_slot: z.number().int().nonnegative().nullable(),
  funded: z.boolean(),
  price_per_slot: z.number().int().nonnegative(),
  cv_approvals: z.number().int().nonnegative(),
  consumed: z.number().int().nonnegative(),
  available_to_approve: z.number().int().nonnegative(),
  refundable: z.number().int().nonnegative(),
  posting_status: postingStatusSchema,
  // Coins returned at close. Non-null only once the posting is closed.
  refunded: z.number().int().nonnegative().nullable(),
});
export type CapacityState = z.infer<typeof capacityStateSchema>;

export const capacityEnvelopeSchema = z.object({
  success: z.literal(true),
  data: capacityStateSchema,
});

export const MIN_SLOTS = 1;
export const MAX_SLOTS = 10_000;

// The form keeps the typed text; the output is the whole number the API receives as { slots }.
export const fundCapacitySchema = z.object({
  slots: z
    .string()
    .trim()
    .regex(/^\d+$/, "Enter a whole number of slots.")
    .transform(Number)
    .pipe(
      z
        .number()
        .min(MIN_SLOTS, `Fund at least ${MIN_SLOTS} slot.`)
        .max(
          MAX_SLOTS,
          `Fund at most ${formatNumber(MAX_SLOTS, defaultLocale)} slots.`,
        ),
    ),
});
export type FundCapacityInput = z.input<typeof fundCapacitySchema>;
export type FundCapacityValues = z.output<typeof fundCapacitySchema>;
