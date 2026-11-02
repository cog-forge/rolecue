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

vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));

vi.mock("goey-toast", () => ({
  gooeyToast: { warning: mocks.toastWarning, error: mocks.toastError },
}));

// Radix Select calls scrollIntoView when it opens; jsdom does not implement it.
if (!HTMLElement.prototype.scrollIntoView)
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: () => {},
  });

const NOW = new Date("2026-10-27T08:00:00Z");

const TOPUP = {
  id: "00000000-0000-0000-0000-000000000001",
  from: "",
  to: "candidate-user-1",
  user_id: "candidate-user-1",
  user_name: "Alice Nguyen",
  user_email: "alice@example.com",
  type: "topup",
  package_or_feature: "Starter Pack",
  paid_vnd: 50000,
  coin_amount: 1500,
  amount: 1500,
  currency: "VND",
  description: "Deposit: Starter Pack (50000 VND)",
  status: "success",
  payos_order_code: 123456789,
  created_at: "2026-10-27T06:16:40Z",
};
const WAITING = {
  ...TOPUP,
  id: "00000000-0000-0000-0000-000000000003",
  package_or_feature: "Pro",
  paid_vnd: 100000,
  description: "Deposit: Pro (100000 VND)",
  status: "pending",
  payos_order_code: 987,
};
const SPEND = {
  ...TOPUP,
  id: "00000000-0000-0000-0000-000000000002",
  type: "spend",
  package_or_feature: "Practice interview charge",
  paid_vnd: 0,
  coin_amount: -20,
  description: "Practice interview charge",
  payos_order_code: null,
};

function respond(items: unknown[], total = items.length) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/admin/billing/summary") {
      return Promise.resolve({
        data: {
          success: true,
          data: {
            total_revenue: 50000,
            successful_deposits: 1500,
            pending_count: 1,
            failed_count: 2,
            cancelled_count: 3,
            coins_spent: 2500,
            coins_refunded: 40,
            package_breakdown: [{ package_name: "Starter Pack" }],
          },
        },
      });
    }
    if (url === "/admin/billing/transactions") {
      return Promise.resolve({
        data: {
          success: true,
          data: { items, total, page: 1, page_size: 20 },
        },
      });
    }
    if (url.startsWith("/admin/billing/transactions/")) {
      return Promise.resolve({ data: { success: true, data: TOPUP } });
    }
    return Promise.reject(new Error(`Unhandled URL: ${url}`));
  });
}

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={queryClient}>
      <AdminBillingView />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

