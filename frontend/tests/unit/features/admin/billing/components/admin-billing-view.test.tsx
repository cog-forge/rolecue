import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminBillingView } from "@/features/admin/billing/components/admin-billing-view";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  toastWarning: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: mocks.get,
  },
}));

vi.mock("goey-toast", () => ({
  gooeyToast: {
    warning: mocks.toastWarning,
    error: mocks.toastError,
  },
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
      <AdminBillingView />
    </QueryClientProvider>,
  );
}

describe("AdminBillingView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/billing/summary") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              total_revenue: 50000,
              net_revenue: 50000,
              successful_deposits: 1,
              success_count: 2,
              pending_count: 1,
              failed_count: 0,
              cancelled_count: 0,
              total_transactions: 3,
              average_deposit: 50000,
              unique_payers: 1,
              coins_sold: 100,
              coins_spent: 20,
              coins_refunded: 0,
              coins_net: 80,
              success_rate: 66.7,
              package_breakdown: [
                {
                  package_name: "Starter Pack",
                  revenue_vnd: 50000,
                  deposit_count: 1,
                  coins_sold: 100,
                },
              ],
              feature_breakdown: [
                {
                  feature_name: "Practice interview charge",
                  coins_spent: 20,
                  spend_count: 1,
                },
              ],
              schema_limitation:
                "public.transactions in the frozen schema (000006_wallet_payment) does not store a created_at timestamp.",
            },
          },
        });
      }
      if (url === "/admin/billing/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              items: [
                {
                  id: "00000000-0000-0000-0000-000000000001",
                  from: "",
                  to: "candidate-user-1",
                  user_id: "candidate-user-1",
                  user_name: "Alice Nguyen",
                  user_email: "alice@example.com",
                  type: "topup",
                  package_or_feature: "Starter Pack",
                  paid_vnd: 50000,
                  coin_amount: 100,
                  amount: 100,
                  currency: "VND",
                  description: "Deposit: Starter Pack (50000 VND)",
                  status: "success",
                  payos_order_code: 123456789,
                },
                {
                  id: "00000000-0000-0000-0000-000000000002",
                  from: "candidate-user-1",
                  to: "",
                  user_id: "candidate-user-1",
                  user_name: "Alice Nguyen",
                  user_email: "alice@example.com",
                  type: "spend",
                  package_or_feature: "Practice interview charge",
                  paid_vnd: 0,
                  coin_amount: -20,
                  amount: 20,
                  currency: "VND",
                  description: "Practice interview charge",
                  status: "success",
                  payos_order_code: null,
                },
              ],
              total: 2,
              page: 1,
              page_size: 20,
              schema_limitation:
                "public.transactions in the frozen schema (000006_wallet_payment) does not store a created_at timestamp.",
            },
          },
        });
      }
      if (
        url ===
        "/admin/billing/transactions/00000000-0000-0000-0000-000000000001"
      ) {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              id: "00000000-0000-0000-0000-000000000001",
              from: "",
              to: "candidate-user-1",
              user_id: "candidate-user-1",
              user_name: "Alice Nguyen",
              user_email: "alice@example.com",
              type: "topup",
              package_or_feature: "Starter Pack",
              paid_vnd: 50000,
              coin_amount: 100,
              amount: 100,
              currency: "VND",
              description: "Deposit: Starter Pack (50000 VND)",
              status: "success",
              payos_order_code: 123456789,
            },
          },
        });
      }
      return Promise.reject(new Error(`Unhandled URL: ${url}`));
    });
  });

  it("renders transactions, separate VND/Coins columns, and read-only metrics", async () => {
    renderComponent();

    expect(
      screen.getByRole("heading", { name: "Payment Transactions" }),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Total Revenue")).toBeInTheDocument();
      expect(screen.getAllByText("50.000 ₫").length).toBeGreaterThanOrEqual(1);
    });

    await waitFor(() => {
      expect(screen.getByText("Starter Pack")).toBeInTheDocument();
      expect(screen.getByText("Practice interview charge")).toBeInTheDocument();
    });

    expect(screen.getByText("Order #123456789")).toBeInTheDocument();
    expect(screen.getByText("+100 coins")).toBeInTheDocument();
    expect(screen.getByText("-20 coins")).toBeInTheDocument();
  });

  it("filters transactions and summary with unified parameters", async () => {
    renderComponent();

    const searchInput = await screen.findByPlaceholderText(
      /Search by user name, email, order code/i,
    );
    fireEvent.change(searchInput, { target: { value: "alice@example.com" } });

    await waitFor(() => {
      expect(mocks.get).toHaveBeenCalledWith(
        "/admin/billing/transactions",
        expect.objectContaining({
          params: expect.objectContaining({
            search: "alice@example.com",
          }),
        }),
      );
      expect(mocks.get).toHaveBeenCalledWith(
        "/admin/billing/summary",
        expect.objectContaining({
          params: expect.objectContaining({
            search: "alice@example.com",
          }),
        }),
      );
    });
  });

  it("opens read-only details drawer on row click and fetches transaction detail without mutation buttons", async () => {
    renderComponent();

    const row = await screen.findByText("Starter Pack");
    fireEvent.click(row);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Transaction Details" }),
      ).toBeInTheDocument();
    });

    expect(mocks.get).toHaveBeenCalledWith(
      "/admin/billing/transactions/00000000-0000-0000-0000-000000000001",
      expect.any(Object),
    );

    // Ensure no mutation actions exist (KAN-191 read-only enforcement)
    expect(
      screen.queryByRole("button", { name: /re-check with payos/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /mark cancelled/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /issue refund/i }),
    ).not.toBeInTheDocument();
  });

  it("handles empty state gracefully", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/billing/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              items: [],
              total: 0,
              page: 1,
              page_size: 20,
              schema_limitation: "",
            },
          },
        });
      }
      if (url === "/admin/billing/summary") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              total_revenue: 0,
              net_revenue: 0,
              successful_deposits: 0,
              success_count: 0,
              pending_count: 0,
              failed_count: 0,
              cancelled_count: 0,
              total_transactions: 0,
              average_deposit: 0,
              unique_payers: 0,
              coins_sold: 0,
              coins_spent: 0,
              coins_refunded: 0,
              coins_net: 0,
              success_rate: 0,
              package_breakdown: [],
              feature_breakdown: [],
            },
          },
        });
      }
      return Promise.reject(new Error("Unhandled"));
    });

    renderComponent();

    await waitFor(() => {
      expect(
        screen.getByText("No transactions match the current filters."),
      ).toBeInTheDocument();
    });
  });

  it("handles error state with retry button", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/billing/transactions") {
        return Promise.reject(new Error("Database connection failure"));
      }
      return Promise.resolve({
        data: {
          success: true,
          data: {
            total_revenue: 0,
            successful_deposits: 0,
            total_transactions: 0,
            package_breakdown: [],
            feature_breakdown: [],
          },
        },
      });
    });

    renderComponent();

    await waitFor(() => {
      expect(
        screen.getByText("Unable to load transactions"),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /retry/i }),
      ).toBeInTheDocument();
    });
  });

  it("shows a warning toast when CSV export is truncated via X-Export-Truncated header", async () => {
    const createObjectURLSpy = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:mock-url");
    const revokeObjectURLSpy = vi
      .spyOn(URL, "revokeObjectURL")
      .mockImplementation(() => {});
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});

    const baseImpl = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/billing/export") {
        return Promise.resolve({
          data: "Transaction ID,Order Code\n1,123",
          headers: {
            "x-export-truncated": "true",
            "x-total-count": "6200",
          },
        });
      }
      return baseImpl(url);
    });

    renderComponent();

    const exportBtn = await screen.findByRole("button", {
      name: /export csv/i,
    });
    fireEvent.click(exportBtn);

    await waitFor(() => {
      expect(mocks.toastWarning).toHaveBeenCalledWith(
        "Export truncated to 5,000 rows",
        expect.objectContaining({
          description: expect.stringContaining("6,200"),
        }),
      );
    });

    createObjectURLSpy.mockRestore();
    revokeObjectURLSpy.mockRestore();
    clickSpy.mockRestore();
  });

  it("shows an error toast when CSV export fails with 400 or 500", async () => {
    const baseImpl = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/billing/export") {
        return Promise.reject(new Error("Internal Server Error"));
      }
      return baseImpl(url);
    });

    renderComponent();

    const exportBtn = await screen.findByRole("button", {
      name: /export csv/i,
    });
    fireEvent.click(exportBtn);

    await waitFor(() => {
      expect(mocks.toastError).toHaveBeenCalledWith(
        "Unable to export CSV",
        expect.any(Object),
      );
    });
  });
});
