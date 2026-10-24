import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TotpManagement } from "@/features/auth/components/totp-management";
const { disable, generateBackupCodes, success } = vi.hoisted(() => ({
  disable: vi.fn(),
  generateBackupCodes: vi.fn(),
  success: vi.fn(),
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success } }));
vi.mock("@/lib/auth/client", () => ({
  authClient: { twoFactor: { disable, generateBackupCodes } },
}));
function renderManagement() {
  const onChanged = vi.fn().mockResolvedValue(undefined);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <TotpManagement enabled onChanged={onChanged} onAccessError={vi.fn()} />
    </QueryClientProvider>,
  );
  return onChanged;
}
describe("authenticator management", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    disable.mockResolvedValue({ data: { status: true }, error: null });
    generateBackupCodes.mockResolvedValue({
      data: { backupCodes: ["NEWCODE-12345"] },
      error: null,
    });
  });
  it("disables only after a password-confirmed action and refreshes server state", async () => {
    const onChanged = renderManagement();
    fireEvent.click(
      screen.getByRole("button", { name: "Disable authenticator" }),
    );
    expect(disable).not.toHaveBeenCalled();
    fireEvent.change(
      screen.getByLabelText("Current password", { exact: true }),
      { target: { value: "test-password" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Confirm disable" }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(disable).toHaveBeenCalledWith({ password: "test-password" });
    expect(onChanged).toHaveBeenCalledOnce();
    expect(success).toHaveBeenCalledWith("Authenticator disabled", {
      description: "Password sign-in no longer asks for a code.",
    });
  });
  it("warns that regeneration invalidates old codes and requires saving the replacement", async () => {
    renderManagement();
    fireEvent.click(
      screen.getByRole("button", { name: "Regenerate backup codes" }),
    );
    expect(
      screen.getByText(/old backup codes will stop working/),
    ).toBeInTheDocument();
    fireEvent.change(
      screen.getByLabelText("Current password", { exact: true }),
      { target: { value: "test-password" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Generate new codes" }));
    expect(await screen.findByText("NEWCODE-12345")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Done" })).toBeDisabled();
    expect(generateBackupCodes).toHaveBeenCalledWith({
      password: "test-password",
    });
  });
});
