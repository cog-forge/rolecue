import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BackupCodes } from "@/features/auth/components/backup-codes";
import { SecuritySummary } from "@/features/auth/components/security-summary";
import { SecuritySessions } from "@/features/auth/components/security-sessions";
import { SignInMethods } from "@/features/auth/components/sign-in-methods";
import { TotpManagement } from "@/features/auth/components/totp-management";
import type { SecuritySession } from "@/features/auth/api/credentials";
import type { UseQueryResult } from "@tanstack/react-query";

vi.mock("goey-toast", () => ({ gooeyToast: { success: vi.fn() } }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), refresh: vi.fn() }),
}));
vi.mock("@/lib/auth/client", () => ({ authClient: {} }));

const session = (id: string): SecuritySession => ({
  id,
  token: `private-${id}`,
  userId: "user-1",
  createdAt: new Date("2026-10-10T10:00:00Z"),
  updatedAt: new Date("2026-10-12T10:00:00Z"),
  expiresAt: new Date("2026-11-10T10:00:00Z"),
  ipAddress: "203.0.113.7",
  userAgent: "Mozilla/5.0 (Macintosh) Safari/605.1",
});

function sessionsResult(data: SecuritySession[]) {
  return {
    isPending: false,
    error: null,
    data,
    refetch: vi.fn(),
  } as unknown as UseQueryResult<SecuritySession[]>;
}

function wrap(ui: React.ReactElement) {
  return renderWithIntl(
    <QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>,
    { locale: "vi" },
  );
}

describe("account security components in Vietnamese", () => {
  beforeEach(() => vi.clearAllMocks());

  it("translates the security summary", () => {
    renderWithIntl(
      <SecuritySummary
        emailVerified
        hasPassword
        twoFactorEnabled={false}
        sessionCount={3}
      />,
      { locale: "vi" },
    );
    const summary = screen.getByRole("navigation", { name: "Tóm tắt bảo mật" });
    expect(summary).toHaveTextContent("Xác thực hai yếu tốTắtNên bật");
    expect(summary).toHaveTextContent(
      "Đang đăng nhập3 phiênTrình duyệt này và 2 phiên khác",
    );
  });

  it("translates sessions, keeps the browser identifier as sent", () => {
    wrap(
      <SecuritySessions
        sessions={sessionsResult([session("current"), session("other")])}
        currentId="current"
        onChanged={vi.fn().mockResolvedValue(undefined)}
        onAccessError={vi.fn()}
      />,
    );
    expect(screen.getByText("Phiên này")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đăng xuất phiên này" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đăng xuất Safari trên macOS" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/203\.0\.113\.7 · Hoạt động/)).toHaveLength(2);
    expect(screen.getAllByText(/UTC/).length).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "Đăng xuất các phiên khác" }),
    ).toBeEnabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Đăng xuất tất cả phiên" }),
    );
    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText("Đăng xuất tất cả phiên?"),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "Bạn cũng sẽ bị đăng xuất khỏi trình duyệt này.",
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Xác nhận đăng xuất" }),
    ).toBeInTheDocument();
  });

  it("translates sign-in methods but keeps provider brands", () => {
    wrap(
      <SignInMethods
        accounts={[
          {
            id: "a",
            providerId: "credential",
            accountId: "x",
            userId: "u",
            createdAt: new Date("2026-09-02T12:00:00Z"),
            updatedAt: new Date(),
            scopes: [],
          },
          {
            id: "b",
            providerId: "google",
            accountId: "y",
            userId: "u",
            createdAt: new Date("2026-09-02T12:00:00Z"),
            updatedAt: new Date(),
            scopes: [],
          },
        ]}
        enabledProviders={["github"]}
        onChanged={vi.fn().mockResolvedValue(undefined)}
        onAccessError={vi.fn()}
      />,
    );
    expect(screen.getByText("Email và mật khẩu")).toBeInTheDocument();
    expect(screen.getAllByText("Đã kết nối")).toHaveLength(2);
    expect(
      screen.getByRole("button", { name: "Ngắt kết nối Google" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Kết nối GitHub" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Chưa kết nối")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Ngắt kết nối Google" }),
    );
    expect(screen.getByText("Ngắt kết nối Google?")).toBeInTheDocument();
  });

  it("translates the backup codes and leaves the codes themselves alone", () => {
    renderWithIntl(
      <BackupCodes codes={["abcd-1234", "efgh-5678"]} onDone={vi.fn()} />,
      {
        locale: "vi",
      },
    );
    expect(screen.getByRole("list", { name: "Mã dự phòng" })).toHaveTextContent(
      "abcd-1234efgh-5678",
    );
    expect(screen.getByRole("button", { name: "Sao chép mã" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Tải mã xuống" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Xong" })).toBeDisabled();
    expect(screen.getByLabelText("Tôi đã lưu các mã dự phòng")).toBeVisible();
  });

  it("translates the authenticator management dialog", () => {
    wrap(
      <TotpManagement
        enabled
        onChanged={vi.fn().mockResolvedValue(undefined)}
        onAccessError={vi.fn()}
      />,
    );
    expect(screen.getByText("Mã dự phòng")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Tắt ứng dụng xác thực" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Tắt ứng dụng xác thực?" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Mật khẩu hiện tại")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Xác nhận tắt" })).toBeVisible();
  });
});
