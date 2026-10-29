import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
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

// Radix Select calls scrollIntoView when it opens; jsdom does not implement it.
if (!HTMLElement.prototype.scrollIntoView)
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: () => {},
  });

// 27/10/2026 15:00 in Vietnam.
const NOW = new Date("2026-10-27T08:00:00Z");

async function chooseOption(label: string, name: string) {
  fireEvent.keyDown(screen.getByRole("combobox", { name: label }), {
    key: "ArrowDown",
  });
  fireEvent.keyDown(await screen.findByRole("option", { name }), {
    key: "Enter",
  });
}

function lastParams(url: string) {
  const calls = mocks.get.mock.calls.filter(([called]) => called === url);
  return calls.at(-1)?.[1]?.params as Record<string, unknown> | undefined;
}

function renderComponent() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return renderWithIntl(
    <QueryClientProvider client={queryClient}>
      <AdminBillingView />
    </QueryClientProvider>,
  );
}

describe("AdminBillingView", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(NOW);
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
                  created_at: "2026-10-27T06:16:40Z",
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
                  created_at: "2026-10-26T18:05:00Z",
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
              created_at: "2026-10-27T06:16:40Z",
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
      expect(
        screen.getByRole("group", { name: "Revenue" }),
      ).toBeInTheDocument();
      expect(screen.getAllByText("50.000 ₫").length).toBeGreaterThanOrEqual(1);
    });

    await waitFor(() => {
      expect(screen.getByText("Starter Pack")).toBeInTheDocument();
      expect(screen.getByText("Practice interview")).toBeInTheDocument();
    });

    expect(screen.getByText(/Order #123456789/)).toBeInTheDocument();
    expect(screen.getByText("+100 coins")).toBeInTheDocument();
    expect(screen.getByText("-20 coins")).toBeInTheDocument();
  });

  it("filters transactions and summary with unified parameters", async () => {
    renderComponent();

    const searchInput = await screen.findByPlaceholderText(
      /Search name, email, order or ID/i,
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

  it("lists newest first by default and shows the created time in Vietnam time", async () => {
    renderComponent();

    expect(await screen.findByText("27/10/2026 13:16")).toBeInTheDocument();
    expect(screen.getByText("27/10/2026 01:05")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^date/i })).toBeInTheDocument();
    expect(lastParams("/admin/billing/transactions")).toMatchObject({
      sort_by: "created_at",
      sort_order: "desc",
    });
    expect(lastParams("/admin/billing/transactions")).not.toHaveProperty(
      "date_from",
    );
  });

  it("applies a preset window to the summary, the list and the export", async () => {
    vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:mock-url");
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    const baseImpl = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation((url: string, config?: unknown) =>
      url === "/admin/billing/export"
        ? Promise.resolve({ data: "Transaction ID\n", headers: {} })
        : baseImpl(url, config),
    );
    const window = {
      date_from: "2026-10-21T00:00:00+07:00",
      date_to: "2026-10-28T00:00:00+07:00",
    };

    renderComponent();
    await screen.findByText("27/10/2026 13:16");
    await chooseOption("Time window", "Last 7 days");

    await waitFor(() => {
      expect(lastParams("/admin/billing/transactions")).toMatchObject({
        ...window,
        page: 1,
      });
      expect(lastParams("/admin/billing/summary")).toMatchObject(window);
    });

    fireEvent.click(screen.getByRole("button", { name: /export csv/i }));
    await waitFor(() => {
      expect(lastParams("/admin/billing/export")).toMatchObject(window);
    });
    vi.restoreAllMocks();
  });

  it("sends an inclusive custom range and refuses a reversed one", async () => {
    renderComponent();
    await screen.findByText("27/10/2026 13:16");
    await chooseOption("Time window", "Custom range");

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-10-10" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-10-01" },
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The start date must be on or before the end date.",
    );
    expect(lastParams("/admin/billing/transactions")).not.toHaveProperty(
      "date_to",
    );

    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-10-31" },
    });

    await waitFor(() => {
      expect(lastParams("/admin/billing/transactions")).toMatchObject({
        date_from: "2026-10-10T00:00:00+07:00",
        date_to: "2026-11-01T00:00:00+07:00",
      });
    });
    expect(screen.getByText(/Vietnam time \(UTC\+07:00\)/)).toBeInTheDocument();
  });

  it("clears the window on Reset Filters", async () => {
    renderComponent();
    await screen.findByText("27/10/2026 13:16");
    await chooseOption("Time window", "Today");
    await waitFor(() => {
      expect(lastParams("/admin/billing/transactions")).toHaveProperty(
        "date_from",
      );
    });

    fireEvent.click(screen.getByRole("button", { name: /reset filters/i }));

    await waitFor(() => {
      expect(lastParams("/admin/billing/transactions")).not.toHaveProperty(
        "date_from",
      );
    });
    expect(
      screen.getByRole("combobox", { name: "Time window" }),
    ).toHaveTextContent("All time");
  });

  it("shows the created time in the details drawer", async () => {
    renderComponent();

    fireEvent.click(await screen.findByText("Starter Pack"));

    await screen.findByRole("heading", { name: "Transaction Details" });
    expect(await screen.findByText("Created at")).toBeInTheDocument();
    expect(
      screen.getAllByText("27/10/2026 13:16").length,
    ).toBeGreaterThanOrEqual(2);
  });

  it("sums up the matching rows in one strip without icons", async () => {
    renderComponent();

    const strip = await screen.findByRole("region", {
      name: "Matching transactions",
    });
    await waitFor(() =>
      expect(
        within(strip).getByRole("group", { name: "Revenue" }),
      ).toHaveTextContent("50.000 ₫"),
    );
    expect(
      within(strip).getByRole("group", { name: "Waiting for payment" }),
    ).toHaveTextContent("1");
    expect(strip.querySelector("svg")).toBeNull();
  });

  it("names each row by what happened and keeps raw IDs in the drawer", async () => {
    renderComponent();

    expect(await screen.findByText("Practice interview")).toBeInTheDocument();
    expect(
      screen.queryByText("Practice interview charge"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("columnheader", { name: /transaction \/ order/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: /amount/i }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getAllByText("Starter Pack")[0]);
    await screen.findByRole("heading", { name: "Transaction Details" });
    expect(
      screen.getByText("00000000-0000-0000-0000-000000000001"),
    ).toBeInTheDocument();
  });

  it("offers coin features as well as packs in the package filter", async () => {
    renderComponent();
    await screen.findByText("27/10/2026 13:16");

    fireEvent.keyDown(
      screen.getByRole("combobox", { name: "Filter by package or feature" }),
      { key: "ArrowDown" },
    );
    expect(
      await screen.findByRole("option", { name: "Practice interviews" }),
    ).toBeInTheDocument();
    fireEvent.keyDown(
      screen.getByRole("option", { name: "Practice interviews" }),
      { key: "Enter" },
    );
    await waitFor(() =>
      expect(lastParams("/admin/billing/transactions")).toMatchObject({
        package: "Practice interview",
      }),
    );
  });
});
