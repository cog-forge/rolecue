import type { Metadata } from "next";
import { ResetPasswordForm } from "@/features/auth/components/reset-password-form";
import { LoginView } from "@/features/auth/components/login-view";

export const metadata: Metadata = {
  title: "Reset your password — RoleCue",
  description: "Set a new password for your RoleCue account.",
};

export default async function ResetPasswordPage({
  searchParams,
}: PageProps<"/reset-password">) {
  const { token } = await searchParams;
  return (
    <LoginView>
      <ResetPasswordForm
        token={typeof token === "string" ? token : undefined}
      />
    </LoginView>
  );
}
