import type { Metadata } from "next";
import { LoginView } from "@/features/auth/components/login-view";
import { LoginForm } from "@/features/auth/components/login-form";
import type { SocialProvider } from "@/features/auth/components/login-form";

export const metadata: Metadata = {
  title: "Log in — RoleCue",
  description:
    "Sign in to your RoleCue account to practice technical interviews.",
};

export default async function LoginPage({ searchParams }: PageProps<"/login">) {
  const { oauth, error, session } = await searchParams;
  // Non-secret display allowlist; provider credentials live only in the auth service.
  const configured = new Set(
    (process.env.AUTH_SOCIAL_PROVIDERS || "").split(","),
  );
  const enabledProviders = (["google", "github", "facebook"] as const).filter(
    (provider) => configured.has(provider),
  ) as SocialProvider[];
  return (
    <LoginView>
      <LoginForm
        oauthFailed={oauth === "failed"}
        oauthError={typeof error === "string" ? error : undefined}
        sessionEnded={session === "ended"}
        enabledProviders={enabledProviders}
      />
    </LoginView>
  );
}
