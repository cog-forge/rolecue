import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TotpEnrollmentDialog } from "@/features/auth/components/totp-enrollment-dialog";
const { enable, verifyTotp, success } = vi.hoisted(() => ({
  enable: vi.fn(),
  verifyTotp: vi.fn(),
  success: vi.fn(),
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success } }));
vi.mock("@/lib/auth/client", () => ({
  authClient: { twoFactor: { enable, verifyTotp } },
}));
function renderEnrollment() {
  const onChanged = vi.fn().mockResolvedValue(undefined);
  const cache = new QueryClient();
  renderWithIntl(
    <QueryClientProvider client={cache}>
      <TotpEnrollmentDialog onChanged={onChanged} onAccessError={vi.fn()} />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Enable authenticator" }));
  return { onChanged, cache };
}
async function start() {
  fireEvent.change(screen.getByLabelText("Current password", { exact: true }), {
    target: { value: "test-password" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Continue" }));
  await screen.findByLabelText("Authenticator code");
}
describe("authenticator enrollment", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    enable.mockResolvedValue({
      data: {
        method: "totp",
        totpURI: "otpauth://totp/RoleCue:test?secret=TESTSECRET&issuer=RoleCue",
        backupCodes: ["ABCDE-12345", "FGHIJ-67890"],
      },
      error: null,
    });
    verifyTotp.mockResolvedValue({ data: { status: true }, error: null });
  });
  it("requires verification before revealing backup codes and explicit saving before finishing", async () => {
    const { onChanged } = renderEnrollment();
    await start();
    expect(screen.getByText("TESTSECRET")).toBeInTheDocument();
    expect(screen.queryByText("ABCDE-12345")).not.toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "012345" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Verify authenticator" }),
    );
    expect(await screen.findByText("ABCDE-12345")).toBeInTheDocument();
    expect(verifyTotp).toHaveBeenCalledWith({
      code: "012345",
      trustDevice: false,
    });
    expect(success).toHaveBeenCalledWith("Authenticator enabled", {
      description: "Password sign-in now asks for a code.",
    });
    expect(screen.getByRole("button", { name: "Done" })).toBeDisabled();
    fireEvent.click(screen.getByLabelText("I have saved my backup codes"));
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(onChanged).toHaveBeenCalledOnce();
  });
  it("clears a cancelled setup and requires the password on reopening", async () => {
    renderEnrollment();
    await start();
    fireEvent.click(screen.getByRole("button", { name: "Cancel setup" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Enable authenticator" }),
    );
    expect(screen.queryByText("TESTSECRET")).not.toBeInTheDocument();
    expect(
      screen.getByLabelText("Current password", { exact: true }),
    ).toHaveValue("");
  });
  it("retains verification after an invalid code without showing success", async () => {
    verifyTotp.mockResolvedValue({
      data: null,
      error: { code: "INVALID_CODE" },
    });
    const { onChanged } = renderEnrollment();
    await start();
    fireEvent.change(screen.getByLabelText("Authenticator code"), {
      target: { value: "123456" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Verify authenticator" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "code is invalid",
    );
    expect(screen.queryByText("ABCDE-12345")).not.toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
  });
  it("provides a safe reauthentication link when the session is stale", async () => {
    enable.mockResolvedValue({
      data: null,
      error: { code: "SESSION_NOT_FRESH", status: 403 },
    });
    renderEnrollment();
    fireEvent.change(
      screen.getByLabelText("Current password", { exact: true }),
      { target: { value: "test-password" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    expect(
      await screen.findByRole("link", { name: "Sign in again" }),
    ).toHaveAttribute("href", "/login?returnTo=%2Fsettings");
  });
});
