import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginForm } from "@/features/auth/components/login-form";

const {
  replace,
  emailSignIn,
  socialSignIn,
  sendVerificationEmail,
  toastError,
  toastSuccess,
} = vi.hoisted(() => ({
  replace: vi.fn(),
  emailSignIn: vi.fn(),
  socialSignIn: vi.fn(),
  sendVerificationEmail: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("@/lib/auth/client", () => ({
  authClient: {
    signIn: { email: emailSignIn, social: socialSignIn },
    sendVerificationEmail,
  },
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("goey-toast", () => ({
  gooeyToast: { error: toastError, success: toastSuccess },
}));
const configuredProviders = ["google", "github", "facebook"] as const;

describe("LoginForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    emailSignIn.mockResolvedValue({ data: {}, error: null });
    socialSignIn.mockResolvedValue({ data: {}, error: null });
    sendVerificationEmail.mockResolvedValue({ data: {}, error: null });
  });

  it("signs in with Better Auth and continues to the dashboard", async () => {
    render(<LoginForm enabledProviders={configuredProviders} />);
    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "candidate@example.com" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "passphrase" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() =>
      expect(emailSignIn).toHaveBeenCalledWith({
        email: "candidate@example.com",
        password: "passphrase",
        callbackURL: "/dashboard?auth=signed-in",
      }),
    );
    expect(replace).toHaveBeenCalledWith("/dashboard?auth=signed-in");
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("shows Better Auth errors without redirecting", async () => {
    emailSignIn.mockResolvedValue({
      data: null,
      error: {
        code: "INVALID_EMAIL_OR_PASSWORD",
        message: "Invalid credentials.",
      },
    });
    render(<LoginForm enabledProviders={configuredProviders} />);
    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "candidate@example.com" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "wrong-password" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Sign-in failed", {
        description: "Email or password is incorrect.",
      }),
    );
    expect(replace).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("offers verification resend after an unverified email cannot sign in", async () => {
    emailSignIn.mockResolvedValue({
      data: null,
      error: { message: "Email not verified" },
    });
    render(<LoginForm enabledProviders={configuredProviders} />);
    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "candidate@example.com" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "passphrase" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    fireEvent.click(
      await screen.findByRole("button", { name: "Resend verification email" }),
    );
    await waitFor(() =>
      expect(sendVerificationEmail).toHaveBeenCalledWith({
        email: "candidate@example.com",
        callbackURL: "/dashboard?auth=email-verified",
      }),
    );
  });

  it("explains how to recover when a social account is not linked", () => {
    render(
      <LoginForm
        enabledProviders={configuredProviders}
        oauthFailed
        oauthError="account_not_linked"
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "This social account is not linked to RoleCue. Sign in with your original method first.",
    );
  });

  it.each([
    ["Google", "google"],
    ["GitHub", "github"],
    ["Facebook", "facebook"],
  ])("starts %s OAuth through Better Auth", async (name, provider) => {
    render(<LoginForm enabledProviders={configuredProviders} />);
    fireEvent.click(
      screen.getByRole("button", { name: `Continue with ${name}` }),
    );
    await waitFor(() =>
      expect(socialSignIn).toHaveBeenCalledWith({
        provider,
        callbackURL: "/dashboard?auth=signed-in",
        errorCallbackURL: "/login?oauth=failed",
      }),
    );
  });

  it("hides social sign-in options when provider credentials are missing", () => {
    render(<LoginForm enabledProviders={[]} />);
    expect(
      screen.queryByRole("button", { name: "Continue with Google" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Continue with GitHub" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Continue with Facebook" }),
    ).not.toBeInTheDocument();
  });
});
