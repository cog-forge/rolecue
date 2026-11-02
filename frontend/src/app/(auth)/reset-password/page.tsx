import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { ResetPasswordForm } from "@/features/auth/components/reset-password-form";
import { LoginView } from "@/features/auth/components/login-view";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth.resetPassword");
  return { title: t("metadataTitle"), description: t("metadataDescription") };
}

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
