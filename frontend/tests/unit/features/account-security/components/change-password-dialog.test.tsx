import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ChangePasswordDialog } from "@/features/account-security/components/change-password-dialog";

const { changePassword, success } = vi.hoisted(() => ({
  changePassword: vi.fn(),
  success: vi.fn(),
}));

vi.mock("@/lib/auth/client", () => ({ authClient: { changePassword } }));
vi.mock("goey-toast", () => ({ gooeyToast: { success } }));

const current = "Current-Secret-1";
const next = "Brand-New-Secret-2";

function renderDialog() {
  const onOpenChange = vi.fn();
  const onNoPassword = vi.fn();
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, "invalidateQueries");
  render(
    <QueryClientProvider client={client}>
      <ChangePasswordDialog
        open
        onOpenChange={onOpenChange}
        onNoPassword={onNoPassword}
      />
    </QueryClientProvider>,
  );
  return { onOpenChange, onNoPassword, invalidate };
}

function fill(values: { current?: string; next?: string; confirm?: string }) {
  fireEvent.change(screen.getByLabelText("Current password"), {
    target: { value: values.current ?? current },
  });
  fireEvent.change(screen.getByLabelText("New password"), {
    target: { value: values.next ?? next },
  });
  fireEvent.change(screen.getByLabelText("Confirm new password"), {
    target: { value: values.confirm ?? values.next ?? next },
  });
}

function submit() {
  fireEvent.click(screen.getByRole("button", { name: "Update password" }));
}

// No typed password may reach a toast or a rendered message.
function expectNoPasswordLeak() {
  const toastText = JSON.stringify(success.mock.calls);
  const alertText = screen
    .queryAllByRole("alert")
    .map((alert) => alert.textContent)
    .join(" ");
  for (const secret of [current, next]) {
    expect(toastText).not.toContain(secret);
    expect(alertText).not.toContain(secret);
  }
}

describe("ChangePasswordDialog", () => {
  beforeEach(() => vi.clearAllMocks());

  it("changes the password, signs out other devices by default and closes", async () => {
    changePassword.mockResolvedValueOnce({ data: { token: "t" }, error: null });
    const { onOpenChange } = renderDialog();
    fill({});
    submit();
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(changePassword).toHaveBeenCalledWith({
      currentPassword: current,
      newPassword: next,
      revokeOtherSessions: true,
    });
    expect(success).toHaveBeenCalledWith("Password updated", {
      description: "Your other devices have been signed out.",
    });
    expectNoPasswordLeak();
  });

  it("keeps other devices signed in when the box is unticked", async () => {
    changePassword.mockResolvedValueOnce({
      data: { token: null },
      error: null,
    });
    const { onOpenChange } = renderDialog();
    fill({});
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Sign out of other devices/ }),
    );
    submit();
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(changePassword).toHaveBeenCalledWith(
      expect.objectContaining({ revokeOtherSessions: false }),
    );
  });

  it("shows a field error when the current password is wrong", async () => {
    changePassword.mockResolvedValueOnce({
      data: null,
      error: {
        code: "INVALID_PASSWORD",
        status: 400,
        message: "Invalid password",
      },
    });
    const { onOpenChange } = renderDialog();
    fill({});
    submit();
    expect(
      await screen.findByText("Current password is incorrect."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Current password")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(success).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalled();
    expectNoPasswordLeak();
  });

  it.each([
    [{ next: "short" }, "Use at least 8 characters"],
    [{ confirm: "Something-Else-3" }, "Passwords do not match"],
    [{ next: current }, "Choose a password different from your current one"],
    [{ current: "" }, "Enter your current password"],
  ])("blocks invalid input %o", async (values, message) => {
    renderDialog();
    fill(values);
    submit();
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(changePassword).not.toHaveBeenCalled();
  });

  it("maps a server-side length error onto the new password", async () => {
    changePassword.mockResolvedValueOnce({
      data: null,
      error: { code: "PASSWORD_TOO_SHORT", status: 400 },
    });
    renderDialog();
    fill({});
    submit();
    expect(
      await screen.findByText("Use at least 8 characters"),
    ).toBeInTheDocument();
  });

  it("asks the user to sign in again when the session is gone", async () => {
    changePassword.mockResolvedValueOnce({
      data: null,
      error: { code: "UNAUTHORIZED", status: 401, message: "Unauthorized" },
    });
    const { invalidate } = renderDialog();
    fill({});
    submit();
    expect(
      await screen.findByText(
        "Your session expired. Sign in again to change your password.",
      ),
    ).toBeInTheDocument();
    expect(success).not.toHaveBeenCalled();
    // SessionGate rechecks the session and redirects to sign in.
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["auth", "me"] });
  });

  it("ignores Escape while the change is in flight", async () => {
    let finish: (value: unknown) => void = () => {};
    changePassword.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const { onOpenChange } = renderDialog();
    fill({});
    submit();
    await waitFor(() => expect(changePassword).toHaveBeenCalledOnce());
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onOpenChange).not.toHaveBeenCalled();
    finish({ data: {}, error: null });
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(success).toHaveBeenCalledOnce();
  });

  it("reports an account without a password to the parent", async () => {
    changePassword.mockResolvedValueOnce({
      data: null,
      error: { code: "CREDENTIAL_ACCOUNT_NOT_FOUND", status: 400 },
    });
    const { onOpenChange, onNoPassword } = renderDialog();
    fill({});
    submit();
    await waitFor(() => expect(onNoPassword).toHaveBeenCalledOnce());
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it.each(["rate limit", "network"])(
    "allows a retry after a %s failure",
    async (failure) => {
      if (failure === "rate limit")
        changePassword.mockResolvedValueOnce({
          data: null,
          error: { status: 429, message: "Too many requests" },
        });
      else changePassword.mockRejectedValueOnce(new Error("Offline"));
      changePassword.mockResolvedValueOnce({ data: {}, error: null });
      const { onOpenChange } = renderDialog();
      fill({});
      submit();
      expect(
        await screen.findByText(
          failure === "rate limit"
            ? "Too many attempts. Wait a few seconds and try again."
            : "Your password could not be changed. Please try again.",
        ),
      ).toBeInTheDocument();
      expectNoPasswordLeak();
      const retry = screen.getByRole("button", { name: "Update password" });
      await waitFor(() => expect(retry).toBeEnabled());
      submit();
      await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
      expect(changePassword).toHaveBeenCalledTimes(2);
    },
  );
});
