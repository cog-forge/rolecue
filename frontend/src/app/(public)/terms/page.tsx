import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { RoutePlaceholder } from "@/components/feedback/route-placeholder";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("public.terms");
  return { title: t("title"), description: t("metadataDescription") };
}

export default async function TermsPage() {
  const t = await getTranslations("public.terms");
  return <RoutePlaceholder title={t("title")} description={t("description")} />;
}
