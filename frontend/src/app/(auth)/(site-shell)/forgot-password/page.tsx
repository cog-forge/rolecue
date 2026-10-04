import type { Metadata } from "next";
import { ForgotPasswordForm } from "@/features/auth/components/forgot-password-form";
import { LoginView } from "@/features/auth/components/login-view";

export const metadata: Metadata = {
  title: "Reset your password — RoleCue",
  description: "Request a secure password reset link for your RoleCue account.",
};

export default function ForgotPasswordPage() {
  return (
    <LoginView>
      <ForgotPasswordForm />
    </LoginView>
  );
}
