import { z } from "zod";

export const listingStatusSchema = z.enum([
  "draft",
  "pending",
  "active",
  "rejected",
  "delisted",
  "suspended",
]);
export type ListingStatus = z.infer<typeof listingStatusSchema>;
export const catalogSortSchema = z.enum(["newest", "price_asc", "price_desc"]);
export type CatalogSort = z.infer<typeof catalogSortSchema>;
const timestamp = z.string().datetime({ offset: true });
const publicFields = {
  id: z.string().uuid(),
  title: z.string(),
  description: z.string().nullable(),
  default_personality: z.string(),
  price_coins: z.number().int().positive(),
  preview_url: z.string().nullable(),
  created_at: timestamp,
};
const seller = z.object({ display_name: z.string() });
export const catalogItemSchema = z.object({
  ...publicFields,
  seller,
  already_owned: z.boolean(),
  is_own_listing: z.boolean(),
});
export type CatalogItem = z.infer<typeof catalogItemSchema>;
const privateFields = {
  ...publicFields,
  asset_id: z.string().uuid(),
  status: listingStatusSchema,
  rights_note: z.string(),
  review_note: z.string().nullable(),
  reviewed_at: timestamp.nullable(),
  updated_at: timestamp,
};
export const sellerListingSchema = z.object({
  ...privateFields,
  can_edit: z.boolean(),
  can_submit: z.boolean(),
  can_delist: z.boolean(),
});
export type SellerListing = z.infer<typeof sellerListingSchema>;
export const adminListingSchema = z.object({
  ...privateFields,
  seller,
  seller_id: z.string().uuid(),
  asset_ready: z.boolean(),
  reviewed_by: z.string().uuid().nullable(),
});
export type AdminListing = z.infer<typeof adminListingSchema>;
export const envelope = <T extends z.ZodType>(data: T) => z.object({ data });
export const pageSchema = <T extends z.ZodType>(item: T) =>
  z.object({
    items: z.array(item),
    pagination: z.object({
      limit: z.number().int().positive(),
      offset: z.number().int().nonnegative(),
      total: z.number().int().nonnegative(),
    }),
  });
export const eligibleSchema = z.array(
  z.object({ avatar_id: z.string().uuid(), name: z.string() }),
);
export const receiptSchema = z.object({
  avatar: z
    .object({
      id: z.string().uuid(),
      name: z.string(),
      personality: z.string(),
      created_at: timestamp,
      updated_at: timestamp,
    })
    .nullable(),
  transaction: z.object({
    id: z.string().uuid(),
    amount: z.number().int().positive(),
    currency: z.literal("COIN"),
    created_at: timestamp,
  }),
  balance: z.number().int().nonnegative(),
  replayed: z.boolean(),
});
export type PurchaseReceipt = z.infer<typeof receiptSchema>;

// Count code points just as the API counts Go runes, including emoji.
const text = (max: number, required: boolean, message: string) =>
  z
    .string()
    .trim()
    .refine(
      (value) =>
        (!required || value.length > 0) &&
        Array.from(value).length <= max &&
        !value.includes("\0"),
      message,
    );
export function createListingFormSchema(
  message: (
    key: "required" | "titleLength" | "textLength" | "priceInvalid",
  ) => string,
) {
  return z.object({
    avatar_id: z.string().uuid(message("required")),
    title: text(100, true, message("titleLength")),
    description: text(2000, false, message("textLength")),
    default_personality: text(2000, true, message("textLength")),
    rights_note: text(2000, true, message("textLength")),
    price_coins: z
      .number({ error: message("priceInvalid") })
      .int(message("priceInvalid"))
      .min(1, message("priceInvalid"))
      .max(1_000_000, message("priceInvalid")),
  });
}
export type ListingFormValues = z.infer<
  ReturnType<typeof createListingFormSchema>
>;
export const listingFormSchema = createListingFormSchema((key) => key);
export const expectedVersionSchema = z.object({
  expected_updated_at: timestamp,
});
export const reasonSchema = text(2000, true, "reasonInvalid");
