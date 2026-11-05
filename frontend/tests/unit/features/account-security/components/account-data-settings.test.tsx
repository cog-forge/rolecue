import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AccountDataSettings,
  DeleteAccountSettings,
} from "@/features/account-security/components/account-data-settings";

const { fetchMyDataExport, deleteMyAccount, signOut, endLocalSession, user } =
  vi.hoisted(() => ({
    fetchMyDataExport: vi.fn(),
    deleteMyAccount: vi.fn(),
    signOut: vi.fn(),
    endLocalSession: vi.fn(),
    user: { role: "candidate" },
  }));

vi.mock("@/features/account-security/api/account", () => ({
  EXPORT_LINK_MINUTES: 15,
  ERASURE_CONFIRMATION: "DELETE",
  fetchMyDataExport,
  deleteMyAccount,
}));
vi.mock("@/lib/auth/client", () => ({ authClient: { signOut } }));
vi.mock("@/features/auth/hooks/use-sign-out", () => ({
  useEndLocalSession: () => endLocalSession,
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => user,
}));
vi.mock("goey-toast", () => ({
  gooeyToast: { success: vi.fn(), error: vi.fn() },
}));

describe("DeleteAccountSettings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    user.role = "candidate";
  });

  function openDialog() {
    renderWithIntl(<DeleteAccountSettings />);
    fireEvent.click(screen.getByRole("button", { name: "Delete account" }));
  }

  it("keeps the delete button disabled until the confirmation word is typed", () => {
    openDialog();
    const confirm = screen.getByRole("button", { name: "Delete my account" });
    expect(confirm).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Type DELETE to confirm"), {
      target: { value: "delete" },
    });
    expect(confirm).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Type DELETE to confirm"), {
      target: { value: "DELETE" },
    });
    expect(confirm).toBeEnabled();
  });

  it("deletes, then ends the session", async () => {
    deleteMyAccount.mockResolvedValueOnce(undefined);
    signOut.mockResolvedValueOnce({ error: null });
    openDialog();
    fireEvent.change(screen.getByLabelText("Type DELETE to confirm"), {
      target: { value: "DELETE" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Delete my account" }));
    await waitFor(() => expect(endLocalSession).toHaveBeenCalledTimes(1));
    expect(deleteMyAccount).toHaveBeenCalledTimes(1);
    expect(signOut).toHaveBeenCalledTimes(1);
  });

  it("stays signed in and shows the reason when the API refuses", async () => {
    deleteMyAccount.mockRejectedValueOnce(
      Object.assign(new Error("conflict"), {
        isAxiosError: true,
        response: {
          status: 409,
          data: {
            success: false,
            error: { code: "ACCOUNT_HAS_ACTIVE_WORK", message: "busy" },
          },
        },
      }),
    );
    openDialog();
    fireEvent.change(screen.getByLabelText("Type DELETE to confirm"), {
      target: { value: "DELETE" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Delete my account" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "You cannot delete your account",
    );
    expect(endLocalSession).not.toHaveBeenCalled();
    expect(signOut).not.toHaveBeenCalled();
  });

  it("is not offered to an administrator", () => {
    user.role = "admin";
    renderWithIntl(<DeleteAccountSettings />);
    expect(screen.queryByRole("button", { name: "Delete account" })).toBeNull();
  });
});

describe("AccountDataSettings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    user.role = "candidate";
  });

  it("fetches the export when asked and states the link lifetime", async () => {
    fetchMyDataExport.mockResolvedValueOnce({ link_ttl_seconds: 900 });
    Object.assign(URL, {
      createObjectURL: vi.fn(() => "blob:test"),
      revokeObjectURL: vi.fn(),
    });
    renderWithIntl(<AccountDataSettings />);
    expect(screen.getByText(/expire after 15 minutes/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Download my data" }));
    await waitFor(() => expect(URL.revokeObjectURL).toHaveBeenCalled(), {
      timeout: 3000,
    });
    expect(fetchMyDataExport).toHaveBeenCalledTimes(1);
  });
});
