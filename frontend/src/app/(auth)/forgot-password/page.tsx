import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { ForgotPasswordForm } from "@/features/auth/components/forgot-password-form";
import { LoginView } from "@/features/auth/components/login-view";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth.forgotPassword");
  return { title: t("metadataTitle"), description: t("metadataDescription") };
}

export default function ForgotPasswordPage() {
  return (
    <LoginView>
      <ForgotPasswordForm />
    </LoginView>
  );
}
