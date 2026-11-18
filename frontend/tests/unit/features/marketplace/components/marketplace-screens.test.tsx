import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { renderWithIntl } from "@tests/helpers/intl";
import {
  adminListing,
  avatarId,
  buyerId,
  catalogItem,
  listingId,
  nextVersion,
  ok,
  pageOf,
  purchaseReceipt,
  sellerListing,
  version,
} from "@tests/fixtures/marketplace";
import {
  CatalogScreen,
  CatalogDetailScreen,
} from "@/features/marketplace/components/catalog-screen";
import { CheckoutDialog } from "@/features/marketplace/components/checkout-dialog";
import { MyListingsScreen } from "@/features/marketplace/components/my-listings-screen";
import { ListingFormScreen } from "@/features/marketplace/components/listing-form-screen";
import {
  AdminMarketplaceScreen,
  AdminMarketplaceDetail,
} from "@/features/marketplace/components/admin-marketplace-screen";
import { marketplaceKeys } from "@/features/marketplace/api/marketplace";
const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  push: vi.fn(),
  replace: vi.fn(),
  params: new URLSearchParams(),
  role: "candidate" as "candidate" | "recruiter" | "admin",
}));
vi.mock("@/lib/api/client", () => ({ apiClient: mocks }));
vi.mock("@/lib/api/config", () => ({
  getApiBaseUrl: () => "http://localhost:8080",
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.params,
  usePathname: () => "/marketplace",
  useRouter: () => ({ push: mocks.push, replace: mocks.replace }),
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: buyerId, role: mocks.role }),
}));
vi.mock("@/features/billing/components/wallet-purchase", () => ({
  AddCoinsDialog: ({ open }: { open: boolean }) =>
    open ? <div>Existing top-up</div> : null,
}));
vi.mock("@/features/avatar/components/buy-slots-dialog", () => ({
  BuySlotsDialog: ({
    open,
    onOpenChange,
  }: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
  }) =>
    open ? (
      <div>
        Existing slot purchase
        <button onClick={() => onOpenChange(false)}>Close slot purchase</button>
      </div>
    ) : null,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: vi.fn() } }));
