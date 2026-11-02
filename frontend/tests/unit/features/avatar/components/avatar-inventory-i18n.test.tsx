import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { AvatarInventory } from "@/features/avatar/components/avatar-inventory";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
  push: vi.fn(),
  params: new URLSearchParams(),
  toast: vi.fn(),
  user: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", role: "candidate" },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post, delete: mocks.delete },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
  usePathname: () => "/avatar-studio",
  useSearchParams: () => mocks.params,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));
vi.mock("@/components/avatar-viewer/avatar-viewer", () => ({
  AvatarViewer: ({ label }: { label: string }) => <div>{label}</div>,
}));
vi.mock("@/components/avatar-viewer/webgl", () => ({
  isWebGLAvailable: () => false,
}));
vi.mock("@/components/avatar-viewer/render-portrait", () => ({
  renderAvatarPortrait: vi.fn(),
}));

const avatar = (n: number) => ({
  id: `1111111${n}-1111-4111-8111-111111111111`,
  name: `Avatar ${n}`,
  personality: `Personality ${n}`,
  created_at: "2026-10-27T10:00:00Z",
  updated_at: "2026-10-27T10:00:00Z",
});
let summary = { capacity: 5, used: 2, available: 3, is_full: false };
let items = [avatar(1), avatar(2)];
let balance = 5;

function apiError(status: number, code: string) {
  return new AxiosError(
    "Request failed",
    "ERR_BAD_RESPONSE",
    undefined,
    undefined,
    {
      status,
      statusText: "",
      data: {
        success: false,
        error: { code, message: "English server text" },
      },
      headers: {},
      config: { headers: new AxiosHeaders() },
    },
  );
}

function renderInventory() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <AvatarInventory />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.params = new URLSearchParams();
  mocks.user = {
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    role: "candidate",
  };
  summary = { capacity: 5, used: 2, available: 3, is_full: false };
  items = [avatar(1), avatar(2)];
  balance = 5;
  mocks.get.mockImplementation(async (url: string) => {
    if (url === "/inventory") return { data: { success: true, data: summary } };
    if (url === "/inventory/avatars")
      return {
        data: {
          success: true,
          data: {
            items,
            pagination: { limit: 12, offset: 0, total: items.length },
          },
        },
      };
    if (url === "/wallet")
      return {
        data: {
          success: true,
          data: { id: "w", user_id: mocks.user.id, balance },
        },
      };
    if (url === "/wallet/packages")
      return { data: { success: true, data: [] } };
    throw new Error(`unexpected GET ${url}`);
  });
});

