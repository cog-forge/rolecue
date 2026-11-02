import { getTranslations } from "next-intl/server";
import { RoutePlaceholder } from "@/components/feedback/route-placeholder";

export default async function Page() {
  const t = await getTranslations("public.pricing");
  return (
    <RoutePlaceholder
      title={t("title")}
      description={t("description")}
    ></RoutePlaceholder>
  );
}
