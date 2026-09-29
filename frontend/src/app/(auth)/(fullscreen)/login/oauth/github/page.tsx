import type { Metadata } from "next";
import { LoginView } from "@/features/auth/components/login-view";
import { OAuthPreviewCard } from "@/features/auth/components/oauth-preview-card";

export const metadata: Metadata = {
  title: "Continue with GitHub — RoleCue",
  description:
    "Preview the GitHub sign-in handoff for RoleCue. OAuth is not connected yet.",
};

export default function GitHubOAuthPage() {
  return (
    <LoginView>
      <OAuthPreviewCard provider="github" />
    </LoginView>
  );
}
