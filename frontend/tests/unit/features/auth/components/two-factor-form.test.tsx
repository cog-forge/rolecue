import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TwoFactorForm } from "@/features/auth/components/two-factor-form";

const { replace, verifyTotp, verifyBackupCode, getSession } = vi.hoisted(
  () => ({
    replace: vi.fn(),
    verifyTotp: vi.fn(),
    verifyBackupCode: vi.fn(),
    getSession: vi.fn(),
  }),
);
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("@/lib/auth/client", () => ({
  authClient: { twoFactor: { verifyTotp, verifyBackupCode }, getSession },
}));

function renderForm(returnTo?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  renderWithIntl(
    <QueryClientProvider client={client}>
      <TwoFactorForm returnTo={returnTo} />
    </QueryClientProvider>,
  );
}

describe("second-factor login", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    verifyTotp.mockResolvedValue({ data: { user: {} }, error: null });
    verifyBackupCode.mockResolvedValue({ data: { user: {} }, error: null });
    getSession.mockResolvedValue({
      data: { user: { role: "candidate" } },
      error: null,
    });
  });

  it("preserves leading zeroes and defaults to not trusting the browser", async () => {
    renderForm();
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "012345" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    await waitFor(() =>
      expect(replace).toHaveBeenCalledWith("/dashboard?auth=signed-in"),
    );
    expect(verifyTotp).toHaveBeenCalledWith({
      code: "012345",
      trustDevice: false,
    });
  });

  it("keeps an invalid code on the challenge rather than treating 401 as logout", async () => {
    verifyTotp.mockResolvedValue({
      data: null,
      error: { code: "INVALID_CODE", status: 401 },
    });
    renderForm();
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "That code is invalid",
    );
    expect(replace).not.toHaveBeenCalled();
    expect(getSession).not.toHaveBeenCalled();
  });

  it("uses a recovery code to create a session and returns only to settings", async () => {
    renderForm("/settings");
    fireEvent.click(screen.getByRole("button", { name: "Use a backup code" }));
    fireEvent.change(screen.getByLabelText("Backup code"), {
      target: { value: "AbC12-DE345" },
    });
    fireEvent.click(screen.getByLabelText("Remember this browser for 30 days"));
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    await waitFor(() =>
      expect(replace).toHaveBeenCalledWith("/settings?auth=signed-in"),
    );
    expect(verifyBackupCode).toHaveBeenCalledWith({
      code: "AbC12-DE345",
      disableSession: false,
      trustDevice: true,
    });
    expect(verifyTotp).not.toHaveBeenCalled();
  });

  it("offers a new login when the challenge has expired", async () => {
    verifyTotp.mockResolvedValue({
      data: null,
      error: { code: "INVALID_TWO_FACTOR_COOKIE", status: 401 },
    });
    renderForm();
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Sign in again");
    expect(
      screen.getByRole("link", { name: "Back to sign in" }),
    ).toHaveAttribute("href", "/login");
    expect(replace).not.toHaveBeenCalled();
  });

  it("does not accept a nonlocal return destination", async () => {
    renderForm("https://example.com");
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    await waitFor(() =>
      expect(replace).toHaveBeenCalledWith("/dashboard?auth=signed-in"),
    );
  });

  it("waits for an explicit retry after rate limiting without losing the code", async () => {
    verifyTotp.mockResolvedValueOnce({
      data: null,
      error: { status: 429, code: "TOO_MANY_REQUESTS" },
    });
    renderForm();
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "012345" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Too many requests",
    );
    expect(verifyTotp).toHaveBeenCalledOnce();
    expect(screen.getByLabelText("Authenticator code")).toHaveValue("012345");
    expect(replace).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    await waitFor(() =>
      expect(replace).toHaveBeenCalledWith("/dashboard?auth=signed-in"),
    );
  });
  it("does not complete sign-in when session confirmation is unavailable", async () => {
    getSession.mockResolvedValue({ data: null, error: { status: 503 } });
    renderForm("/settings");
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "012345" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify and sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "couldn't complete",
    );
    expect(replace).not.toHaveBeenCalled();
  });
});
