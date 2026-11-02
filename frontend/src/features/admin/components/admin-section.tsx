import { useTranslations } from "next-intl";
import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
export function AdminSection({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  const t = useTranslations("admin.shared");
  return (
    <RoutePlaceholder title={title} description={description}>
      <p className="text-sm text-muted-foreground">{t("sectionPreparing")}</p>
    </RoutePlaceholder>
  );
}
