"use client";

import Link from "next/link";
import { skipToken, useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ArrowUpRight, Target } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Progress } from "@/components/ui/progress";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getInterviewReport, reportKeys } from "@/features/report/api/report";
import { formatScore } from "@/features/report/utils/scoring";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { PracticeQuery } from "../api/dashboard";
import { recentPractice } from "../utils/practice-summary";
import { DashboardCard, CardError, CardLoading } from "./dashboard-card";

export function PracticeFocusCard({
  query,
  className,
}: {
  query: PracticeQuery;
  className?: string;
}) {
  const t = useTranslations("candidate.dashboard.focus");
  const user = useSessionUser();
  const fmt = useFormatters();
  const latest = recentPractice(query.data?.items ?? []).find(
    (item) => item.status === "completed",
  );
  const report = useQuery({
    queryKey: reportKeys.detail(user.id, latest?.id ?? "none"),
    queryFn: latest
      ? ({ signal }) => getInterviewReport(latest.id, signal)
      : skipToken,
    retry: false,
  });
  const metrics =
    report.data?.type === "practice"
      ? [...(report.data.breakdown ?? [])]
          .filter(
            (metric) =>
              metric.weight === null ||
              metric.weight === undefined ||
              metric.weight > 0,
          )
          .filter((metric) => metric.score !== null)
          .sort((a, b) => (a.score ?? 0) - (b.score ?? 0))
          .slice(0, 3)
      : [];
  const pending = query.isPending || (Boolean(latest) && report.isPending);
  return (
    <DashboardCard isLoading={pending} title={t("title")} className={className}>
      {pending ? (
        <CardLoading label={t("loading")} />
      ) : query.isError || report.isError ? (
        <CardError
          message={t("error")}
          disabled={query.isFetching || report.isFetching}
          onRetry={() =>
            void (query.isError ? query.refetch() : report.refetch())
          }
        />
      ) : !metrics.length ? (
        <EmptyState
          variant="plain"
          icons={[Target]}
          title={t(latest ? "unassessedTitle" : "emptyTitle")}
          description={t(latest ? "unassessed" : "empty")}
          action={
            latest
              ? {
                  label: t("view"),
                  href: routes.reports.detail(latest.id),
                  variant: "outline",
                }
              : undefined
          }
          className="min-h-40 px-0! py-2!"
        />
      ) : (
        <>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {t("description")}
          </p>
          <ul className="space-y-5">
            {metrics.map((metric) => (
              <li key={metric.metric_id} className="space-y-2">
                <div className="flex items-start justify-between gap-4 text-sm">
                  <span className="min-w-0 font-medium break-words">
                    {metric.name}
                  </span>
                  <span className="shrink-0 font-semibold tabular-nums">
                    {formatScore(metric.score ?? 0, fmt.number)}
                    <span className="font-normal text-muted-foreground">
                      {" "}
                      / 100
                    </span>
                  </span>
                </div>
                <Progress
                  value={metric.score}
                  aria-label={metric.name}
                  className="h-1.5 bg-muted [&_[data-slot=progress-indicator]]:bg-rolecue-brand/60"
                />
              </li>
            ))}
          </ul>
          <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
            <p className="max-w-56 text-xs leading-relaxed text-muted-foreground">
              {latest?.role_profile_title} ·{" "}
              {latest && fmt.date(latest.started_at, "calendarDay")}
            </p>
            <Button asChild variant="ghost" className="h-11 px-0">
              <Link href={routes.reports.detail(latest!.id)}>
                {t("view")}
                <ArrowUpRight aria-hidden className="size-4" />
              </Link>
            </Button>
          </div>
        </>
      )}
    </DashboardCard>
  );
}
