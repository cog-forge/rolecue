import { getTranslations } from "next-intl/server";
import { SiteShell } from "@/components/layout/site-shell";
import { routes } from "@/config/routes";

export default async function PricingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const t = await getTranslations("public.pricing");
  const navigation = [
    { label: t("nav.home"), href: routes.home },
    { label: t("nav.pricing"), href: routes.pricing },
    { label: t("nav.login"), href: routes.login },
  ];
  return (
    <SiteShell navigation={navigation} label={t("shellLabel")}>
      {children}
    </SiteShell>
  );
}
