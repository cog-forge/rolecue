import { useTranslations } from "next-intl";
import { Skeleton } from "@/components/ui/skeleton";
export default function Loading() {
  const t = useTranslations("common");
  return (
    <div role="status" className="space-y-4">
      <span className="sr-only">{t("loadingPage")}</span>
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-32 w-full" />
    </div>
  );
}
