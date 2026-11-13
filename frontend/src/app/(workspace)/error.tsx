"use client";

import { useTranslations } from "next-intl";
import { ErrorState } from "@/components/ui/error-state";
import { routes } from "@/config/routes";

export default function WorkspaceError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("common.workspaceError");

  return (
    <div className="flex min-h-[60vh] w-full items-center justify-center p-4">
      <ErrorState
        variant="card"
        title={t("title")}
        error={error}
        onRetry={reset}
        homeHref={routes.home}
        className="max-w-xl"
      />
    </div>
  );
}
