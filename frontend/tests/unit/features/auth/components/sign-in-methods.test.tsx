import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SignInMethods } from "@/features/auth/components/sign-in-methods";
const { unlinkAccount, linkSocial, success } = vi.hoisted(() => ({
  unlinkAccount: vi.fn(),
  linkSocial: vi.fn(),
  success: vi.fn(),
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success } }));
vi.mock("@/lib/auth/client", () => ({
  authClient: { unlinkAccount, linkSocial },
}));
const account = (providerId: string) => ({
  id: `${providerId}-record`,
  providerId,
  accountId: `external-${providerId}`,
  userId: "user-1",
  createdAt: new Date(),
  updatedAt: new Date(),
  scopes: [],
});
function renderMethods(
  providers = ["credential", "google"],
  enabledProviders: ("google" | "github" | "facebook")[] = ["github"],
) {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <SignInMethods
        accounts={providers.map(account)}
        enabledProviders={enabledProviders}
        onChanged={vi.fn().mockResolvedValue(undefined)}
        onAccessError={vi.fn()}
      />
    </QueryClientProvider>,
  );
}
describe("connected sign-in methods", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    unlinkAccount.mockResolvedValue({ data: { status: true }, error: null });
    linkSocial.mockResolvedValue({
      data: { url: "https://github.com/login", redirect: true },
      error: null,
    });
  });
  it("unlinks using the account record ID, even for a currently unconfigured provider", async () => {
    renderMethods();
    fireEvent.click(screen.getByRole("button", { name: "Disconnect Google" }));
    expect(unlinkAccount).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm disconnect" }));
    await waitFor(() =>
      expect(unlinkAccount).toHaveBeenCalledWith({
        accountId: "google-record",
      }),
    );
    await waitFor(() =>
      expect(success).toHaveBeenCalledWith("Google disconnected"),
    );
  });
  it("protects the last sign-in method and only offers configured providers", () => {
    renderMethods(["google"]);
    expect(
      screen.getByRole("button", { name: "Disconnect Google" }),
    ).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Connect Facebook" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Disconnect Email/ }),
    ).not.toBeInTheDocument();
  });
  it("links only through the SDK with fixed local callbacks", async () => {
    renderMethods();
    fireEvent.click(screen.getByRole("button", { name: "Connect GitHub" }));
    await waitFor(() =>
      expect(linkSocial).toHaveBeenCalledWith({
        provider: "github",
        callbackURL: "/settings?linked=success",
        errorCallbackURL: "/settings?linked=failed",
      }),
    );
  });
});
