"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useEffect } from "react";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useSearchParams } from "next/navigation";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/layout/page-header";
import { routes } from "@/config/routes";
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
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <PageHeader
        title={t("title")}
        description={t("description")}
        meta={
          overview ? (
            <span>
              {t("updated", {
                days: overview.window.days,
                updated: fmt.date(overview.generated_at, "localDateTime"),
              })}
            </span>
          ) : undefined
        }
      />

      <WindowSelector days={days} />

      {query.isPending ? (
        <div role="status" className="space-y-4">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-40 rounded-xl" />
          ))}
          <span className="sr-only">{t("loading")}</span>
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

      <nav aria-label={t("tools.label")} className="rounded-xl border p-5">
        <h2 className="font-semibold">{t("tools.title")}</h2>
        <ul className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {quickLinks.map((link) => (
            <li key={link.href}>
              <Link
                href={link.href}
                className="inline-flex min-h-11 items-center rounded-md px-2 text-sm font-medium underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                {t(`tools.${link.key}`)}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </section>
  );
}

const quickLinks = [
  { href: routes.admin.jobPostings, key: "jobPostings" },
  { href: routes.admin.interviews, key: "interviews" },
  { href: routes.admin.voices, key: "voices" },
  { href: routes.admin.billing, key: "billing" },
  { href: routes.admin.revenue, key: "revenue" },
  { href: routes.admin.users, key: "users" },
] as const;