describe("avatar inventory in Vietnamese", () => {
  it("translates the page, the capacity panel and the slot cards, and keeps avatar data as is", async () => {
    renderInventory();
    expect(
      await screen.findByRole("list", { name: "Ô avatar, đã dùng 2/5" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "Avatar Studio cá nhân" }),
    ).toBeInTheDocument();
    // The digits animate in separate spans, so match the paragraph by its full text.
    expect(
      screen.getByText(
        (_, element) =>
          element?.tagName === "P" && element.textContent === "Đã dùng 2/5",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Còn 3 ô trống cho avatar mới."),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Tạo avatar" })).toHaveAttribute(
      "href",
      "/avatar-studio/new",
    );
    expect(
      screen.getByRole("link", { name: "Tạo avatar ở ô trống 3/5" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Ô trống")).toHaveLength(2);
    expect(screen.getByText("Ô 4/5")).toBeInTheDocument();
    expect(screen.getAllByText("Thiết bị này không hỗ trợ 3D")).toHaveLength(2);
    expect(
      screen.getByRole("button", { name: "Mở Avatar 1 ở chế độ 3D" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xóa Avatar 2" }),
    ).toBeInTheDocument();
    // Avatar names and personalities are the user's own text.
    expect(screen.getByText("Avatar 1")).toBeInTheDocument();
    expect(screen.getByText("Personality 2")).toBeInTheDocument();
    expect(screen.getAllByText("27 thg 10, 2026")).toHaveLength(2);
    expect(screen.queryByText(/free slot/i)).not.toBeInTheDocument();
  });

  it("shows the full state with the add-slots card priced in xu", async () => {
    summary = { capacity: 2, used: 2, available: 0, is_full: true };
    renderInventory();
    expect(await screen.findByText("Đã đầy")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Hãy xóa avatar bạn không cần nữa hoặc mua thêm ô để tạo avatar mới.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("1 xu mỗi ô")).toBeInTheDocument();
    expect(screen.getByText("Thêm ô avatar")).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Mua ô avatar" }).length,
    ).toBeGreaterThan(0);
  });

  it("explains an empty inventory by inviting the first avatar", async () => {
    items = [];
    summary = { capacity: 3, used: 0, available: 3, is_full: false };
    renderInventory();
    expect(
      await screen.findByRole("link", { name: "Tạo avatar ở ô trống 1/3" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Tạo avatar", { selector: "p" })).toBeVisible();
    expect(
      screen.getByText("Còn 3 ô trống cho avatar mới."),
    ).toBeInTheDocument();
  });

  it("maps a delete failure by code, never showing the server's text", async () => {
    mocks.delete.mockRejectedValue(apiError(409, "AVATAR_IN_USE"));
    renderInventory();
    fireEvent.click(
      await screen.findByRole("button", { name: "Xóa Avatar 1" }),
    );
    const dialog = await screen.findByRole("dialog", { name: "Xóa Avatar 1?" });
    expect(
      within(dialog).getByText(
        "Thao tác này giải phóng một ô trong kho avatar. Sức chứa của bạn không đổi. Không thể hoàn tác.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Xóa avatar" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Avatar này đang được dùng trong một tin tuyển dụng hoặc buổi phỏng vấn nên không thể xóa.",
    );
    expect(screen.queryByText("English server text")).not.toBeInTheDocument();
  });

  it("reports a deletion in Vietnamese", async () => {
    mocks.delete.mockResolvedValue({ status: 204, data: "" });
    renderInventory();
    fireEvent.click(
      await screen.findByRole("button", { name: "Xóa Avatar 2" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Xóa avatar" }));
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith("Đã xóa avatar", {
        description: "Kho avatar của bạn đã trống thêm một ô.",
      }),
    );
  });
});

describe("buy slots in Vietnamese", () => {
  async function openDialog() {
    renderInventory();
    await screen.findByRole("list", { name: /^Ô avatar/ });
    fireEvent.click(
      screen.getAllByRole("button", { name: "Mua ô avatar" })[0]!,
    );
    return screen.findByRole("dialog", { name: "Mua ô avatar" });
  }

  it("prices slots in xu and totals the capacity after the purchase", async () => {
    const dialog = await openDialog();
    expect(
      within(dialog).getByText(
        "Mỗi ô avatar thêm chỗ chứa cho kho avatar của bạn. Mỗi ô có giá 1 xu và được trừ từ ví của bạn.",
      ),
    ).toBeInTheDocument();
    expect(await within(dialog).findByText("5 xu")).toBeInTheDocument();
    expect(
      within(dialog).getByText("Mỗi lần mua từ 1 đến 10 ô."),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Thêm một ô" }));
    expect(within(dialog).getByText("2 xu")).toBeInTheDocument();
    expect(within(dialog).getByText("7 ô")).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Mua với giá 2 xu" }),
    ).toBeEnabled();
  });

  it("blocks a purchase the balance can't cover and offers to add coins", async () => {
    balance = 0;
    const dialog = await openDialog();
    expect(
      await within(dialog).findByText(/Bạn cần thêm 1 xu\./),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Nạp thêm xu" }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Mua với giá 1 xu" }),
    ).toBeDisabled();
  });

  it("validates the amount in Vietnamese", async () => {
    const dialog = await openDialog();
    const input = within(dialog).getByLabelText("Số ô");
    fireEvent.change(input, { target: { value: "11" } });
    fireEvent.click(
      within(dialog).getByRole("button", { name: /^Mua với giá/ }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Mỗi lần chỉ được mua tối đa 10 ô.",
    );
  });

  it("maps an insufficient-balance reply by code", async () => {
    mocks.post.mockRejectedValue(apiError(402, "INSUFFICIENT_BALANCE"));
    const dialog = await openDialog();
    fireEvent.click(
      await within(dialog).findByRole("button", { name: /^Mua với giá/ }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Bạn không đủ xu cho giao dịch này. Hãy nạp thêm vào ví rồi thử lại.",
    );
    expect(screen.queryByText("English server text")).not.toBeInTheDocument();
  });
});
