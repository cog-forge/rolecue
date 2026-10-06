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
  const { oauth, error } = await searchParams;
  const enabledProviders = (
    [
      ["google", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"],
      ["github", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET"],
      ["facebook", "FACEBOOK_CLIENT_ID", "FACEBOOK_CLIENT_SECRET"],
    ] as const
  )
    .filter(([, id, secret]) => process.env[id] && process.env[secret])
    .map(([provider]) => provider) as SocialProvider[];
  return (
    <LoginView>
      <LoginForm
        oauthFailed={oauth === "failed"}
        oauthError={typeof error === "string" ? error : undefined}
        enabledProviders={enabledProviders}
      />
    </LoginView>
  );
}
