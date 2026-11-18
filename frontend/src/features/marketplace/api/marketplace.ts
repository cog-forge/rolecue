import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import { getApiBaseUrl } from "@/lib/api/config";
import { readApiError } from "@/lib/i18n/api-errors";
import {
  adminListingSchema,
  catalogItemSchema,
  eligibleSchema,
  envelope,
  expectedVersionSchema,
  listingFormSchema,
  pageSchema,
  reasonSchema,
  receiptSchema,
  sellerListingSchema,
  type CatalogSort,
  type ListingFormValues,
  type ListingStatus,
} from "../schemas/marketplace-schema";

export const marketplaceKeys = {
  all: (userId: string) => ["marketplace", userId] as const,
  catalog: (userId: string) =>
    [...marketplaceKeys.all(userId), "catalog"] as const,
  detail: (userId: string, id: string) =>
    [...marketplaceKeys.catalog(userId), id] as const,
  own: (userId: string) => [...marketplaceKeys.all(userId), "own"] as const,
  listing: (userId: string, id: string) =>
    [...marketplaceKeys.own(userId), id] as const,
  eligible: (userId: string) =>
    [...marketplaceKeys.all(userId), "eligible"] as const,
  admin: (userId: string) => [...marketplaceKeys.all(userId), "admin"] as const,
  review: (userId: string, id: string) =>
    [...marketplaceKeys.admin(userId), id] as const,
};
export const marketplacePageSize = 12;
const segment = (id: string) => encodeURIComponent(z.string().uuid().parse(id));
export async function getCatalog(
  params: { q: string; sort: CatalogSort; offset: number },
  signal?: AbortSignal,
) {
  const result = await apiClient.get<unknown>("/marketplace/listings", {
    params: { ...params, limit: marketplacePageSize },
    signal,
  });
  return envelope(pageSchema(catalogItemSchema)).parse(result.data).data;
}
export async function getCatalogItem(id: string, signal?: AbortSignal) {
  const result = await apiClient.get<unknown>(
    `/marketplace/listings/${segment(id)}`,
    { signal },
  );
  return envelope(catalogItemSchema).parse(result.data).data;
}
export async function getEligibleAvatars(signal?: AbortSignal) {
  const result = await apiClient.get<unknown>("/marketplace/eligible-avatars", {
    signal,
  });
  return envelope(eligibleSchema).parse(result.data).data;
}
export async function getMyListings(offset: number, signal?: AbortSignal) {
  const result = await apiClient.get<unknown>("/marketplace/my-listings", {
    params: { limit: marketplacePageSize, offset },
    signal,
  });
  return envelope(pageSchema(sellerListingSchema)).parse(result.data).data;
}
export async function getMyListing(id: string, signal?: AbortSignal) {
  const result = await apiClient.get<unknown>(
    `/marketplace/my-listings/${segment(id)}`,
    { signal },
  );
  return envelope(sellerListingSchema).parse(result.data).data;
}
export async function saveListing({
  id,
  version,
  values,
}: {
  id?: string;
  version?: string;
  values: ListingFormValues;
}) {
  const fields = listingFormSchema.parse(values);
  const { avatar_id, ...editable } = fields;
  const result = id
    ? await apiClient.put<unknown>(`/marketplace/my-listings/${segment(id)}`, {
        ...editable,
        ...expectedVersionSchema.parse({ expected_updated_at: version }),
      })
    : await apiClient.post<unknown>("/marketplace/my-listings", {
        avatar_id,
        ...editable,
      });
  return envelope(sellerListingSchema).parse(result.data).data;
}
export async function sellerAction(
  id: string,
  action: "submit" | "delist",
  version: string,
) {
  const result = await apiClient.post<unknown>(
    `/marketplace/my-listings/${segment(id)}/${action}`,
    expectedVersionSchema.parse({ expected_updated_at: version }),
  );
  return envelope(sellerListingSchema).parse(result.data).data;
}
export async function getAdminListings(
  status: ListingStatus | "all",
  offset: number,
  signal?: AbortSignal,
) {
  const result = await apiClient.get<unknown>("/admin/marketplace/listings", {
    params: {
      status: status === "all" ? undefined : status,
      limit: marketplacePageSize,
      offset,
    },
    signal,
  });
  return envelope(pageSchema(adminListingSchema)).parse(result.data).data;
}
export async function getAdminListing(id: string, signal?: AbortSignal) {
  const result = await apiClient.get<unknown>(
    `/admin/marketplace/listings/${segment(id)}`,
    { signal },
  );
  return envelope(adminListingSchema).parse(result.data).data;
}
export async function adminAction(
  id: string,
  action: "approve" | "reject" | "suspend",
  version: string,
  reason: string,
) {
  // Suspend has no version field in the contract and rejects unknown JSON fields.
  const body =
    action === "suspend"
      ? { reason: reasonSchema.parse(reason) }
      : {
          ...expectedVersionSchema.parse({ expected_updated_at: version }),
          ...(action === "reject"
            ? { reason: reasonSchema.parse(reason) }
            : {}),
        };
  const result = await apiClient.post<unknown>(
    `/admin/marketplace/listings/${segment(id)}/${action}`,
    body,
  );
  return envelope(adminListingSchema).parse(result.data).data;
}
export function createPurchaseIntent(id: string, price: number) {
  z.number().int().min(1).max(1_000_000).parse(price);
  return {
    price,
    key: `mkt1:${segment(id).toLowerCase()}:${price}:${crypto.randomUUID()}`,
  };
}
export type PurchaseIntent = ReturnType<typeof createPurchaseIntent>;
export async function purchaseListing(id: string, intent: PurchaseIntent) {
  const result = await apiClient.post<unknown>(
    `/marketplace/listings/${segment(id)}/purchase`,
    { price_coins: intent.price },
    { headers: { "Idempotency-Key": intent.key } },
  );
  return envelope(receiptSchema).parse(result.data).data;
}
export function purchaseOutcomeUnknown(error: unknown) {
  const { status } = readApiError(error);
  return status === 0 || status >= 500;
}
export function previewSrc(path: string | null) {
  // Only the image redirect endpoint is allowed, never a model URL or arbitrary external URL.
  if (
    !path ||
    !/^\/(?:admin\/)?marketplace\/(?:listings|my-listings)\/[0-9a-f-]+\/preview$/.test(
      path,
    )
  )
    return null;
  const base = getApiBaseUrl();
  return base ? `${base.replace(/\/$/, "")}${path}` : null;
}
