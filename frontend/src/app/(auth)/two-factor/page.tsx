import type { Metadata } from "next";
import { LoginView } from "@/features/auth/components/login-view";
import { TwoFactorForm } from "@/features/auth/components/two-factor-form";
import { securityReturnTo } from "@/features/auth/utils/auth-navigation";

export const metadata: Metadata = {
  title: "Verify sign-in — RoleCue",
  description: "Complete your account's two-factor verification.",
};

export default async function TwoFactorPage({
  searchParams,
}: PageProps<"/two-factor">) {
  const { returnTo } = await searchParams;
  return (
    <LoginView>
      <TwoFactorForm returnTo={securityReturnTo(returnTo)} />
    </LoginView>
  );
}
