import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
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
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.searchParams,
}));

function renderOverview() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <BillingOverview />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

function mockLedger(items: unknown[], balance = 1250) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/wallet")
      return Promise.resolve({
        data: {
          success: true,
          data: { id: "w-1", user_id: mocks.user.id, balance },
        },
      });
    if (url === "/wallet/transactions")
      return Promise.resolve({
        data: {
          success: true,
          data: { items, total: items.length, page: 1, page_size: 20 },
        },
      });
    return Promise.reject(new Error("unknown url"));
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.user.role = "candidate";
  mocks.searchParams = new URLSearchParams();
});

describe("BillingOverview in Vietnamese", () => {
  it("translates the page, balance, filters, statuses and ledger labels", async () => {
    mockLedger([
      {
        id: "tx-1",
        from: null,
        to: mocks.user.id,
        amount: 1500,
        currency: "COIN",
        description: "Deposit: Standard (100000 VND)",
        status: "success",
        payos_order_code: 100200,
        created_at: "2026-10-15T10:00:00Z",
      },
      {
        id: "tx-2",
        from: mocks.user.id,
        to: null,
        amount: 25,
        currency: "COIN",
        description: "Practice interview 5c1f2e8a-0000-4000-8000-000000000001",
        status: "pending",
        payos_order_code: null,
        created_at: "2026-10-16T14:30:00Z",
      },
      {
        id: "tx-3",
        from: mocks.user.id,
        to: null,
        amount: 40,
        currency: "COIN",
        description: "Manual adjustment by support",
        status: "failed",
        payos_order_code: null,
        created_at: "2026-10-17T09:00:00Z",
      },
    ]);
    renderOverview();

    expect(
      screen.getByRole("heading", { level: 1, name: "Lịch sử giao dịch" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Xu bạn đã mua, đã dùng và được hoàn lại."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thêm xu" })).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole("group", { name: "Số dư xu" })).toHaveTextContent(
        "1.250 xu",
      ),
    );

    expect(
      screen.getByRole("table", { name: "Giao dịch xu" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Tất cả giao dịch" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "Nạp xu (PayOS)" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "Hoàn xu" })).toBeVisible();

    // Ledger wording is mapped to labels; VND keeps its Vietnamese format.
    expect(await screen.findByText("Gói Standard")).toBeInTheDocument();
    expect(screen.getByText(/100\.000 ₫/)).toBeInTheDocument();
    expect(screen.getByText("Đơn hàng #100200")).toBeInTheDocument();
    expect(screen.getByText("Phỏng vấn luyện tập")).toBeInTheDocument();
    // Text the API wrote that we cannot map is shown as stored.
    expect(
      screen.getByText("Manual adjustment by support"),
    ).toBeInTheDocument();
    expect(screen.getByText("Thành công")).toBeInTheDocument();
    expect(screen.getByText("Đang chờ")).toBeInTheDocument();
    expect(screen.getByText("Thất bại")).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "+1.500 xu" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "-25 xu" })).toBeInTheDocument();
    expect(
      screen.getByText("Hiển thị 3 trong 3 giao dịch trên trang này."),
    ).toBeInTheDocument();
  });

  it("translates the empty ledger", async () => {
    mockLedger([]);
    renderOverview();
    expect(
      await screen.findByText("Không tìm thấy giao dịch nào"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Bạn chưa có giao dịch xu nào. Hãy nạp xu ở trên để bắt đầu.",
      ),
    ).toBeInTheDocument();
  });

  it("translates the administrator notice", () => {
    mocks.user.role = "admin";
    renderOverview();
    expect(
      screen.getByText("Tài khoản quản trị viên không có ví cá nhân"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Ví & xu" }),
    ).toBeInTheDocument();
  });

  it("translates the payment return banner and keeps the order code", async () => {
    mocks.searchParams = new URLSearchParams("cancel=true&orderCode=555444");
    mocks.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          id: "tx-sync",
          amount: 100,
          status: "cancelled",
          payos_order_code: 555444,
        },
      },
    });
    mockLedger([]);
    renderOverview();
    expect(await screen.findByText("Đã hủy thanh toán")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Thanh toán cho đơn hàng #555444 đã bị hủy. Không có xu nào được cộng.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đóng thông báo thanh toán" }),
    ).toBeInTheDocument();
  });
});
