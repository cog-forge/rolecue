import type { Metadata } from "next";
import { LoginView } from "@/features/auth/components/login-view";
import { LoginForm } from "@/features/auth/components/login-form";
import { securityReturnTo } from "@/features/auth/utils/auth-navigation";
import { enabledSocialProviders } from "@/features/auth/utils/social-providers";

export const metadata: Metadata = {
  title: "Log in — RoleCue",
  description:
    "Sign in to your RoleCue account to practice technical interviews.",
};

export default async function LoginPage({ searchParams }: PageProps<"/login">) {
  const { oauth, error, session, returnTo } = await searchParams;
  return (
    <LoginView>
      <LoginForm
        oauthFailed={oauth === "failed"}
        oauthError={typeof error === "string" ? error : undefined}
        sessionEnded={session === "ended"}
        returnTo={securityReturnTo(returnTo)}
        enabledProviders={enabledSocialProviders(
          process.env.AUTH_SOCIAL_PROVIDERS,
        )}
      />
    </LoginView>
  );
}
