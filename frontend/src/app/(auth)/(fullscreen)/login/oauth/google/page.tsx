import type { Metadata } from "next";
import { LoginView } from "@/features/auth/components/login-view";
import { OAuthPreviewCard } from "@/features/auth/components/oauth-preview-card";

export const metadata: Metadata = {
  title: "Continue with Google — RoleCue",
  description:
    "Preview the Google sign-in handoff for RoleCue. OAuth is not connected yet.",
};

export default function GoogleOAuthPage() {
  return (
    <LoginView>
      <OAuthPreviewCard provider="google" />
    </LoginView>
  );
}
