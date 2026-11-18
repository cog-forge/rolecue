import type {
  AdminListing,
  CatalogItem,
  SellerListing,
  PurchaseReceipt,
} from "@/features/marketplace/schemas/marketplace-schema";
export const listingId = "11111111-1111-4111-8111-111111111111";
export const avatarId = "22222222-2222-4222-8222-222222222222";
export const sellerId = "33333333-3333-4333-8333-333333333333";
export const buyerId = "44444444-4444-4444-8444-444444444444";
export const version = "2026-11-16T05:01:02.123456Z";
export const nextVersion = "2026-11-16T05:01:03.654321Z";
export function catalogItem(overrides: Partial<CatalogItem> = {}): CatalogItem {
  return {
    id: listingId,
    title: "Refund Bot",
    description: "A calm interview companion.",
    default_personality: "Ask one clear question at a time.",
    price_coins: 20,
    seller: { display_name: "Mai Nguyen" },
    already_owned: false,
    is_own_listing: false,
    preview_url: `/marketplace/listings/${listingId}/preview`,
    created_at: version,
    ...overrides,
  };
}
export function sellerListing(
  overrides: Partial<SellerListing> = {},
): SellerListing {
  const { seller, already_owned, is_own_listing, ...common } = catalogItem();
  void seller;
  void already_owned;
  void is_own_listing;
  return {
    ...common,
    asset_id: avatarId,
    status: "draft",
    rights_note: "I created this avatar and own its commercial rights.",
    review_note: null,
    reviewed_at: null,
    updated_at: version,
    can_edit: true,
    can_submit: true,
    can_delist: false,
    preview_url: `/marketplace/my-listings/${listingId}/preview`,
    ...overrides,
  };
}
export function adminListing(
  overrides: Partial<AdminListing> = {},
): AdminListing {
  const { can_edit, can_submit, can_delist, ...common } = sellerListing();
  void can_edit;
  void can_submit;
  void can_delist;
  return {
    ...common,
    status: "pending",
    seller: { display_name: "Mai Nguyen" },
    seller_id: sellerId,
    asset_ready: true,
    reviewed_by: null,
    preview_url: `/admin/marketplace/listings/${listingId}/preview`,
    ...overrides,
  };
}
export function purchaseReceipt(
  overrides: Partial<PurchaseReceipt> = {},
): PurchaseReceipt {
  return {
    avatar: {
      id: avatarId,
      name: "Refund Bot",
      personality: "Ask one clear question at a time.",
      created_at: version,
      updated_at: version,
    },
    transaction: {
      id: listingId,
      amount: 20,
      currency: "COIN",
      created_at: version,
    },
    balance: 80,
    replayed: false,
    ...overrides,
  };
}
export const pageOf = <T>(items: T[], total = items.length) => ({
  items,
  pagination: { limit: 12, offset: 0, total },
});
export const ok = (data: unknown) => ({ data: { data } });
