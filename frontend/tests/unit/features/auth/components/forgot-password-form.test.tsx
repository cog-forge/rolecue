import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ForgotPasswordForm } from "@/features/auth/components/forgot-password-form";

const { requestPasswordReset, info, error } = vi.hoisted(() => ({
  requestPasswordReset: vi.fn(),
  info: vi.fn(),
  error: vi.fn(),
}));

vi.mock("@/lib/auth/client", () => ({
  authClient: { requestPasswordReset },
}));
vi.mock("goey-toast", () => ({ gooeyToast: { info, error } }));

describe("ForgotPasswordForm", () => {
  beforeEach(() => vi.clearAllMocks());

  it.each(["sdk", "network"])(
    "keeps the form available after a %s failure and permits retry",
    async (failure) => {
      if (failure === "sdk")
        requestPasswordReset.mockResolvedValueOnce({
          error: { message: "Internal provider detail" },
        });
      else requestPasswordReset.mockRejectedValueOnce(new Error("Offline"));
      requestPasswordReset.mockResolvedValueOnce({ data: {}, error: null });
      render(<ForgotPasswordForm />);
      fireEvent.change(screen.getByLabelText("Email address"), {
        target: { value: "candidate@example.com" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Send reset link" }));
      await waitFor(() => expect(error).toHaveBeenCalledOnce());
      expect(info).not.toHaveBeenCalled();
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
      const retry = screen.getByRole("button", { name: "Send reset link" });
      await waitFor(() => expect(retry).toBeEnabled());
      fireEvent.click(retry);
      await waitFor(() => expect(info).toHaveBeenCalledOnce());
      expect(requestPasswordReset).toHaveBeenCalledTimes(2);
      expect(screen.getByRole("status")).toHaveTextContent(
        "If an account exists",
      );
      expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
    },
  );
});
