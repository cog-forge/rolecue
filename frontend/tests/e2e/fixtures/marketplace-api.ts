import { expect, type Page } from "@playwright/test";
import {
  adminListing,
  avatarId,
  buyerId,
  catalogItem,
  listingId,
  nextVersion,
  pageOf,
  purchaseReceipt,
  sellerListing,
  version,
} from "../../fixtures/marketplace";
import type { ListingStatus } from "@/features/marketplace/schemas/marketplace-schema";
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,PUT,OPTIONS",
  "access-control-allow-headers": "content-type,idempotency-key",
};
export async function mockMarketplace(
  page: Page,
  options: {
    role?: "candidate" | "recruiter" | "admin";
    status?: ListingStatus;
    owned?: boolean;
    own?: boolean;
    lostReply?: boolean;
    purchaseCode?: string;
    staleReview?: boolean;
    balance?: number;
    full?: boolean;
    empty?: boolean;
    failed?: boolean;
    unready?: boolean;
  } = {},
) {
  let current = sellerListing({
    status: options.status ?? (options.role === "admin" ? "pending" : "draft"),
  });
  let bought = options.owned ?? false,
    price = 20,
    balance = options.balance ?? 100,
    capacity = options.full ? 1 : 5;
  let lost = options.lostReply ?? false,
    purchaseCode = options.purchaseCode;
  const writes: {
    path: string;
    body: Record<string, unknown>;
    key: string | undefined;
  }[] = [];
  const reads: string[] = [];
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request(),
      url = new URL(request.url()),
      path = url.pathname,
      method = request.method();
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });
    const fail = (code: string, status = 409) =>
      route.fulfill({
        headers,
        status,
        json: {
          success: false,
          error: { code, message: "Private server text" },
        },
      });
    if (method === "OPTIONS") return route.fulfill({ headers, status: 204 });
    if (method === "GET") reads.push(path + url.search);
    if (path === "/auth/me")
      return ok({
        id: buyerId,
        email: "review@rolecue.test",
        full_name: "Marketplace Review",
        role: options.role ?? "candidate",
        email_verified: true,
        is_locked: false,
        onboarding_role_selected: true,
        onboarding_completed: true,
        image: null,
      });
    if (path === "/wallet")
      return ok({ id: buyerId, user_id: buyerId, balance });
    if (path === "/wallet/packages")
      return ok([
        {
          id: avatarId,
          name: "Starter",
          price_vnd: 10000,
          coin_amount: 100,
          is_active: true,
        },
      ]);
    if (path === "/wallet/transactions")
      return ok({ items: [], total: 0, page: 1, page_size: 20 });
    const summary = () => ({
      capacity,
      used: bought ? 2 : 1,
      available: Math.max(capacity - (bought ? 2 : 1), 0),
      is_full: capacity <= (bought ? 2 : 1),
    });
    if (path === "/inventory") return ok(summary());
    if (path === "/inventory/avatars")
      return ok({
        items: bought ? [purchaseReceipt().avatar] : [],
        pagination: { limit: 12, offset: 0, total: bought ? 1 : 0 },
      });
    if (path.endsWith("/preview"))
      return route.fulfill({ status: 404, headers });
    if (path === "/marketplace/listings")
      return options.failed
        ? fail("INTERNAL_ERROR", 500)
        : ok(
            pageOf(
              options.empty
                ? []
                : [
                    catalogItem({
                      price_coins: price,
                      already_owned: bought,
                      is_own_listing: options.own ?? false,
                    }),
                  ],
            ),
          );
    if (path === `/marketplace/listings/${listingId}`)
      return ok(
        catalogItem({
          price_coins: price,
          already_owned: bought,
          is_own_listing: options.own ?? false,
        }),
      );
    if (path === "/marketplace/eligible-avatars")
      return ok(
        options.empty ? [] : [{ avatar_id: avatarId, name: "Own avatar" }],
      );
    if (path === "/marketplace/my-listings" && method === "GET")
      return ok(pageOf([current]));
    if (path === `/marketplace/my-listings/${listingId}` && method === "GET")
      return ok(current);
    if (path === "/admin/marketplace/listings")
      return ok(
        pageOf([
          adminListing({
            ...current,
            asset_ready: !options.unready,
            status: current.status,
          }),
        ]),
      );
    if (path === `/admin/marketplace/listings/${listingId}` && method === "GET")
      return ok(
        adminListing({
          ...current,
          asset_ready: !options.unready,
          status: current.status,
        }),
      );
    if (method === "POST" || method === "PUT") {
      const body = request.postDataJSON() as Record<string, unknown>;
      writes.push({ path, body, key: request.headers()["idempotency-key"] });
      if (
        path.endsWith("/purchase") &&
        path.startsWith("/marketplace/listings/")
      ) {
        if (purchaseCode) {
          const code = purchaseCode;
          purchaseCode = undefined;
          if (code === "MARKETPLACE_PRICE_CHANGED") price = 30;
          return fail(code);
        }
        if (lost) {
          lost = false;
          bought = true;
          balance -= price;
          return route.abort("failed");
        }
        const replayed = bought;
        bought = true;
        if (!replayed) balance -= price;
        return ok(
          purchaseReceipt({
            balance,
            replayed,
            transaction: { ...purchaseReceipt().transaction, amount: price },
          }),
        );
      }
      if (path === "/inventory/purchase") {
        capacity += Number(body.amount);
        return ok({ ...summary(), balance: balance - Number(body.amount) });
      }
      if (path.startsWith("/marketplace/my-listings")) {
        if (method === "PUT" || path === "/marketplace/my-listings") {
          current = { ...current, ...body, updated_at: nextVersion };
          return ok(current);
        }
        expect(body.expected_updated_at).toBe(current.updated_at);
        current = {
          ...current,
          status: path.endsWith("/submit") ? "pending" : "delisted",
          can_edit: false,
          can_submit: false,
          can_delist: false,
          updated_at: nextVersion,
        };
        return ok(current);
      }
      if (path.startsWith("/admin/marketplace/listings/")) {
        if (options.staleReview) return fail("MARKETPLACE_LISTING_STALE");
        const status = path.endsWith("/approve")
          ? "active"
          : path.endsWith("/reject")
            ? "rejected"
            : "suspended";
        current = {
          ...current,
          status,
          updated_at: nextVersion,
          review_note: typeof body.reason === "string" ? body.reason : null,
        };
        return ok(adminListing({ ...current }));
      }
    }
    return fail("NOT_MOCKED", 404);
  });
  return {
    writes,
    reads,
    get version() {
      return version;
    },
  };
}
