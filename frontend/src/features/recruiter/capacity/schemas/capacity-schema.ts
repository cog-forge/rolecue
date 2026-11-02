import { z } from "zod";
import type { useTranslations } from "next-intl";
import { postingStatusSchema } from "@/features/recruiter/job-postings/schemas/job-posting-schema";

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

// The form's `useTranslations("recruiter.capacity.validation")` satisfies this.
export type CapacityValidation = ReturnType<
  typeof useTranslations<"recruiter.capacity.validation">
>;

// The form keeps the typed text; the output is the whole number the API receives as { slots }.
export function createFundCapacitySchema(v: CapacityValidation) {
  return z.object({
    slots: z
      .string()
      .trim()
      .regex(/^\d+$/, v("wholeNumber"))
      .transform(Number)
      .pipe(
        z
          .number()
          .min(MIN_SLOTS, v("min", { min: MIN_SLOTS }))
          .max(MAX_SLOTS, v("max", { max: MAX_SLOTS })),
      ),
  });
}
export type FundCapacityInput = z.input<
  ReturnType<typeof createFundCapacitySchema>
>;
export type FundCapacityValues = z.output<
  ReturnType<typeof createFundCapacitySchema>
>;
