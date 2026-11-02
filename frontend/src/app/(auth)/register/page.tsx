import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { LoginView } from "@/features/auth/components/login-view";
import { RegisterForm } from "@/features/auth/components/register-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth.register");
  return { title: t("metadataTitle"), description: t("metadataDescription") };
}

export default function RegisterPage() {
  return (
    <LoginView>
      <RegisterForm />
    </LoginView>
  );
}
