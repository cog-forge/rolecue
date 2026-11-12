import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { InformationPage } from "@/features/marketing/components/information-page";
export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("public.pages");
  return { title: t("privacy"), description: t("privacyIntro") };
}
export default function Page() {
  return <InformationPage kind="privacy" />;
}
