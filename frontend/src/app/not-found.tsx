import Link from "next/link";
import { useTranslations } from "next-intl";
import { routes } from "@/config/routes";
export default function NotFound() {
  const t = useTranslations("common.notFound");
  return (
    <main className="mx-auto max-w-xl space-y-4 p-12">
      <h1 className="text-2xl font-semibold">{t("title")}</h1>
      <Link href={routes.home} className="underline">
        {t("home")}
      </Link>
    </main>
  );
}
