"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useEffect } from "react";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { useSearchParams } from "next/navigation";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/layout/page-header";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { cn } from "@/lib/utils";
import { getOverview, isAccessDenied, overviewKeys } from "../api/overview";
import { parseOverviewDays } from "../utils/overview-days";
import { OverviewBody } from "./overview-body";
import { WindowSelector } from "./window-selector";

// The reporting window lives in the URL (?days=) so reload, back and sharing keep it.
export function AdminOverview() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const fmt = useFormatters();
  const t = useTranslations("admin.overview");
  const tCommon = useTranslations("common");
  const days = parseOverviewDays(useSearchParams().get("days"));
  const query = useQuery({
    queryKey: overviewKeys.overview(user.id, days),
    queryFn: ({ signal }) => getOverview(days, signal),
    retry: false,
    // Keep the previous window on screen while the next one loads.
    placeholderData: keepPreviousData,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const overview = query.data;

  return (
    <section className="mx-auto w-full max-w-7xl space-y-6">
      <PageHeader
        title={t("title")}
        actions={<WindowSelector days={days} />}
        meta={
          overview ? (
            <span>
              {t("updated", {
                days: overview.window.days,
                updated: fmt.date(overview.generated_at, "vietnamDateTime"),
              })}
            </span>
          ) : undefined
        }
      />

      {query.isPending ? (
        <div role="status" className="space-y-5">
          <span className="sr-only">{t("loading")}</span>
          <div aria-hidden className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-28 rounded-xl" />
            ))}
          </div>
          <div aria-hidden className="grid gap-4 xl:grid-cols-12">
            <Skeleton className="h-52 rounded-xl xl:order-last xl:h-100 xl:col-span-4" />
            <Skeleton className="h-86 rounded-xl xl:h-100 xl:col-span-8" />
          </div>
        </div>
      ) : denied ? (
        <ErrorState
          title={t("denied.title")}
          description={t("denied.description")}
        ></ErrorState>
      ) : query.isError || !overview ? (
        <ErrorState
          title={t("error.title")}
          description={tCommon("messages.tryAgainLater")}
          onRetry={() => void query.refetch()}
          retryLabel={tCommon("actions.tryAgain")}
          isRetrying={query.isFetching}
        ></ErrorState>
      ) : (
        <div
          aria-busy={query.isPlaceholderData}
          className={cn(
            "transition-opacity duration-200 motion-reduce:transition-none",
            // The numbers still belong to the previous window until the new one arrives.
            query.isPlaceholderData && "opacity-60",
          )}
        >
          <p role="status" className="sr-only">
            {query.isPlaceholderData ? t("loadingWindow", { days }) : ""}
          </p>
          <OverviewBody overview={overview} />
        </div>
      )}
    </section>
  );
}
