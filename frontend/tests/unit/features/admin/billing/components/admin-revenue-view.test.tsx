import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
      if (url === "/admin/revenue") {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              stats: {
                total_revenue: 5000,
                successful_count: 10,
                pending_count: 2,
                failed_count: 1,
                cancelled_count: 0,
                total_count: 13,
              },
              summary: [
                {
                  period: "2026-10-21T00:00:00Z",
                  revenue: 3000,
                  successful_deposits: 6,
                },
                {
                  period: "2026-10-20T00:00:00Z",
                  revenue: 2000,
                  successful_deposits: 4,
                },
              ],
            },
          },
        });
      }
      return Promise.reject(new Error(`Unhandled: ${url}`));
    });
  });

  it("renders revenue reports and metrics correctly", async () => {
    renderComponent();

    // Check heading
    expect(
      screen.getByRole("heading", { name: "Revenue Reports" }),
    ).toBeInTheDocument();

    // Check metrics
    await waitFor(() => {
      expect(screen.getByText("Total Revenue")).toBeInTheDocument();
      expect(screen.getByText(/5[.,]000/)).toBeInTheDocument();
      expect(screen.getByText("Successful")).toBeInTheDocument();
      expect(screen.getByText("10")).toBeInTheDocument();
    });

    // Check chart title
    expect(screen.getByText("Daily Revenue (VND)")).toBeInTheDocument();

    // Check breakdown table
    expect(screen.getByText("Daily Breakdown")).toBeInTheDocument();
    expect(screen.getByText(/\+3[.,]000/)).toBeInTheDocument();
    expect(screen.getByText(/\+2[.,]000/)).toBeInTheDocument();
  });

  it("updates dates when a preset is clicked", async () => {
    renderComponent();

    const sevenDaysBtn = await screen.findByRole("button", {
      name: "Last 7 days",
    });
    fireEvent.click(sevenDaysBtn);

    await waitFor(() => {
      expect(mocks.get).toHaveBeenCalledWith(
        "/admin/revenue",
        expect.objectContaining({
          params: expect.objectContaining({
            date_from: expect.any(String),
            date_to: expect.any(String),
          }),
        }),
      );
    });
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
