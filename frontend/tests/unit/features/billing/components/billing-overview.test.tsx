import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WalletPurchase } from "@/features/billing/components/wallet-purchase";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BillingOverview } from "@/features/billing/components/billing-overview";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  searchParams: new URLSearchParams(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    email: "candidate@example.com",
    full_name: "Alex Doe",
    role: "candidate" as "candidate" | "recruiter" | "admin",
    email_verified: true,
    is_locked: false,
    image: null,
    onboarding_role_selected: true,
    onboarding_completed: true,
  },
}));

vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: mocks.get,
    post: mocks.post,
  },
}));

vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.searchParams,
}));

function renderComponent() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <WalletPurchase
          userId={mocks.user.id}
          role={mocks.user.role}
          collapsed={false}
        />
        <BillingOverview />
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

describe("BillingOverview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.user.role = "candidate";
    mocks.searchParams = new URLSearchParams();
    mocks.post.mockImplementation(
      (url: string, body: { order_code: number }) => {
        if (url === "/wallet/deposit/sync") {
          return Promise.resolve({
            data: {
              success: true,
              data: {
                id: "tx-sync",
                amount: 100,
                status: "cancelled",
                payos_order_code: body.order_code,
              },
            },
          });
        }
        return Promise.reject(new Error("unexpected post"));
      },
    );
  });

  it("renders Admin isolation view when user is an administrator", () => {
    mocks.user.role = "admin";
    renderComponent();

    expect(
      screen.getByText("Administrator Accounts Do Not Have a Personal Wallet"),
    ).toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalled();
  });

  it("loads and displays wallet balance, packages, and transactions", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url === "/wallet") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              id: "22222222-2222-4222-8222-222222222222",
              user_id: mocks.user.id,
              balance: 250,
            },
          },
        });
      }
      if (url === "/wallet/packages") {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: "33333333-3333-4333-8333-333333333333",
                name: "Basic Pack",
                price_vnd: 50000,
                coin_amount: 100,
                description: "100 practice coins",
                currency: "VND",
                is_active: true,
              },
            ],
          },
        });
      }
      if (url === "/wallet/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              items: [
                {
                  id: "44444444-4444-4444-8444-444444444444",
                  from: null,
                  to: mocks.user.id,
                  amount: 100,
                  currency: "COIN",
                  description: "Deposit: Basic Pack",
                  status: "success",
                  payos_order_code: 100200,
                  created_at: "2026-10-15T10:00:00Z",
                },
                {
                  id: "55555555-5555-4555-8555-555555555555",
                  from: mocks.user.id,
                  to: null,
                  amount: 25,
                  currency: "COIN",
                  description: "System Design Mock Interview",
                  status: "pending",
                  payos_order_code: null,
                  created_at: "2026-10-16T14:30:00Z",
                },
              ],
              total: 2,
              page: 1,
              page_size: 20,
            },
          },
        });
      }
      return Promise.reject(new Error("unknown url"));
    });

    renderComponent();

    // 1. Balance card
    await waitFor(() => {
      expect(screen.getByText("250")).toBeInTheDocument();
    });

    // 3. Transactions ledger
    expect(screen.getByText("Deposit: Basic Pack")).toBeInTheDocument();
    expect(screen.getByText("Order #100200")).toBeInTheDocument();
    expect(
      screen.getByRole("cell", { name: "+100 Coins" }),
    ).toBeInTheDocument();

    expect(
      screen.getByText("System Design Mock Interview"),
    ).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "-25 Coins" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Buy coins" }));
    await screen.findByText("Basic Pack");
    // 2. Package grid
    expect(screen.getByText("Basic Pack")).toBeInTheDocument();
  });

  it("initiates PayOS checkout when clicking a package", async () => {
    const locationMock = {
      origin: "http://localhost:3000",
      pathname: "/billing",
      href: "",
    };
    vi.stubGlobal("location", locationMock);

    mocks.get.mockImplementation((url: string) => {
      if (url === "/wallet") {
        return Promise.resolve({
          data: {
            success: true,
            data: { id: "w-1", user_id: mocks.user.id, balance: 100 },
          },
        });
      }
      if (url === "/wallet/packages") {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: "33333333-3333-4333-8333-333333333333",
                name: "Pro Pack",
                price_vnd: 100000,
                coin_amount: 250,
                is_active: true,
              },
            ],
          },
        });
      }
      if (url === "/wallet/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: { items: [], total: 0, page: 1, page_size: 20 },
          },
        });
      }
      return Promise.reject(new Error("unknown url"));
    });

    mocks.post.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          checkout_url: "https://pay.payos.vn/checkout-session-123",
          order_code: 100300,
          amount: 100000,
          coin_amount: 250,
          transaction_id: "tx-new",
        },
      },
    });

    renderComponent();

    fireEvent.click(screen.getByRole("button", { name: "Buy coins" }));
    const buyButton = await screen.findByRole("button", {
      name: "Buy Pro Pack with PayOS",
    });
    fireEvent.click(buyButton);

    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalledWith("/wallet/deposit/checkout", {
        package_id: "33333333-3333-4333-8333-333333333333",
        return_url: "http://localhost:3000/billing?status=success",
        cancel_url: "http://localhost:3000/billing?cancel=true",
      });
      expect(locationMock.href).toBe(
        "https://pay.payos.vn/checkout-session-123",
      );
    });

    vi.unstubAllGlobals();
  });

  it("offers coin packages from the transactions page, not only the sidebar", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url === "/wallet")
        return Promise.resolve({
          data: {
            success: true,
            data: { id: "w-1", user_id: mocks.user.id, balance: 0 },
          },
        });
      if (url === "/wallet/packages")
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                id: "33333333-3333-4333-8333-333333333333",
                name: "Pro Pack",
                price_vnd: 100000,
                coin_amount: 250,
                is_active: true,
              },
            ],
          },
        });
      if (url === "/wallet/transactions")
        return Promise.resolve({
          data: {
            success: true,
            data: { items: [], total: 0, page: 1, page_size: 20 },
          },
        });
      return Promise.reject(new Error("unknown url"));
    });

    renderComponent();

    fireEvent.click(screen.getByRole("button", { name: "Add coins" }));
    expect(
      await screen.findByRole("button", { name: "Buy Pro Pack with PayOS" }),
    ).toBeInTheDocument();
  });

  it("renders payment success only after backend verification", async () => {
    mocks.post.mockResolvedValueOnce({
      data: {
        success: true,
        data: {
          id: "verified",
          amount: 100,
          status: "success",
          payos_order_code: 999888,
        },
      },
    });
    mocks.searchParams = new URLSearchParams(
      "status=PAID&orderCode=999888&code=00",
    );

    mocks.get.mockImplementation((url: string) => {
      if (url === "/wallet") {
        return Promise.resolve({
          data: {
            success: true,
            data: { id: "w-1", user_id: mocks.user.id, balance: 350 },
          },
        });
      }
      if (url === "/wallet/packages") {
        return Promise.resolve({
          data: { success: true, data: [] },
        });
      }
      if (url === "/wallet/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: { items: [], total: 0, page: 1, page_size: 20 },
          },
        });
      }
      return Promise.reject(new Error("unknown url"));
    });

    renderComponent();

    expect(
      await screen.findByText("Payment Completed Successfully"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Order #999888 was processed. Your coin balance has been credited.",
      ),
    ).toBeInTheDocument();

    const dismissBtn = screen.getByRole("button", {
      name: /Dismiss payment notification/i,
    });
    fireEvent.click(dismissBtn);

    expect(
      screen.queryByText("Payment Completed Successfully"),
    ).not.toBeInTheDocument();

    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalledWith("/wallet/deposit/sync", {
        order_code: 999888,
      });
    });
  });

  it("renders cancelled payment return banner", async () => {
    mocks.searchParams = new URLSearchParams("cancel=true&orderCode=555444");

    mocks.get.mockImplementation((url: string) => {
      if (url === "/wallet") {
        return Promise.resolve({
          data: {
            success: true,
            data: { id: "w-1", user_id: mocks.user.id, balance: 100 },
          },
        });
      }
      if (url === "/wallet/packages") {
        return Promise.resolve({
          data: { success: true, data: [] },
        });
      }
      if (url === "/wallet/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: { items: [], total: 0, page: 1, page_size: 20 },
          },
        });
      }
      return Promise.reject(new Error("unknown url"));
    });

    renderComponent();

    expect(await screen.findByText("Payment Cancelled")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Payment for Order #555444 was cancelled. No coins were credited.",
      ),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalledWith("/wallet/deposit/sync", {
        order_code: 555444,
      });
    });
  });

  it("filters transactions by sliding tab", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url === "/wallet") {
        return Promise.resolve({
          data: {
            success: true,
            data: { id: "w-1", user_id: mocks.user.id, balance: 100 },
          },
        });
      }
      if (url === "/wallet/packages") {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === "/wallet/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              items: [
                {
                  id: "tx-1",
                  from: null,
                  to: mocks.user.id,
                  amount: 100,
                  currency: "COIN",
                  description: "Deposit: Starter Package",
                  status: "success",
                  payos_order_code: 100,
                  created_at: "2026-10-15T10:00:00Z",
                },
                {
                  id: "tx-2",
                  from: mocks.user.id,
                  to: null,
                  amount: 20,
                  currency: "COIN",
                  description: "Interview Practice Question",
                  status: "success",
                  payos_order_code: null,
                  created_at: "2026-10-15T11:00:00Z",
                },
              ],
              total: 2,
              page: 1,
              page_size: 20,
            },
          },
        });
      }
      return Promise.reject(new Error("unknown url"));
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText("Deposit: Starter Package")).toBeInTheDocument();
      expect(
        screen.getByText("Interview Practice Question"),
      ).toBeInTheDocument();
    });

    // Switch to Coin Spending tab
    const spendingTab = screen.getByRole("tab", { name: "Coin Spending" });
    fireEvent.mouseDown(spendingTab, { button: 0, ctrlKey: false });

    expect(
      screen.queryByText("Deposit: Starter Package"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Interview Practice Question")).toBeInTheDocument();

    // Switch to Top-ups tab
    const topupsTab = screen.getByRole("tab", {
      name: "Top-ups (PayOS)",
    });
    fireEvent.mouseDown(topupsTab, { button: 0, ctrlKey: false });

    expect(screen.getByText("Deposit: Starter Package")).toBeInTheDocument();
    expect(
      screen.queryByText("Interview Practice Question"),
    ).not.toBeInTheDocument();
  });
});