describe("AdminBillingView in Vietnamese", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(NOW);
    vi.clearAllMocks();
    respond([TOPUP, WAITING, SPEND], 1234);
  });

  it("translates the screen chrome, filters and summary", async () => {
    renderView();

    expect(
      screen.getByRole("heading", { name: "Giao dịch thanh toán" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "Nạp xu qua PayOS" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đặt lại bộ lọc" }),
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("Tìm tên, email, đơn hàng hoặc mã"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "Khoảng thời gian" }),
    ).toHaveTextContent("Mọi lúc");
    expect(
      screen.getByRole("combobox", { name: "Lọc theo trạng thái" }),
    ).toHaveTextContent("Mọi trạng thái");
    expect(
      screen.getByRole("spinbutton", { name: "Số tiền tối thiểu" }),
    ).toBeInTheDocument();

    const summary = await screen.findByRole("region", {
      name: "Giao dịch phù hợp",
    });
    const revenue = await within(summary).findByRole("group", {
      name: "Doanh thu",
    });
    expect(revenue).toHaveTextContent("50.000 ₫");
    expect(revenue).toHaveTextContent("1.500 lượt nạp xu đã thanh toán");
    expect(
      within(summary).getByRole("group", { name: "Thất bại hoặc đã hủy" }),
    ).toHaveTextContent("2 thất bại · 3 đã hủy");
    expect(
      within(summary).getByRole("group", { name: "Xu đã chi tiêu" }),
    ).toHaveTextContent("2.500");
  });

  it("shows row statuses, types, coins and the order in Vietnamese", async () => {
    renderView();

    const table = await screen.findByRole("table");
    await within(table).findByText("Starter Pack");
    expect(
      within(table).getByRole("button", { name: /Ngày/ }),
    ).toBeInTheDocument();
    // Dates stay dd/MM/yyyy HH:mm in Vietnam time.
    expect(within(table).getAllByText("27/10/2026 13:16").length).toBe(3);
    expect(within(table).getByText("Gói Pro")).toBeInTheDocument();
    expect(within(table).getByText("Đã thanh toán")).toBeInTheDocument();
    expect(within(table).getByText("Đang chờ")).toBeInTheDocument();
    expect(within(table).getByText("Đã ghi nhận")).toBeInTheDocument();
    expect(within(table).getByText("+1.500 xu")).toBeInTheDocument();
    expect(within(table).getByText("-20 xu")).toBeInTheDocument();
    expect(within(table).getByText("Chưa cộng xu")).toBeInTheDocument();
    // The ledger wording is mapped to the same labels the candidate sees.
    expect(within(table).getByText("Phỏng vấn luyện tập")).toBeInTheDocument();
    expect(
      within(table).getAllByText(/Nạp xu qua PayOS · .*Đơn hàng #123456789/)
        .length,
    ).toBe(1);
    expect(
      within(table).getByRole("button", {
        name: "Xem chi tiết Starter Pack lúc 27/10/2026 13:16",
      }),
    ).toBeInTheDocument();
  });

  it("counts transactions with a Vietnamese pager", async () => {
    renderView();

    expect(
      await screen.findByText("1–3 trong 1.234 giao dịch"),
    ).toBeInTheDocument();
    expect(screen.getByText("Trang 1/62")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Sau$/ })).toBeEnabled();
    expect(
      screen.getByRole("combobox", { name: "Số dòng mỗi trang" }),
    ).toBeInTheDocument();
  });

  it("says when no transaction matches", async () => {
    respond([], 0);
    renderView();

    expect(
      await screen.findByText(
        "Không có giao dịch nào khớp với bộ lọc hiện tại.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("0 trong 0 giao dịch")).toBeInTheDocument();
  });

  it("opens the details drawer with Vietnamese labels and keeps the ledger text as stored", async () => {
    renderView();

    fireEvent.click(await screen.findByText("Starter Pack"));

    expect(
      await screen.findByRole("heading", { name: "Chi tiết giao dịch" }),
    ).toBeInTheDocument();
    const drawer = await screen.findByRole("dialog");
    expect(within(drawer).getByText("Tạo lúc")).toBeInTheDocument();
    expect(within(drawer).getByText("Đơn hàng PayOS")).toBeInTheDocument();
    expect(
      within(drawer).getByRole("button", { name: "Sao chép mã giao dịch" }),
    ).toBeInTheDocument();
    expect(within(drawer).getByText("+1.500 xu")).toBeInTheDocument();
    expect(
      within(drawer).getByText("Deposit: Starter Pack (50000 VND)"),
    ).toBeInTheDocument();
  });

  it("filters by status with the Vietnamese labels and still sends API values", async () => {
    renderView();
    await screen.findByText("Starter Pack");

    fireEvent.keyDown(
      screen.getByRole("combobox", { name: "Lọc theo trạng thái" }),
      { key: "ArrowDown" },
    );
    fireEvent.keyDown(await screen.findByRole("option", { name: "Thất bại" }), {
      key: "Enter",
    });

    await waitFor(() =>
      expect(mocks.get).toHaveBeenCalledWith(
        "/admin/billing/transactions",
        expect.objectContaining({
          params: expect.objectContaining({ status: "failed" }),
        }),
      ),
    );
  });

  it("warns in Vietnamese when the export is truncated", async () => {
    const createObjectURL = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:mock-url");
    const revokeObjectURL = vi
      .spyOn(URL, "revokeObjectURL")
      .mockImplementation(() => {});
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});
    const baseImpl = mocks.get.getMockImplementation()!;
    mocks.get.mockImplementation((url: string) =>
      url === "/admin/billing/export"
        ? Promise.resolve({
            data: "a,b",
            headers: { "x-export-truncated": "true", "x-total-count": "12345" },
          })
        : baseImpl(url),
    );
    renderView();

    fireEvent.click(await screen.findByRole("button", { name: "Xuất CSV" }));

    await waitFor(() =>
      expect(mocks.toastWarning).toHaveBeenCalledWith(
        "Đã rút gọn tệp xuất còn 5.000 dòng",
        expect.objectContaining({
          description: expect.stringContaining("5.000 trong tổng số 12.345"),
        }),
      ),
    );

    createObjectURL.mockRestore();
    revokeObjectURL.mockRestore();
    click.mockRestore();
  });
});
