import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { LoginView } from "@/features/auth/components/login-view";
import { TwoFactorForm } from "@/features/auth/components/two-factor-form";
import { securityReturnTo } from "@/features/auth/utils/auth-navigation";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth.twoFactor");
  return { title: t("metadataTitle"), description: t("metadataDescription") };
}

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
