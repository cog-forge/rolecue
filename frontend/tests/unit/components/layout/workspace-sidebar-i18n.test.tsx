import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { WorkspaceSidebar } from "@/components/layout/workspace-sidebar";
import { TooltipProvider } from "@/components/ui/tooltip";
import { WorkspaceSearch } from "@/components/layout/workspace-search";

const mocks = vi.hoisted(() => ({ pathname: "/dashboard" }));
vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
}));
vi.mock("@/providers/workspace-provider", () => ({
  useWorkspacePreferences: () => ({
    ready: true,
    mobile: false,
    collapsed: false,
    reducedMotion: true,
    theme: "light",
    toggleCollapsed: vi.fn(),
    setCollapsed: vi.fn(),
    setTheme: vi.fn(),
  }),
}));

const user = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Nguyễn Văn An",
  email: "an@example.com",
  role: "candidate" as const,
};

function sidebar(
  role: "candidate" | "recruiter" | "admin",
  locale: "en" | "vi",
) {
  return renderWithIntl(
    <TooltipProvider>
      <WorkspaceSidebar
        user={{ ...user, role }}
        collapsed={false}
        onToggle={vi.fn()}
        onSearch={vi.fn()}
        onNavigate={vi.fn()}
        onSignOut={vi.fn()}
        signingOut={false}
        signOutError={false}
        navigationBadges={{
          "/jobs": { count: 3, label: "tin tuyển dụng mới hôm nay" },
        }}
      />
    </TooltipProvider>,
    { locale },
  );
}

describe("workspace sidebar", () => {
  it("keeps the English navigation for every role", () => {
    sidebar("candidate", "en");
    const nav = screen.getByRole("navigation", {
      name: "Candidate navigation",
    });
    expect(within(nav).getByText("Practice")).toBeInTheDocument();
    expect(
      within(nav).getByRole("link", { name: "Personal Avatar Studio" }),
    ).toHaveAttribute("href", "/avatar-studio");
    expect(screen.getByText("Candidate workspace")).toBeInTheDocument();
  });

  it("translates the candidate navigation and keeps the user's name", () => {
    sidebar("candidate", "vi");
    const nav = screen.getByRole("navigation", { name: "Điều hướng ứng viên" });
    expect(within(nav).getByText("Luyện tập")).toBeInTheDocument();
    expect(within(nav).getByText("Sự nghiệp")).toBeInTheDocument();
    expect(
      within(nav).getByRole("link", { name: "Tổng quan" }),
    ).toHaveAttribute("href", "/dashboard");
    expect(
      within(nav).getByRole("link", { name: "Hồ sơ vị trí" }),
    ).toHaveAttribute("href", "/role-profiles");
    expect(
      within(nav).getByRole("link", { name: "Bảng tin tuyển dụng" }),
    ).toHaveAttribute("href", "/jobs");
    expect(
      within(nav).getByRole("link", { name: "Đơn ứng tuyển của tôi" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Không gian ứng viên")).toBeInTheDocument();
    expect(
      screen.getByText("3 tin tuyển dụng mới hôm nay"),
    ).toBeInTheDocument();
    expect(screen.getByText("Nguyễn Văn An")).toBeInTheDocument();
    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
  });

  it("translates the recruiter and admin navigation", () => {
    const first = sidebar("recruiter", "vi");
    expect(
      screen.getByRole("navigation", { name: "Điều hướng nhà tuyển dụng" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Tin tuyển dụng của tôi" }),
    ).toBeInTheDocument();
    first.unmount();
    sidebar("admin", "vi");
    expect(
      screen.getByRole("navigation", { name: "Điều hướng quản trị viên" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Kiểm duyệt tin tuyển dụng" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Báo cáo doanh thu" }),
    ).toBeInTheDocument();
  });

  it("translates the account menu", async () => {
    sidebar("candidate", "vi");
    const trigger = screen.getByRole("button", { name: "Mở menu tài khoản" });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    expect(
      await screen.findByRole("menuitem", { name: "Hồ sơ của bạn" }),
    ).toHaveAttribute("href", "/profile");
    expect(
      screen.getByRole("menuitem", { name: "Bảo mật tài khoản" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitemradio", { name: "Tối" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Đăng xuất" }),
    ).toBeInTheDocument();
    expect(screen.getByText("an@example.com")).toBeInTheDocument();
  });
});

describe("workspace search", () => {
  it("searches the translated page names", () => {
    renderWithIntl(
      <WorkspaceSearch
        role="candidate"
        open
        onOpenChange={vi.fn()}
        onNavigate={vi.fn()}
        returnFocus={vi.fn()}
      />,
      { locale: "vi" },
    );
    expect(screen.getByText("Tìm một trang")).toBeInTheDocument();
    const input = screen.getByRole("combobox", {
      name: "Tìm trang trong không gian làm việc",
    });
    expect(screen.getAllByRole("option")).toHaveLength(7);
    fireEvent.change(input, { target: { value: "lịch sử" } });
    expect(screen.getAllByRole("option")).toHaveLength(1);
    expect(
      screen.getByRole("option", { name: "Lịch sử phỏng vấn" }),
    ).toBeInTheDocument();
    fireEvent.change(input, { target: { value: "xyz" } });
    expect(screen.getByText("Không có trang phù hợp.")).toBeInTheDocument();
  });
});
