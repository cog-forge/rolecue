import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountCredentials } from "@/features/auth/components/account-credentials";

const { getSession, listAccounts, listSessions, replace, enable } = vi.hoisted(
  () => ({
    getSession: vi.fn(),
    listAccounts: vi.fn(),
    listSessions: vi.fn(),
    replace: vi.fn(),
    enable: vi.fn(),
  }),
);
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({
    id: "user-1",
    email: "alex@example.com",
    role: "candidate",
  }),
}));
vi.mock("@/lib/auth/client", () => ({
  authClient: { getSession, listAccounts, listSessions, twoFactor: { enable } },
}));

function renderPage() {
  const cache = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={cache}>
      <AccountCredentials enabledProviders={[]} />
    </QueryClientProvider>,
  );
}

describe("account credentials reads", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getSession.mockResolvedValue({
      error: null,
      data: {
        user: {
          id: "user-1",
          email: "alex@example.com",
          emailVerified: true,
          twoFactorEnabled: false,
          createdAt: new Date("2026-01-01"),
        },
        session: { id: "current", userId: "user-1" },
      },
    });
    listAccounts.mockResolvedValue({ error: null, data: [] });
    listSessions.mockResolvedValue({ error: null, data: [] });
  });

  it("summarises protection and withholds TOTP from OAuth-only accounts", async () => {
    renderPage();
    const summary = await screen.findByRole("navigation", {
      name: "Security summary",
    });
    expect(summary).toHaveTextContent("EmailVerified");
    expect(summary).toHaveTextContent("PasswordNot set");
    expect(summary).toHaveTextContent("Two-factorOff");
    expect(
      screen.getByText(/Add a password to your account first/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Enable authenticator" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Change password" }),
    ).not.toBeInTheDocument();
  });

  it("does not infer no password from an account-list error", async () => {
    listAccounts.mockResolvedValue({
      data: null,
      error: { code: "INTERNAL_SERVER_ERROR" },
    });
    renderPage();
    expect(
      await screen.findByText(/couldn.t complete this request/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Create a password" }),
    ).not.toBeInTheDocument();
  });

  it("hides sensitive content and rejects an unexpected account identity", async () => {
    getSession.mockResolvedValue({
      error: null,
      data: { user: { id: "another-user" }, session: {} },
    });
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "session has expired",
    );
    expect(screen.queryByText("alex@example.com")).not.toBeInTheDocument();
    expect(replace).toHaveBeenCalledWith("/login?session=ended");
  });

  it("clears a failed security flow and requires reloading before an uncertain request can be repeated", async () => {
    listAccounts.mockResolvedValue({
      data: [
        { id: "credential-record", providerId: "credential", userId: "user-1" },
      ],
      error: null,
    });
    enable.mockRejectedValue(new Error("Connection lost"));
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Enable authenticator" }),
    );
    fireEvent.change(
      screen.getByLabelText("Current password", { exact: true }),
      { target: { value: "current-password" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    expect(
      await screen.findByRole("button", { name: "Reload account security" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(enable).toHaveBeenCalledOnce();
  });
});
