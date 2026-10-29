import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AccountSecurity,
  PasswordSettings,
} from "@/features/account-security/components/account-security";

const { listAccounts, changePassword } = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  changePassword: vi.fn(),
}));

vi.mock("@/lib/auth/client", () => ({
  authClient: { listAccounts, changePassword },
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: vi.fn() } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: userId, email: "alex@example.test" }),
}));

const userId = "11111111-1111-4111-8111-111111111111";

function accounts(...providerIds: string[]) {
  return {
    data: providerIds.map((providerId, index) => ({
      id: String(index),
      userId,
      providerId,
    })),
    error: null,
  };
}

function renderPage() {
  renderWithIntl(
    <QueryClientProvider client={new QueryClient()}>
      <AccountSecurity>
        <PasswordSettings />
      </AccountSecurity>
    </QueryClientProvider>,
  );
}

describe("AccountSecurity", () => {
  beforeEach(() => vi.clearAllMocks());

  it("names the screen and the account it secures", async () => {
    listAccounts.mockResolvedValueOnce(accounts("credential"));
    renderPage();
    expect(
      screen.getByRole("heading", { level: 1, name: "Account security" }),
    ).toBeInTheDocument();
    expect(screen.getByText("alex@example.test")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Edit profile" })).toHaveAttribute(
      "href",
      "/profile",
    );
    expect(await screen.findByText("Set")).toBeInTheDocument();
  });

  it("offers the change password dialog to an account with a password", async () => {
    listAccounts.mockResolvedValueOnce(accounts("credential", "google"));
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Change password" }),
    );
    expect(
      await screen.findByRole("dialog", { name: "Change password" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Current password")).toHaveValue("");
  });

  it("explains that a social-only account has no password", async () => {
    listAccounts.mockResolvedValueOnce(accounts("google", "github"));
    renderPage();
    expect(
      await screen.findByText(/You sign in with Google or GitHub/),
    ).toHaveTextContent(
      "You sign in with Google or GitHub, so this account has no password to change.",
    );
    expect(
      screen.getByRole("link", { name: "Forgot password" }),
    ).toHaveAttribute("href", "/forgot-password");
    expect(
      screen.queryByRole("button", { name: "Change password" }),
    ).not.toBeInTheDocument();
  });

  it("shows an error with a retry when sign-in methods cannot load", async () => {
    listAccounts.mockResolvedValueOnce({
      data: null,
      error: { status: 500, message: "boom" },
    });
    listAccounts.mockResolvedValueOnce(accounts("credential"));
    renderPage();
    expect(
      await screen.findByText("We couldn't load your sign-in methods."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Change password" }),
      ).toBeInTheDocument(),
    );
  });

  it("switches to the no-password note when the server finds no password", async () => {
    listAccounts.mockResolvedValueOnce(accounts("credential"));
    changePassword.mockResolvedValueOnce({
      data: null,
      error: { code: "CREDENTIAL_ACCOUNT_NOT_FOUND", status: 400 },
    });
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Change password" }),
    );
    fireEvent.change(screen.getByLabelText("Current password"), {
      target: { value: "Current-Secret-1" },
    });
    fireEvent.change(screen.getByLabelText("New password"), {
      target: { value: "Brand-New-Secret-2" },
    });
    fireEvent.change(screen.getByLabelText("Confirm new password"), {
      target: { value: "Brand-New-Secret-2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Update password" }));
    expect(
      await screen.findByText(/this account has no password to change/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Change password" }),
    ).not.toBeInTheDocument();
    expect(listAccounts).toHaveBeenCalledOnce();
  });

  it("clears typed passwords when the dialog is closed", async () => {
    listAccounts.mockResolvedValueOnce(accounts("credential"));
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Change password" }),
    );
    fireEvent.change(screen.getByLabelText("Current password"), {
      target: { value: "Typed-Secret-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByLabelText("Current password")).toHaveValue("");
  });
});
