import { getTranslations } from "next-intl/server";
import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
import Link from "next/link";
import { routes } from "@/config/routes";
export default async function Page() {
  const t = await getTranslations("candidate.setup.index");
  return (
    <RoutePlaceholder title={t("title")} description={t("description")}>
      <Link className="underline" href={routes.interviews.new.jobDescription}>
        {t("create")}
      </Link>
    </RoutePlaceholder>
  );
}
