import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { describe, expect, it, vi } from "vitest";
import { ResetPasswordForm } from "@/features/auth/components/reset-password-form";

vi.mock("@/lib/auth/client", () => ({
  authClient: { resetPassword: vi.fn() },
}));

describe("ResetPasswordForm translations", () => {
  it("shows the missing-link state in Vietnamese", () => {
    renderWithIntl(<ResetPasswordForm />, { locale: "vi" });
    expect(
      screen.getByRole("heading", { name: "Không có liên kết đặt lại" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Yêu cầu liên kết đặt lại mới" }),
    ).toHaveAttribute("href", "/forgot-password");
  });
});
