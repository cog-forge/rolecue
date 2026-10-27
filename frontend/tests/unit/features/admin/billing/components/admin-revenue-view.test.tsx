import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminRevenueView } from "@/features/admin/billing/components/admin-revenue-view";

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
      <AdminRevenueView />
    </QueryClientProvider>,
  );
}

describe("AdminRevenueView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.get.mockImplementation((url: string) => {
      if (url === "/admin/billing/summary") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              total_revenue: 150000,
              net_revenue: 150000,
              successful_deposits: 2,
              total_deposits: 3,
              success_count: 4,
              pending_count: 1,
              failed_count: 1,
              cancelled_count: 0,
              total_transactions: 6,
              average_deposit: 75000,
              unique_payers: 2,
              coins_sold: 300,
              coins_spent: 40,
              coins_refunded: 10,
              coins_net: 270,
              success_rate: 66.7,
              package_breakdown: [
                {
                  package_name: "Pro Practice Pack",
                  revenue_vnd: 100000,
                  deposit_count: 1,
                  coins_sold: 200,
                },
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
                  coins_spent: 40,
                  spend_count: 2,
                },
              ],
              schema_limitation:
                "public.transactions in the frozen schema (000006_wallet_payment) does not store a created_at timestamp.",
            },
          },
        });
      }
      return Promise.reject(new Error(`Unhandled: ${url}`));
    });
  });

  it("renders revenue reports and real breakdown data correctly", async () => {
    renderComponent();

    expect(
      screen.getByRole("heading", { name: "Revenue Reports" }),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Total Revenue (PayOS)")).toBeInTheDocument();
      expect(screen.getAllByText("150.000 ₫").length).toBeGreaterThanOrEqual(1);
    });

    expect(
      screen.getAllByText("Pro Practice Pack").length,
    ).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Starter Pack").length).toBeGreaterThanOrEqual(
      1,
    );
    expect(screen.getByText("Practice interview charge")).toBeInTheDocument();
  });

  it("scopes the success rate to PayOS deposit attempts, not all ledger rows", async () => {
    renderComponent();

    // 2 of 3 PayOS attempts paid, even though total_transactions (6) also counts coin rows.
    expect(
      await screen.findByText(/66\.7% of 3 attempts paid/),
    ).toBeInTheDocument();
  });

  it("shows honest empty states when breakdown data is empty without fabricating numbers", async () => {
    mocks.get.mockImplementation((url: string) => {
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
      return Promise.reject(new Error(`Unhandled: ${url}`));
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText("No package revenue data")).toBeInTheDocument();
      expect(screen.getByText("No coin spending data")).toBeInTheDocument();
    });

    // Must never fabricate "Live Coding Sandbox" or fake package splits
    expect(screen.queryByText(/Live Coding Sandbox/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Enterprise Boost/i)).not.toBeInTheDocument();
  });

  it("does not render misleading date range pickers on the frozen schema", async () => {
    renderComponent();

    await waitFor(() => {
      expect(screen.getByText("Total Revenue (PayOS)")).toBeInTheDocument();
    });

    expect(
      screen.queryByRole("button", { name: "Last 7 days" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Last 30 days" }),
    ).not.toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      "/admin/billing/summary",
      expect.objectContaining({
        params: {},
      }),
    );
  });

  it("handles error state with retry button", async () => {
    mocks.get.mockImplementation(() =>
      Promise.reject(new Error("Network Error")),
    );

    renderComponent();

    await waitFor(() => {
      expect(
        screen.getByText("Unable to load revenue report"),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /retry/i }),
      ).toBeInTheDocument();
    });
  });
});
