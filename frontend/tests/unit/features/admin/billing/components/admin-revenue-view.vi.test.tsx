import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminRevenueView } from "@/features/admin/billing/components/admin-revenue-view";
import { octoberReport } from "../fixtures/revenue-report";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));

vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));

// Radix Select calls scrollIntoView when it opens; jsdom does not implement it.
if (!HTMLElement.prototype.scrollIntoView)
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: () => {},
  });

// 27/10/2026 15:00 in Vietnam.
const NOW = new Date("2026-10-27T08:00:00Z");

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={queryClient}>
      <AdminRevenueView />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

describe("AdminRevenueView in Vietnamese", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(NOW);
    vi.clearAllMocks();
    mocks.get.mockResolvedValue({
      data: { success: true, data: octoberReport },
    });
  });

  it("states the finding with Vietnamese months and keeps VND conventions", async () => {
    renderView();

    expect(
      screen.getByRole("heading", { level: 1, name: "Doanh thu" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        "1–27 thg 10, 2026 · 200.000 ₫, tăng 33% so với 1–27 thg 10, 2025 (+50.000 ₫)",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "Khoảng thời gian" }),
    ).toHaveTextContent("Tháng này");
    expect(
      screen.getByRole("combobox", { name: "So sánh với" }),
    ).toHaveTextContent("Cùng kỳ năm trước");
  });

  it("translates the headline figures, deltas and notes", async () => {
    renderView();

    const kpis = await screen.findByRole("region", { name: "Chỉ số chính" });
    const revenue = within(kpis).getByRole("group", { name: "Doanh thu" });
    expect(revenue).toHaveTextContent("200.000 ₫");
    expect(revenue).toHaveTextContent("so với 1–27 thg 10, 2025");
    expect(revenue).toHaveTextContent("3 lượt nạp xu đã thanh toán");

    const payers = within(kpis).getByRole("group", {
      name: "Người dùng đã thanh toán",
    });
    expect(payers).toHaveTextContent("Không đổi");
    expect(payers).toHaveTextContent("0 mới · 2 quay lại");

    const success = within(kpis).getByRole("group", {
      name: "Tỷ lệ thanh toán thành công",
    });
    expect(success).toHaveTextContent("−16,7 điểm %");
    expect(success).toHaveTextContent("3 trên 6 lượt thanh toán PayOS");
  });

  it("names the chart, the mix and the coins in Vietnamese with local grouping", async () => {
    renderView();

    expect(
      await screen.findByRole("region", { name: "Doanh thu theo thời gian" }),
    ).toHaveTextContent("Doanh thu theo ngày");
    expect(
      screen.getByLabelText(
        "Doanh thu theo ngày, 1–27 thg 10, 2026 so với 1–27 thg 10, 2025",
      ),
    ).toBeInTheDocument();

    const mix = screen.getByRole("region", { name: "Doanh thu đến từ đâu" });
    expect(within(mix).getByText("Nhà tuyển dụng")).toBeInTheDocument();
    expect(within(mix).getByText("Ứng viên")).toBeInTheDocument();
    expect(within(mix).getByText("Standard")).toBeInTheDocument();

    const coins = screen.getByRole("region", { name: "Xu" });
    expect(within(coins).getByText("Lượt phỏng vấn")).toBeInTheDocument();
    expect(within(coins).getByText("Phỏng vấn luyện tập")).toBeInTheDocument();
    expect(
      within(coins).getByText("Dung lượng lưu trữ avatar"),
    ).toBeInTheDocument();
    expect(within(coins).getByText("30 xu kỳ trước")).toBeInTheDocument();

    const owed = screen.getByRole("region", { name: "Xu người dùng còn giữ" });
    expect(owed).toHaveTextContent("1.258 xu");
    expect(owed).toHaveTextContent("≈ 1.258.000 ₫ theo giá trung bình");
  });

  it("offers the Vietnamese period and comparison options", async () => {
    renderView();
    await screen.findByRole("region", { name: "Chỉ số chính" });

    fireEvent.keyDown(screen.getByRole("combobox", { name: "So sánh với" }), {
      key: "ArrowDown",
    });
    expect(
      await screen.findByRole("option", { name: "Kỳ trước" }),
    ).toBeInTheDocument();
  });

  it("shows the error card in Vietnamese", async () => {
    mocks.get.mockRejectedValue(new Error("Network Error"));
    renderView();

    expect(
      await screen.findByText("Không thể tải báo cáo doanh thu."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
  });
});
