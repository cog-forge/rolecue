import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AccountSecurity,
  PasswordSettings,
} from "@/features/account-security/components/account-security";

const { listAccounts } = vi.hoisted(() => ({ listAccounts: vi.fn() }));
vi.mock("@/lib/auth/client", () => ({
  authClient: { listAccounts, changePassword: vi.fn() },
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: vi.fn() } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: userId, email: "alex@example.test" }),
}));

const userId = "11111111-1111-4111-8111-111111111111";
const accounts = (...providerIds: string[]) => ({
  data: providerIds.map((providerId, index) => ({
    id: String(index),
    userId,
    providerId,
  })),
  error: null,
});

function renderPage() {
  renderWithIntl(
    <QueryClientProvider client={new QueryClient()}>
      <AccountSecurity>
        <PasswordSettings />
      </AccountSecurity>
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

describe("AccountSecurity in Vietnamese", () => {
  beforeEach(() => vi.clearAllMocks());

  it("translates the header and keeps the email as stored", async () => {
    listAccounts.mockResolvedValueOnce(accounts("credential"));
    renderPage();
    expect(
      screen.getByRole("heading", { level: 1, name: "Bảo mật tài khoản" }),
    ).toBeInTheDocument();
    expect(screen.getByText("alex@example.test")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Chỉnh sửa hồ sơ" }),
    ).toHaveAttribute("href", "/profile");
    expect(await screen.findByText("Đã đặt")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Mật khẩu" })).toBeVisible();
  });

  it("opens a translated change-password dialog", async () => {
    listAccounts.mockResolvedValueOnce(accounts("credential"));
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Đổi mật khẩu" }),
    );
    const dialog = await screen.findByRole("dialog", { name: "Đổi mật khẩu" });
    expect(dialog).toBeInTheDocument();
    expect(screen.getByLabelText("Mật khẩu hiện tại")).toHaveValue("");
    expect(screen.getByLabelText("Mật khẩu mới")).toBeInTheDocument();
    expect(
      screen.getByRole("list", { name: "Yêu cầu mật khẩu" }),
    ).toHaveTextContent("Ít nhất 8 ký tự");
    expect(
      screen.getByRole("button", { name: "Hiện mật khẩu hiện tại" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Cập nhật mật khẩu" }),
    ).toBeInTheDocument();
  });

  it("explains a social-only account in Vietnamese", async () => {
    listAccounts.mockResolvedValueOnce(accounts("google", "github"));
    renderPage();
    expect(
      await screen.findByText(/Bạn đăng nhập bằng Google hoặc GitHub/),
    ).toHaveTextContent(
      "Bạn đăng nhập bằng Google hoặc GitHub, nên tài khoản này không có mật khẩu để đổi.",
    );
    expect(screen.getByRole("link", { name: "Quên mật khẩu" })).toHaveAttribute(
      "href",
      "/forgot-password",
    );
  });
});
