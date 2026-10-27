import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { RoleGate } from "@/features/auth/components/role-gate";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  user: { id: "u", role: "candidate" },
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace }),
  usePathname: () => "/avatar-studio",
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

beforeEach(() => vi.clearAllMocks());

describe("RoleGate", () => {
  it.each(["candidate", "recruiter"])(
    "admits a %s to a shared page",
    (role) => {
      mocks.user = { id: "u", role };
      render(
        <RoleGate role={["candidate", "recruiter"]}>
          <p>Studio</p>
        </RoleGate>,
      );
      expect(screen.getByText("Studio")).toBeInTheDocument();
      expect(mocks.replace).not.toHaveBeenCalled();
    },
  );

  it("sends an admin home without rendering the page", () => {
    mocks.user = { id: "u", role: "admin" };
    render(
      <RoleGate role={["candidate", "recruiter"]}>
        <p>Studio</p>
      </RoleGate>,
    );
    expect(screen.queryByText("Studio")).not.toBeInTheDocument();
    expect(mocks.replace).toHaveBeenCalledWith("/admin/dashboard");
  });

  it("still accepts a single role", () => {
    mocks.user = { id: "u", role: "recruiter" };
    render(
      <RoleGate role="candidate">
        <p>Practice</p>
      </RoleGate>,
    );
    expect(screen.queryByText("Practice")).not.toBeInTheDocument();
    expect(mocks.replace).toHaveBeenCalledWith("/recruiter/dashboard");
  });
});
