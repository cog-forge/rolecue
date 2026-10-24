import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminBillingView } from "@/features/admin/billing/components/admin-billing-view";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: mocks.get,
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
      if (url === "/admin/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              items: [
                {
                  id: "00000000-0000-0000-0000-000000000001",
                  from: null,
                  to: "candidate-user-1",
                  amount: 100,
                  currency: "COIN",
                  description: "Deposit: Starter (10000 VND)",
                  status: "success",
                  payos_order_code: 123456789,
                  created_at: "2026-10-21T10:00:00Z",
                  updated_at: "2026-10-21T10:05:00Z",
                },
                {
                  id: "00000000-0000-0000-0000-000000000002",
                  from: "candidate-user-1",
                  to: null,
                  amount: 20,
                  currency: "COIN",
                  description: "Practice interview charge",
                  status: "success",
                  payos_order_code: null,
                  created_at: "2026-10-21T11:00:00Z",
                  updated_at: "2026-10-21T11:00:00Z",
                },
              ],
              total: 2,
              page: 1,
              page_size: 20,
            },
          },
        });
      }
      if (url === "/admin/revenue") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              stats: {
                total_revenue: 100,
                successful_count: 2,
                pending_count: 0,
                failed_count: 0,
                cancelled_count: 0,
                total_count: 2,
              },
              summary: [
                {
                  period: "2026-10-21T00:00:00Z",
                  revenue: 100,
                  successful_deposits: 1,
                },
              ],
            },
          },
        });
      }
      return Promise.reject(new Error(`Unhandled URL: ${url}`));
    });
  });

  it("renders transactions and metrics correctly", async () => {
    renderComponent();

    // Check heading
    expect(
      screen.getByRole("heading", { name: "Payment Transactions" }),
    ).toBeInTheDocument();

    // Wait for revenue stats to load
    await waitFor(() => {
      expect(screen.getByText("Total Revenue")).toBeInTheDocument();
      expect(screen.getByText("100 ₫")).toBeInTheDocument();
    });

    // Check that transactions are listed
    await waitFor(() => {
      expect(screen.getByText(/Starter/i)).toBeInTheDocument();
      expect(
        screen.getByText(/Practice interview charge/i),
      ).toBeInTheDocument();
    });

    // Verify From / To are resolved nicely and not null or 000-0-0000000...
    expect(screen.getByText("Banking (PayOS)")).toBeInTheDocument();
    expect(screen.getByText("System")).toBeInTheDocument();

    // Check order code
    expect(screen.getByText("Order #123456789")).toBeInTheDocument();
  });

  it("filters transactions via toolbar search", async () => {
    renderComponent();

    const searchInput = await screen.findByPlaceholderText(
      "Search ID or description…",
    );
    fireEvent.change(searchInput, { target: { value: "Starter" } });

    await waitFor(() => {
      expect(mocks.get).toHaveBeenCalledWith(
        "/admin/transactions",
        expect.objectContaining({
          params: expect.objectContaining({
            search: "Starter",
          }),
        }),
      );
    });
  });

  it("opens details sheet on transaction row click", async () => {
    renderComponent();

    const row = await screen.findByText(/Starter/i);
    fireEvent.click(row);

    // Expect sheet title to appear
    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Transaction details" }),
      ).toBeInTheDocument();
    });

    expect(
      screen.getAllByText("candidate-user-1").length,
    ).toBeGreaterThanOrEqual(1);
  });

  it("handles empty state gracefully", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/transactions") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              items: [],
              total: 0,
              page: 1,
              page_size: 20,
            },
          },
        });
      }
      if (url === "/admin/revenue") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              stats: {
                total_revenue: 0,
                successful_count: 0,
                pending_count: 0,
                failed_count: 0,
                cancelled_count: 0,
                total_count: 0,
              },
              summary: [],
            },
          },
        });
      }
      return Promise.reject(new Error("Unhandled"));
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText("No transactions recorded")).toBeInTheDocument();
    });
  });

  it("handles error state with retry button", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/transactions") {
        return Promise.reject(new Error("Database connection failure"));
      }
      return Promise.resolve({
        data: {
          success: true,
          data: {
            stats: {
              total_revenue: 0,
              successful_count: 0,
              pending_count: 0,
              failed_count: 0,
              cancelled_count: 0,
              total_count: 0,
            },
            summary: [],
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
});
