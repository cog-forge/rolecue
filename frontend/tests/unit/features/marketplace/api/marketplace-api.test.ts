import { beforeEach, describe, expect, it, vi } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import {
  adminAction,
  createPurchaseIntent,
  getAdminListing,
  getAdminListings,
  getCatalog,
  getCatalogItem,
  getEligibleAvatars,
  getMyListing,
  getMyListings,
  marketplaceKeys,
  previewSrc,
  purchaseListing,
  purchaseOutcomeUnknown,
  saveListing,
  sellerAction,
} from "@/features/marketplace/api/marketplace";
import {
  createListingFormSchema,
  listingFormSchema,
} from "@/features/marketplace/schemas/marketplace-schema";
import {
  adminListing,
  avatarId,
  catalogItem,
  listingId,
  nextVersion,
  ok,
  pageOf,
  purchaseReceipt,
  sellerListing,
  version,
} from "@tests/fixtures/marketplace";
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ apiClient: mocks }));
vi.mock("@/lib/api/config", () => ({
  getApiBaseUrl: () => "http://localhost:8080",
}));
const values = {
  avatar_id: avatarId,
  title: " Refund Bot ",
  description: "",
  default_personality: "calm",
  price_coins: 20,
  rights_note: "I own this model",
};
beforeEach(() => vi.resetAllMocks());
it("scopes every cache to the session", () =>
  expect(marketplaceKeys.all("seller")).not.toEqual(
    marketplaceKeys.all("buyer"),
  ));
