import { z } from "zod";

// Mirrors Go domain.PricePerSlot (Decision 7). The API checks the balance, so it stays authoritative.
export const slotPriceCoins = 1;
export const maxSlotsPerPurchase = 10;

// Mirrors domain.CreateAvatarInput.Validate: trimmed, counted in code points like Go runes.
function metadataText(label: string, max: number) {
  return z
    .string()
    .trim()
    .min(1, `${label} is required.`)
    .refine(
      (value) => Array.from(value).length <= max && !value.includes("\0"),
      `${label} must be at most ${max} characters and contain no null characters.`,
    );
}

export const avatarMetadataSchema = z.object({
  name: metadataText("Name", 100),
  personality: metadataText("Personality", 2000),
});
export type AvatarMetadata = z.infer<typeof avatarMetadataSchema>;

export const purchaseFormSchema = z.object({
  amount: z
    .number({ error: "Enter a number of slots." })
    .int("Enter a whole number of slots.")
    .min(1, "Buy at least 1 slot.")
    .max(
      maxSlotsPerPurchase,
      `Buy at most ${maxSlotsPerPurchase} slots at once.`,
    ),
});
export type PurchaseForm = z.infer<typeof purchaseFormSchema>;

const envelope = <T extends z.ZodType>(data: T) =>
  z.object({ success: z.literal(true), data });

const capacitySchema = z.object({
  capacity: z.number().int().nonnegative(),
  used: z.number().int().nonnegative(),
  available: z.number().int().nonnegative(),
  is_full: z.boolean(),
});
export type Capacity = z.infer<typeof capacitySchema>;

const avatarSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  personality: z.string(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
export type Avatar = z.infer<typeof avatarSchema>;

const avatarDetailSchema = avatarSchema.extend({
  model_access: z.object({
    url: z.url({ protocol: /^https?$/ }),
    method: z.literal("GET"),
    headers: z.record(z.string(), z.string()),
    expires_at: z.iso.datetime({ offset: true }),
  }),
});
export type AvatarDetail = z.infer<typeof avatarDetailSchema>;

export const capacityEnvelopeSchema = envelope(capacitySchema);
export const avatarEnvelopeSchema = envelope(avatarSchema);
export const avatarDetailEnvelopeSchema = envelope(avatarDetailSchema);
export const avatarPageEnvelopeSchema = envelope(
  z.object({
    items: z.array(avatarSchema),
    pagination: z.object({
      limit: z.number().int(),
      offset: z.number().int(),
      total: z.number().int().nonnegative(),
    }),
  }),
);
export type AvatarPage = z.infer<typeof avatarPageEnvelopeSchema>["data"];
// POST /inventory/purchase: the GET /inventory summary after the purchase, plus the balance after the debit.
export const purchaseEnvelopeSchema = envelope(
  capacitySchema.extend({ balance: z.number().int() }),
);

// Avaturn SDK 1.1.4 ExportAvatarResult. RoleCue only relies on these fields; the GLB bytes are checked
// after download and again by the API, so the data URL's declared MIME type is not trusted either way.
export const avaturnExportSchema = z.object({
  avatarSupportsFaceAnimations: z.boolean(),
  urlType: z.enum(["dataURL", "httpURL"]),
  url: z.string().min(1),
});
