import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ProfileOverview } from "@/features/profile/components/profile-overview";
import type { Profile } from "@/features/profile/schemas/profile-schema";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  patch: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Nguyễn Văn An",
    email: "an@example.com",
    role: "recruiter" as "candidate" | "recruiter" | "admin",
    email_verified: true,
    is_locked: false,
    onboarding_role_selected: true,
    onboarding_completed: true,
    image: null as string | null,
  },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, patch: mocks.patch },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const profile: Profile = {
  ...mocks.user,
  company_name: "Acme Studio",
  company_website: "https://acme.example",
  created_at: "2026-10-07T16:00:00Z",
  updated_at: "2026-10-07T16:00:00Z",
};

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  renderWithIntl(
    <QueryClientProvider client={client}>
      <ProfileOverview />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockResolvedValue({ data: { success: true, data: profile } });
});

describe("own profile screen in Vietnamese", () => {
  it("translates the headings, labels and summary, but not the stored values", async () => {
    mount();
    expect(
      screen.getByRole("heading", { level: 1, name: "Hồ sơ của bạn" }),
    ).toBeInTheDocument();
    expect(await screen.findByLabelText("Tên hiển thị")).toHaveValue(
      "Nguyễn Văn An",
    );
    expect(screen.getByLabelText("URL avatar")).toBeInTheDocument();
    expect(screen.getByLabelText("Tên công ty")).toHaveValue("Acme Studio");
    expect(screen.getByLabelText("Website công ty")).toHaveValue(
      "https://acme.example",
    );
    expect(
      screen.getByRole("heading", { name: "Thông tin công ty" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Nhà tuyển dụng")).toBeInTheDocument();
    expect(screen.getByText("Địa chỉ email")).toBeInTheDocument();
    expect(screen.getByText("Đã xác minh")).toBeInTheDocument();
    expect(screen.getByText("an@example.com")).toBeInTheDocument();
    expect(screen.getByText(/tháng 10/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Bảo mật tài khoản" }),
    ).toHaveAttribute("href", "/settings");
    expect(screen.getByRole("button", { name: "Lưu thay đổi" })).toBeDisabled();
  });

  it("validates in Vietnamese", async () => {
    mount();
    const name = await screen.findByLabelText("Tên hiển thị");
    fireEvent.change(name, { target: { value: "   " } });
    await waitFor(() =>
      expect(
        screen.getByText("Nhập tên hiển thị từ 1 đến 100 ký tự."),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hủy thay đổi" })).toBeEnabled();
  });

  it("shows a translated message when the profile cannot load", async () => {
    mocks.get.mockRejectedValue(new Error("down"));
    mount();
    expect(
      await screen.findByText("Không thể tải hồ sơ của bạn"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Vui lòng thử lại sau ít phút."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeEnabled();
  });
});