it("parses catalog without requiring a success flag, only returning public fields", async () => {
  mocks.get.mockResolvedValue(
    ok(
      pageOf([
        {
          ...catalogItem(),
          rights_note: "secret",
          model_url: "secret",
          seller_id: "private",
        },
      ]),
    ),
  );
  const signal = new AbortController().signal;
  const result = await getCatalog(
    { q: "Refund", sort: "price_asc", offset: 12 },
    signal,
  );
  expect(mocks.get).toHaveBeenCalledWith("/marketplace/listings", {
    params: { q: "Refund", sort: "price_asc", offset: 12, limit: 12 },
    signal,
  });
  expect(result.items[0]).not.toHaveProperty("rights_note");
  expect(result.items[0]).not.toHaveProperty("model_url");
  expect(result.items[0]).not.toHaveProperty("seller_id");
});
it.each([
  [
    () => getCatalogItem(listingId),
    catalogItem(),
    `/marketplace/listings/${listingId}`,
  ],
  [
    () => getMyListing(listingId),
    sellerListing(),
    `/marketplace/my-listings/${listingId}`,
  ],
  [
    () => getAdminListing(listingId),
    adminListing(),
    `/admin/marketplace/listings/${listingId}`,
  ],
  [
    () => getEligibleAvatars(),
    [{ avatar_id: avatarId, name: "Own avatar" }],
    "/marketplace/eligible-avatars",
  ],
] as const)("validates read boundary %#", async (call, payload, path) => {
  mocks.get.mockResolvedValue(ok(payload));
  expect(await call()).toEqual(payload);
  expect(mocks.get.mock.calls[0][0]).toBe(path);
  mocks.get.mockResolvedValue(ok({ bad: true }));
  await expect(call()).rejects.toThrow();
});
it("paginates seller and admin lists and omits all-status filter", async () => {
  mocks.get
    .mockResolvedValueOnce(ok(pageOf([sellerListing()])))
    .mockResolvedValueOnce(ok(pageOf([adminListing()])));
  await getMyListings(12);
  await getAdminListings("all", 24);
  expect(mocks.get.mock.calls[0][1].params).toEqual({ limit: 12, offset: 12 });
  expect(mocks.get.mock.calls[1][1].params).toEqual({
    status: undefined,
    limit: 12,
    offset: 24,
  });
});
it("creates a draft and edits the full payload with the untouched microsecond version", async () => {
  mocks.post.mockResolvedValue(ok(sellerListing()));
  mocks.put.mockResolvedValue(ok(sellerListing({ updated_at: nextVersion })));
  await saveListing({ values });
  expect(mocks.post.mock.calls[0][1]).toEqual({
    ...values,
    title: "Refund Bot",
  });
  await saveListing({ id: listingId, version, values });
  const body = mocks.put.mock.calls[0][1];
  expect(body.expected_updated_at).toBe(version);
  expect(body).not.toHaveProperty("avatar_id");
  expect(body.title).toBe("Refund Bot");
});
it.each(["submit", "delist"] as const)(
  "sends the original version to %s",
  async (action) => {
    mocks.post.mockResolvedValue(ok(sellerListing()));
    await sellerAction(listingId, action, version);
    expect(mocks.post).toHaveBeenCalledWith(
      `/marketplace/my-listings/${listingId}/${action}`,
      { expected_updated_at: version },
    );
  },
);
it.each(["approve", "reject", "suspend"] as const)(
  "matches the exact %s body",
  async (action) => {
    mocks.post.mockResolvedValue(ok(adminListing()));
    await adminAction(listingId, action, version, " Reason ");
    expect(mocks.post.mock.calls[0][1]).toEqual(
      action === "suspend"
        ? { reason: "Reason" }
        : {
            expected_updated_at: version,
            ...(action === "reject" ? { reason: "Reason" } : {}),
          },
    );
  },
);
it("keeps one key and confirmed price on retry and generates new keys for new intents", async () => {
  const intent = createPurchaseIntent(listingId, 20);
  expect(intent.key).toMatch(
    new RegExp(`^mkt1:${listingId}:20:[A-Za-z0-9_-]{16,64}$`),
  );
  mocks.post
    .mockRejectedValueOnce(new Error("reply lost"))
    .mockResolvedValue(ok(purchaseReceipt({ replayed: true })));
  await expect(purchaseListing(listingId, intent)).rejects.toThrow();
  expect((await purchaseListing(listingId, intent)).replayed).toBe(true);
  expect(mocks.post.mock.calls[0]).toEqual(mocks.post.mock.calls[1]);
  expect(mocks.post.mock.calls[0][1]).toEqual({ price_coins: 20 });
  expect(createPurchaseIntent(listingId, 20).key).not.toBe(intent.key);
});
it("accepts a replay with deleted avatar without manufacturing inventory", async () => {
  mocks.post.mockResolvedValue(
    ok(purchaseReceipt({ avatar: null, replayed: true })),
  );
  expect(
    (await purchaseListing(listingId, createPurchaseIntent(listingId, 20)))
      .avatar,
  ).toBeNull();
});
it("rejects malformed successful purchase receipts as outcome unknown", async () => {
  mocks.post.mockResolvedValue(ok({ balance: 0 }));
  let error: unknown;
  try {
    await purchaseListing(listingId, createPurchaseIntent(listingId, 20));
  } catch (caught) {
    error = caught;
  }
  expect(error).toBeDefined();
  expect(purchaseOutcomeUnknown(error)).toBe(true);
});
it("permits image endpoints only, never model grants or external URLs", () => {
  expect(previewSrc(catalogItem().preview_url)).toBe(
    `http://localhost:8080/marketplace/listings/${listingId}/preview`,
  );
  for (const path of [
    null,
    "https://example.com/model.vrm",
    "/inventory/avatars/a",
    "/marketplace/listings/../preview",
    "/marketplace/listings/abc/preview?model=1",
  ]) {
    expect(previewSrc(path)).toBeNull();
  }
});
describe("form boundaries", () => {
  it("trims fields, allows empty description, and counts emoji as code points", () => {
    expect(
      listingFormSchema.parse({ ...values, title: "🙂".repeat(100) }).title,
    ).toHaveLength(200);
    expect(
      listingFormSchema.safeParse({ ...values, title: "🙂".repeat(101) })
        .success,
    ).toBe(false);
  });
  it.each([0, -1, 1.2, 1_000_001, NaN])(
    "rejects invalid coin price %s",
    (price_coins) =>
      expect(
        listingFormSchema.safeParse({ ...values, price_coins }).success,
      ).toBe(false),
  );
  it("uses supplied translated validation messages", () => {
    const schema = createListingFormSchema((key) => `vi:${key}`);
    expect(
      schema.safeParse({ ...values, rights_note: "" }).error?.issues[0].message,
    ).toBe("vi:textLength");
  });
});
it("treats network/5xx failures as uncertain, confirmed 409 as refusals", () => {
  for (const status of [0, 500, 502]) {
    const error = new AxiosError("failed");
    if (status)
      error.response = {
        status,
        data: {},
        statusText: "",
        headers: {},
        config: { headers: new AxiosHeaders() },
      };
    expect(purchaseOutcomeUnknown(error)).toBe(true);
  }
  const error = new AxiosError("conflict");
  error.response = {
    status: 409,
    data: {},
    statusText: "",
    headers: {},
    config: { headers: new AxiosHeaders() },
  };
  expect(purchaseOutcomeUnknown(error)).toBe(false);
});