function failure(code: string, status = 409) {
  const error = new AxiosError(code);
  error.response = {
    status,
    statusText: "",
    headers: {},
    config: { headers: new AxiosHeaders() },
    data: {
      success: false,
      error: { code, message: "Backend text must stay hidden" },
    },
  };
  return error;
}
function mount(node: React.ReactNode, locale: "en" | "vi" = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const view = renderWithIntl(
    <QueryClientProvider client={client}>{node}</QueryClientProvider>,
    { locale },
  );
  return { client, ...view };
}
beforeEach(() => {
  vi.resetAllMocks();
  mocks.params = new URLSearchParams();
  mocks.role = "candidate";
  mocks.get.mockImplementation((path: string) => {
    if (path === "/wallet")
      return Promise.resolve({
        data: {
          success: true,
          data: { id: buyerId, user_id: buyerId, balance: 100 },
        },
      });
    if (path === "/inventory")
      return Promise.resolve({
        data: {
          success: true,
          data: { capacity: 5, used: 1, available: 4, is_full: false },
        },
      });
    if (path === "/marketplace/listings")
      return Promise.resolve(ok(pageOf([catalogItem()])));
    if (path === "/marketplace/eligible-avatars")
      return Promise.resolve(ok([{ avatar_id: avatarId, name: "Own avatar" }]));
    if (path === "/marketplace/my-listings")
      return Promise.resolve(ok(pageOf([sellerListing()])));
    if (path.includes("/my-listings/"))
      return Promise.resolve(ok(sellerListing()));
    if (path === "/admin/marketplace/listings")
      return Promise.resolve(ok(pageOf([adminListing()])));
    if (path.includes("/admin/")) return Promise.resolve(ok(adminListing()));
    return Promise.resolve(ok(catalogItem()));
  });
  mocks.post.mockResolvedValue(ok(purchaseReceipt()));
  mocks.put.mockResolvedValue(ok(sellerListing({ updated_at: nextVersion })));
});
describe("catalog", () => {
  it.each(["en", "vi"] as const)(
    "renders public cards, fallback and navigation in %s without requesting models",
    async (locale) => {
      mount(<CatalogScreen />, locale);
      expect(
        await screen.findByRole("heading", { name: "Refund Bot" }),
      ).toBeVisible();
      fireEvent.error(screen.getByRole("img"));
      expect(
        screen.getByText(
          locale === "en"
            ? "Image preview unavailable"
            : "Chưa có ảnh xem trước",
        ),
      ).toBeVisible();
      expect(
        mocks.get.mock.calls.some(([path]) =>
          String(path).includes("/inventory/avatars"),
        ),
      ).toBe(false);
    },
  );
  it("submits search and resets pagination", async () => {
    mount(<CatalogScreen />);
    await screen.findByText("Refund Bot");
    fireEvent.change(screen.getByRole("textbox", { name: "Search avatars" }), {
      target: { value: "calm" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(mocks.push).toHaveBeenCalledWith("/marketplace?q=calm", {
      scroll: false,
    });
  });
  it.each(["owned", "own"] as const)(
    "hides purchase for %s listings",
    async (kind) => {
      mocks.get.mockResolvedValue(
        ok(
          catalogItem({
            already_owned: kind === "owned",
            is_own_listing: kind === "own",
          }),
        ),
      );
      mount(<CatalogDetailScreen id={listingId} />);
      await screen.findByRole("heading", { name: "Refund Bot" });
      expect(screen.queryByRole("button", { name: /Buy for/ })).toBeNull();
      expect(
        screen.getByRole("link", {
          name: kind === "own" ? "Your listing" : "Open my collection",
        }),
      ).toBeVisible();
    },
  );
  it("uses shared empty and error states", async () => {
    mocks.get.mockResolvedValue(ok(pageOf([])));
    const { unmount } = mount(<CatalogScreen />);
    expect(
      await screen.findByRole("status", { name: "No avatars here yet" }),
    ).toBeVisible();
    unmount();
    mocks.get.mockRejectedValue(failure("FORBIDDEN", 403));
    mount(<CatalogScreen />);
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.queryByText("Backend text must stay hidden")).toBeNull();
  });
});
describe("checkout", () => {
  it("deduplicates a double click, retries an uncertain result with the original key and refreshes inventory/wallet", async () => {
    let rejectFirst: (error: Error) => void = () => {};
    mocks.post
      .mockImplementationOnce(
        () =>
          new Promise((_, reject) => {
            rejectFirst = reject;
          }),
      )
      .mockResolvedValueOnce(ok(purchaseReceipt({ replayed: true })));
    const { client } = mount(
      <CheckoutDialog
        item={catalogItem()}
        onClose={vi.fn()}
        onReload={vi.fn()}
      />,
    );
    const button = await screen.findByRole("button", {
      name: "Confirm purchase · 20 coins",
    });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);
    fireEvent.click(button);
    await waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(1));
    await act(async () => rejectFirst(new Error("reply lost")));
    expect(
      await screen.findByText(/We could not confirm the outcome/),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByText("This purchase was already processed"),
    ).toBeVisible();
    expect(mocks.post.mock.calls[1]).toEqual(mocks.post.mock.calls[0]);
    expect(
      client.getQueryState(["inventory", buyerId, "summary"]),
    ).toBeDefined();
    expect(screen.getByText("80 coins")).toBeVisible();
  });
  it("shows a deleted-avatar replay as a receipt, never restoration", async () => {
    mocks.post.mockResolvedValue(
      ok(purchaseReceipt({ avatar: null, replayed: true })),
    );
    mount(
      <CheckoutDialog
        item={catalogItem()}
        onClose={vi.fn()}
        onReload={vi.fn()}
      />,
    );
    const button = await screen.findByRole("button", {
      name: /Confirm purchase/,
    });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);
    expect(await screen.findByText(/does not restore it/)).toBeVisible();
  });
  it.each([
    "MARKETPLACE_PRICE_CHANGED",
    "MARKETPLACE_LISTING_UNAVAILABLE",
    "MARKETPLACE_ASSET_NOT_READY",
    "MARKETPLACE_LISTING_NOT_FOUND",
    "MARKETPLACE_OWN_LISTING",
    "COIN_TRANSFER_KEY_CONFLICT",
    "COIN_TRANSFER_KEY_INVALID",
    "COIN_RECIPIENT_BALANCE_LIMIT",
    "FORBIDDEN",
  ])(
    "requires fresh review for %s instead of silently purchasing",
    async (code) => {
      mocks.post.mockRejectedValue(failure(code));
      mount(
        <CheckoutDialog
          item={catalogItem()}
          onClose={vi.fn()}
          onReload={vi.fn()}
        />,
      );
      const button = await screen.findByRole("button", {
        name: /Confirm purchase/,
      });
      await waitFor(() => expect(button).toBeEnabled());
      fireEvent.click(button);
      expect(
        await screen.findByRole("button", { name: "Reload listing" }),
      ).toBeVisible();
      expect(
        screen.queryByRole("button", { name: /Confirm purchase/ }),
      ).toBeNull();
      expect(mocks.post).toHaveBeenCalledTimes(1);
    },
  );
  it("offers inventory access for already-owned refusal", async () => {
    mocks.post.mockRejectedValue(failure("MARKETPLACE_ALREADY_OWNED"));
    mount(
      <CheckoutDialog
        item={catalogItem()}
        onClose={vi.fn()}
        onReload={vi.fn()}
      />,
    );
    const button = await screen.findByRole("button", {
      name: /Confirm purchase/,
    });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);
    expect(
      await screen.findByRole("link", { name: "Open my collection" }),
    ).toBeVisible();
  });
  it.each(["INSUFFICIENT_BALANCE", "INVENTORY_FULL"])(
    "offers existing recovery flow for %s",
    async (code) => {
      mocks.post.mockRejectedValue(
        failure(code, code === "INSUFFICIENT_BALANCE" ? 422 : 409),
      );
      mount(
        <CheckoutDialog
          item={catalogItem()}
          onClose={vi.fn()}
          onReload={vi.fn()}
        />,
      );
      const button = await screen.findByRole("button", {
        name: /Confirm purchase/,
      });
      await waitFor(() => expect(button).toBeEnabled());
      fireEvent.click(button);
      const recovery = await screen.findByRole("button", {
        name: code === "INSUFFICIENT_BALANCE" ? "Add coins" : "Buy more slots",
      });
      fireEvent.click(recovery);
      expect(
        screen.getByText(
          code === "INSUFFICIENT_BALANCE"
            ? "Existing top-up"
            : "Existing slot purchase",
        ),
      ).toBeVisible();
    },
  );
});
describe("seller", () => {
  it("lists every lifecycle state without inventing edit or relist actions", async () => {
    const statuses = [
      "draft",
      "pending",
      "active",
      "rejected",
      "delisted",
      "suspended",
    ] as const;
    mocks.get.mockResolvedValue(
      ok(
        pageOf(
          statuses.map((status, i) =>
            sellerListing({
              id: `1111111${i}-1111-4111-8111-111111111111`,
              status,
            }),
          ),
        ),
      ),
    );
    mount(<MyListingsScreen />);
    expect(await screen.findByText("Changes requested")).toBeVisible();
    expect(screen.getByText("Delisted")).toBeVisible();
    expect(screen.getByText("Suspended")).toBeVisible();
    expect(screen.queryByText("Relist")).toBeNull();
  });
  it("shows eligibility empty state", async () => {
    mocks.get.mockResolvedValue(ok([]));
    mount(<ListingFormScreen />);
    expect(
      await screen.findByText("No eligible avatars to sell"),
    ).toBeVisible();
    expect(screen.queryByRole("button", { name: "Save draft" })).toBeNull();
  });
  it("edits rejected payload using the original token, then submits the saved token", async () => {
    const rejected = sellerListing({
      status: "rejected",
      review_note: "Clarify the license.",
    });
    mocks.get.mockImplementation(() => Promise.resolve(ok(rejected)));
    mocks.put.mockImplementation(async () => {
      const saved = {
        ...rejected,
        rights_note: "Updated commercial license",
        updated_at: nextVersion,
      };
      mocks.get.mockResolvedValue(ok(saved));
      return ok(saved);
    });
    mocks.post.mockResolvedValue(
      ok({
        ...rejected,
        status: "pending",
        can_edit: false,
        can_submit: false,
        updated_at: nextVersion,
      }),
    );
    mount(<ListingFormScreen id={listingId} />);
    expect(await screen.findByText("Clarify the license.")).toBeVisible();
    fireEvent.change(screen.getByLabelText("Rights declaration"), {
      target: { value: "Updated commercial license" },
    });
    expect(
      screen.getByRole("button", { name: "Submit for review" }),
    ).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(mocks.put).toHaveBeenCalled());
    expect(mocks.put.mock.calls[0][1].expected_updated_at).toBe(version);
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Submit for review" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Submit for review" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Submit for review" }),
    );
    await waitFor(() => expect(mocks.post).toHaveBeenCalled());
    expect(mocks.post.mock.calls[0][1]).toEqual({
      expected_updated_at: nextVersion,
    });
  });
  it("keeps unsaved fields on stale saves and asks before discarding them", async () => {
    mocks.put.mockRejectedValue(failure("MARKETPLACE_LISTING_STALE"));
    mount(<ListingFormScreen id={listingId} />);
    await screen.findByDisplayValue("Refund Bot");
    fireEvent.change(screen.getByLabelText("Listing title"), {
      target: { value: "My unsaved edit" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("This listing changed")).toBeVisible();
    expect(screen.getByLabelText("Listing title")).toHaveValue(
      "My unsaved edit",
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Reload latest details" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Discard unsaved changes?" }),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Keep editing" }));
    expect(screen.getByLabelText("Listing title")).toHaveValue(
      "My unsaved edit",
    );
  });
  it.each(["pending", "active", "delisted", "suspended"] as const)(
    "keeps %s listings read only according to capability flags",
    async (status) => {
      mocks.get.mockResolvedValue(
        ok(
          sellerListing({
            status,
            can_edit: false,
            can_submit: false,
            can_delist: status === "active",
          }),
        ),
      );
      mount(<ListingFormScreen id={listingId} />);
      await screen.findByRole("heading", { name: "Refund Bot" });
      expect(screen.queryByRole("textbox")).toBeNull();
      expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();
      expect(screen.queryAllByRole("button", { name: "Delist" })).toHaveLength(
        status === "active" ? 1 : 0,
      );
    },
  );
});
describe("admin", () => {
  it("defaults the queue to pending and respects status pagination", async () => {
    mocks.role = "admin";
    mount(<AdminMarketplaceScreen />);
    await screen.findByRole("heading", { name: "Refund Bot" });
    expect(mocks.get.mock.calls[0][1].params).toMatchObject({
      status: "pending",
      offset: 0,
    });
  });
  it("requires a rejection reason and handles stale review without reporting success", async () => {
    mocks.role = "admin";
    mocks.post.mockRejectedValue(failure("MARKETPLACE_LISTING_STALE"));
    mount(<AdminMarketplaceDetail id={listingId} />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Reject listing" }),
    );
    const dialog = within(screen.getByRole("dialog"));
    fireEvent.click(dialog.getByRole("button", { name: "Reject listing" }));
    expect(await dialog.findByText(/Enter a reason/)).toBeVisible();
    expect(mocks.post).not.toHaveBeenCalled();
    fireEvent.change(dialog.getByLabelText("Reason for the seller"), {
      target: { value: "Provide a valid commercial license." },
    });
    fireEvent.click(dialog.getByRole("button", { name: "Reject listing" }));
    expect(await dialog.findByText("This listing changed")).toBeVisible();
    expect(mocks.post.mock.calls[0][1]).toEqual({
      expected_updated_at: version,
      reason: "Provide a valid commercial license.",
    });
    expect(
      dialog.getByRole("button", { name: "Reject listing" }),
    ).toBeDisabled();
  });
  it("shows the exact private rights payload only in admin detail and disables approval for unready assets", async () => {
    mocks.get.mockResolvedValue(ok(adminListing({ asset_ready: false })));
    mount(<AdminMarketplaceDetail id={listingId} />);
    expect(
      await screen.findByText(
        "I created this avatar and own its commercial rights.",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Approve listing" }),
    ).toBeDisabled();
  });
  it("does not approve a replacement version that was never reviewed", async () => {
    const { client } = mount(<AdminMarketplaceDetail id={listingId} />);
    await screen.findByRole("heading", { name: "Refund Bot" });
    act(() =>
      client.setQueryData(
        marketplaceKeys.review(buyerId, listingId),
        adminListing({
          updated_at: nextVersion,
          title: "Changed by another admin",
        }),
      ),
    );
    expect(await screen.findByText("This listing changed")).toBeVisible();
    expect(screen.getByRole("heading", { name: "Refund Bot" })).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Approve listing" }),
    ).toBeDisabled();
  });
});

