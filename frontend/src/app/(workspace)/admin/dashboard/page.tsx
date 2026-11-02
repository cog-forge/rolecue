import { Suspense } from "react";
import { getTranslations } from "next-intl/server";
import { AdminOverview } from "@/features/admin/overview/components/admin-overview";

export default async function Page() {
  const t = await getTranslations("admin.overview");
  return (
    <Suspense
      fallback={
        <p role="status" className="text-sm text-muted-foreground">
          {t("loading")}
        </p>
      }
    >
      <AdminOverview />
    </Suspense>
  );
}
