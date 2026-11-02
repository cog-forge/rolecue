import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { renderWithIntl } from "@tests/helpers/intl";
import { RoleProfileLibrary } from "@/features/role-profile/components/role-profile-library";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cand",
    email: "cand@example.com",
    role: "candidate" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));

vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, delete: vi.fn() },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const item = (over: Record<string, unknown> = {}) => ({
  id: "22222222-2222-4222-8222-222222222222",
  title: "Senior Backend Engineer (Go)",
  seniorityLevel: "senior",
  status: "review",
  createdAt: "2026-10-10T00:00:00Z",
  ...over,
});
const list = (items: unknown[]) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset: 0, total: items.length } },
  },
});

function renderLibrary(locale: "en" | "vi") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <RoleProfileLibrary />
    </QueryClientProvider>,
    { locale },
  );
}

describe("RoleProfileLibrary in Vietnamese", () => {
  beforeEach(() => {
    mocks.get.mockReset();
  });

  it("translates the chrome and maps enums, but keeps the title as stored", async () => {
    mocks.get.mockResolvedValue(
      list([
        item(),
        item({
          id: "33333333-3333-4333-8333-333333333333",
          title: "Mystery",
          seniorityLevel: null,
        }),
      ]),
    );
    renderLibrary("vi");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Hồ sơ vị trí" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Thêm hồ sơ vị trí" }),
    ).toBeInTheDocument();
    const table = await screen.findByRole("table", { name: "Hồ sơ vị trí" });
    for (const header of ["Vị trí", "Trạng thái", "Ngày tạo", "Thao tác"])
      expect(
        within(table).getByRole("columnheader", { name: header }),
      ).toBeInTheDocument();
    expect(within(table).getAllByText("Cần xem lại").length).toBeGreaterThan(0);
    expect(within(table).getByText("Senior")).toBeInTheDocument();
    expect(within(table).getByText("Cần xem lại cấp bậc")).toBeInTheDocument();
    expect(within(table).getAllByText("10 thg 10, 2026")).toHaveLength(2);
    // User-entered data is rendered exactly as stored.
    expect(
      screen.getByText("Senior Backend Engineer (Go)"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xóa Senior Backend Engineer (Go)" }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Xóa Senior Backend Engineer (Go)" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText("Xóa hồ sơ vị trí này?"),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        /^Senior Backend Engineer \(Go\) và các yêu cầu/,
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Giữ hồ sơ" }),
    ).toBeInTheDocument();
  });

  it("shows the translated empty state", async () => {
    mocks.get.mockResolvedValue(list([]));
    renderLibrary("vi");
    expect(
      await screen.findByRole("heading", {
        name: "Vị trí tiếp theo của bạn bắt đầu từ đây.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Thêm một vị trí" }),
    ).toBeInTheDocument();
  });

  it("keeps the English wording in English", async () => {
    mocks.get.mockResolvedValue(list([]));
    renderLibrary("en");
    expect(
      await screen.findByRole("heading", {
        name: "Your next role starts here.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Bring in a role" }),
    ).toBeInTheDocument();
  });

  it("shows a translated message for a coded failure, never the server text", async () => {
    const config = { headers: new AxiosHeaders() };
    const failure = new AxiosError(
      "boom",
      "ERR_BAD_RESPONSE",
      undefined,
      undefined,
      {
        status: 502,
        statusText: "Bad Gateway",
        headers: new AxiosHeaders(),
        config,
        data: {
          success: false,
          error: { code: "EXTRACTION_FAILED", message: "Server English text" },
        },
      },
    );
    mocks.get.mockRejectedValue(failure);
    renderLibrary("vi");
    expect(
      await screen.findByText(
        "Phân tích không thành công. Hãy thử lại với bản nháp đã lưu.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Server English text")).not.toBeInTheDocument();
  });
});
