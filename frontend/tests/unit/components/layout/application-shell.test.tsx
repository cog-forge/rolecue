import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { ApplicationShell } from "@/components/layout/application-shell";
import { renderWithIntl } from "@tests/helpers/intl";

const mocks = vi.hoisted(() => ({ pathname: "/dashboard" }));
vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
  useRouter: () => ({ refresh: vi.fn(), push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("@/providers/workspace-provider", () => ({
  useWorkspacePreferences: () => ({
    ready: true,
    mobile: false,
    collapsed: false,
    theme: "light",
    toggleCollapsed: vi.fn(),
    setCollapsed: vi.fn(),
    setTheme: vi.fn(),
  }),
}));
vi.mock("@/components/layout/workspace-sidebar", () => ({
  WorkspaceSidebar: () => <nav>Sidebar</nav>,
}));
vi.mock("@/components/layout/workspace-search", () => ({
  WorkspaceSearch: () => null,
}));

const user = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Nam",
  email: "nam@example.com",
  role: "candidate" as const,
  email_verified: true,
  is_locked: false,
  image: null,
  onboarding_role_selected: true,
  onboarding_completed: true,
};

function mount(children: ReactNode = <p>Content</p>, locale?: "en" | "vi") {
  return renderWithIntl(
    <ApplicationShell
      user={user}
      onSignOut={vi.fn()}
      signingOut={false}
      signOutError={false}
    >
      {children}
    </ApplicationShell>,
    { locale },
  );
}

describe("language switcher in the workspace", () => {
  beforeEach(() => {
    mocks.pathname = "/dashboard";
  });

  it.each(["candidate", "recruiter", "admin"] as const)(
    "is in the header for a %s",
    (role) => {
      renderWithIntl(
        <ApplicationShell
          user={{ ...user, role }}
          onSignOut={vi.fn()}
          signingOut={false}
          signOutError={false}
        >
          <p>Content</p>
        </ApplicationShell>,
      );
      expect(
        screen.getByRole("button", { name: "Change language: English" }),
      ).toBeInTheDocument();
    },
  );

  it("follows the active language", () => {
    mount(<p>Content</p>, "vi");
    expect(
      screen.getByRole("button", { name: "Đổi ngôn ngữ: Tiếng Việt" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Chuyển đến nội dung chính" }),
    ).toBeInTheDocument();
  });

  it("is not shown in the interview room, which has no workspace chrome", () => {
    mocks.pathname = "/interviews/abc-123/room";
    mount(<p>Room</p>);
    expect(screen.getByText("Room")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /language/i })).toBeNull();
  });
});
