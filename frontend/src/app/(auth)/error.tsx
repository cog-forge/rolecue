"use client";

import { useTranslations } from "next-intl";
import { ErrorState } from "@/components/ui/error-state";
import { routes } from "@/config/routes";

export default function AuthError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("common.globalError");

  return (
    <div className="flex min-h-[50vh] w-full items-center justify-center p-4">
      <ErrorState
        variant="card"
        title={t("title")}
        description={t("description")}
        retryLabel={t("retry")}
        error={error}
        onRetry={reset}
        homeHref={routes.home}
        className="max-w-md"
      />
    </div>
  );
}
