import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginForm } from "@/features/auth/components/login-form";
import { login } from "@/features/auth/api/login";
import { useAuthStore } from "@/stores/auth-store";

const { replace, toastError } = vi.hoisted(() => ({
  replace: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("@/features/auth/api/login", () => ({ login: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("sonner", () => ({ toast: { error: toastError } }));

function renderLoginForm() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LoginForm />
    </QueryClientProvider>,
  );
}

describe("LoginForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuthStore.getState().clearSession();
  });

  it("validates credentials, calls Login, stores its token and redirects", async () => {
    vi.mocked(login).mockResolvedValue({ accessToken: "test-token" });
    renderLoginForm();

    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "  Candidate@Example.com " },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "passphrase" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() =>
      expect(vi.mocked(login).mock.calls[0]?.[0]).toEqual({
        email: "Candidate@Example.com",
        password: "passphrase",
      }),
    );
    expect(useAuthStore.getState().accessToken).toBe("test-token");
    expect(replace).toHaveBeenCalledWith("/dashboard");
  });

  it("shows normalized backend errors through Sonner", async () => {
    vi.mocked(login).mockRejectedValue({
      isAxiosError: true,
      response: {
        data: { success: false, error: { message: "Invalid credentials." } },
      },
    });
    renderLoginForm();
    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "candidate@example.com" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "wrong-password" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Invalid credentials."),
    );
    expect(useAuthStore.getState().accessToken).toBeNull();
    expect(replace).not.toHaveBeenCalled();
  });

  it("links all provider choices to their preview routes", () => {
    renderLoginForm();
    expect(
      screen.getByRole("link", { name: "Continue with Google" }),
    ).toHaveAttribute("href", "/login/oauth/google");
    expect(
      screen.getByRole("link", { name: "Continue with GitHub" }),
    ).toHaveAttribute("href", "/login/oauth/github");
    expect(
      screen.getByRole("link", { name: "Continue with Facebook" }),
    ).toHaveAttribute("href", "/login/oauth/facebook");
  });
});
