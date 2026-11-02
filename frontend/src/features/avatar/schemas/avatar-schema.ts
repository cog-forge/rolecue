import type { useTranslations } from "next-intl";
import { z } from "zod";

// Mirrors Go domain.PricePerSlot (Decision 7). The API checks the balance, so it stays authoritative.
export const slotPriceCoins = 1;
export const maxSlotsPerPurchase = 10;

// Mirrors domain.CreateAvatarInput.Validate: trimmed, counted in code points like Go runes.
type MetadataMessages = {
  nameRequired: string;
  nameInvalid: string;
  personalityRequired: string;
  personalityInvalid: string;
};

function metadataText(required: string, invalid: string, max: number) {
  return z
    .string()
    .trim()
    .min(1, required)
    .refine(
      (value) => Array.from(value).length <= max && !value.includes("\0"),
      invalid,
    );
}

function buildMetadataSchema(messages: MetadataMessages) {
  return z.object({
    name: metadataText(messages.nameRequired, messages.nameInvalid, 100),
    personality: metadataText(
      messages.personalityRequired,
      messages.personalityInvalid,
      2000,
    ),
  });
}

type PurchaseMessages = {
  amountRequired: string;
  amountInteger: string;
  amountMin: string;
  amountMax: string;
};

function buildPurchaseSchema(messages: PurchaseMessages) {
  return z.object({
    amount: z
      .number({ error: messages.amountRequired })
      .int(messages.amountInteger)
      .min(1, messages.amountMin)
      .max(maxSlotsPerPurchase, messages.amountMax),
  });
}

// Field messages come from `candidate.avatars.validation`: build the schema from the screen's translator.
export type AvatarValidation = ReturnType<
  typeof useTranslations<"candidate.avatars.validation">
>;

export function createAvatarMetadataSchema(t: AvatarValidation) {
  return buildMetadataSchema({
    nameRequired: t("nameRequired"),
    nameInvalid: t("nameInvalid", { max: 100 }),
    personalityRequired: t("personalityRequired"),
    personalityInvalid: t("personalityInvalid", { max: 2000 }),
  });
}

export function createPurchaseFormSchema(t: AvatarValidation) {
  return buildPurchaseSchema({
    amountRequired: t("amountRequired"),
    amountInteger: t("amountInteger"),
    amountMin: t("amountMin"),
    amountMax: t("amountMax", { max: maxSlotsPerPurchase }),
  });
}

// For the API layer, which only checks the shape: its messages are never shown.
export const avatarMetadataSchema = buildMetadataSchema({
  nameRequired: "nameRequired",
  nameInvalid: "nameInvalid",
  personalityRequired: "personalityRequired",
  personalityInvalid: "personalityInvalid",
});
export type AvatarMetadata = z.infer<typeof avatarMetadataSchema>;

export const purchaseFormSchema = buildPurchaseSchema({
  amountRequired: "amountRequired",
  amountInteger: "amountInteger",
  amountMin: "amountMin",
  amountMax: "amountMax",
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