it("preserves unsaved seller fields when a background reload fails", async () => {
  const { client } = mount(<ListingFormScreen id={listingId} />);
  await screen.findByDisplayValue("Refund Bot");
  fireEvent.change(screen.getByLabelText("Listing title"), {
    target: { value: "Keep this edit" },
  });
  mocks.get.mockRejectedValue(new Error("offline"));
  await act(async () => {
    await client.invalidateQueries({
      queryKey: marketplaceKeys.listing(buyerId, listingId),
    });
  });
  expect(await screen.findByRole("alert")).toBeVisible();
  expect(screen.getByLabelText("Listing title")).toHaveValue("Keep this edit");
});
it("allows a confirmed inventory-full refusal to be retried after using the capacity dialog", async () => {
  mocks.post
    .mockRejectedValueOnce(failure("INVENTORY_FULL"))
    .mockResolvedValueOnce(ok(purchaseReceipt()));
  mount(
    <CheckoutDialog
      item={catalogItem()}
      onClose={vi.fn()}
      onReload={vi.fn()}
    />,
  );
  const buy = await screen.findByRole("button", { name: /Confirm purchase/ });
  await waitFor(() => expect(buy).toBeEnabled());
  fireEvent.click(buy);
  fireEvent.click(
    await screen.findByRole("button", { name: "Buy more slots" }),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Close slot purchase", hidden: true }),
  );
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: /Confirm purchase/ }),
    ).toBeEnabled(),
  );
  fireEvent.click(screen.getByRole("button", { name: /Confirm purchase/ }));
  expect(
    await screen.findByText("Avatar added to your collection"),
  ).toBeVisible();
  expect(mocks.post.mock.calls[0][2]).toEqual(mocks.post.mock.calls[1][2]);
});
it("invalidates the recruiter avatar picker after a purchase", async () => {
  mocks.role = "recruiter";
  mocks.post.mockResolvedValue(ok(purchaseReceipt()));
  const { client } = mount(
    <CheckoutDialog
      item={catalogItem()}
      onClose={vi.fn()}
      onReload={vi.fn()}
    />,
  );
  const invalidate = vi.spyOn(client, "invalidateQueries");
  const buy = await screen.findByRole("button", { name: /Confirm purchase/ });
  await waitFor(() => expect(buy).toBeEnabled());
  fireEvent.click(buy);
  await screen.findByText("Avatar added to your collection");
  expect(invalidate).toHaveBeenCalledWith({
    queryKey: ["recruiter-job-postings", buyerId, "avatars"],
  });
});
