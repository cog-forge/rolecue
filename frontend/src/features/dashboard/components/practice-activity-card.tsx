"use client";

import { useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import type { PracticeQuery } from "../api/dashboard";
import type { InterviewHistoryItem } from "../schemas/dashboard-schema";
import { recentPractice } from "../utils/practice-summary";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { formatScore } from "@/features/report/utils/scoring";

export function PracticeActivityCard({
  query,
  className,
}: {
  query: PracticeQuery;
  className?: string;
}) {
  const t = useTranslations("candidate.dashboard.practice");
  return (
    <DashboardCard
      isLoading={query.isPending}
      title={t("title")}
      className={className}
      action={
        <Button asChild variant="ghost" size="sm" className="min-h-11 px-2">
          <Link href={routes.history}>
            {t("view")}
            <ArrowUpRight aria-hidden className="size-4" />
          </Link>
        </Button>
      }
    >
      {query.isPending ? (
        <CardLoading label={t("loading")} />
      ) : query.isError ? (
        <CardError
          message={t("error")}
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : query.data.total === 0 ? (
        <EmptyState
          variant="plain"
          title={t("emptyTitle")}
          description={t("empty")}
        />
      ) : (
        <>
          <p className="text-sm text-muted-foreground">
            {t("count", {
              count: query.data.total,
              countText: String(query.data.total),
            })}{" "}
            · {t("recent")}
          </p>
          <ul className="divide-y">
            {recentPractice(query.data.items)
              .slice(0, 3)
              .map((item) => (
                <PracticeRow key={item.id} item={item} />
              ))}
          </ul>
        </>
      )}
    </DashboardCard>
  );
}

function PracticeRow({ item }: { item: InterviewHistoryItem }) {
  const t = useTranslations("candidate.dashboard.practice");
  const fmt = useFormatters();
  return (
    <li>
      <Link
        href={routes.reports.detail(item.id)}
        className="group flex min-h-20 items-center justify-between gap-4 rounded-md py-3 outline-none transition-colors hover:bg-muted/40 focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <div className="min-w-0 space-y-1">
          <p className="truncate text-sm font-medium group-hover:underline underline-offset-4">
            {item.role_profile_title}
          </p>
          <p className="text-xs text-muted-foreground">
            {fmt.date(item.started_at, "calendarDay")} ·{" "}
            {t(`status.${item.status}`)}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          {item.score !== null ? (
            <span className="text-sm font-semibold tabular-nums">
              {t("scoreValue", { score: formatScore(item.score, fmt.number) })}
              {item.score_outcome === "partial" && (
                <span className="block text-right text-xs font-normal text-muted-foreground">
                  {t("partial")}
                </span>
              )}
            </span>
          ) : (
            <Badge
              variant="secondary"
              className="max-w-28 whitespace-normal text-right"
            >
              {item.score_outcome === "withheld"
                ? t("scoreWithheld")
                : t(`status.${item.status}`)}
            </Badge>
          )}
          <ArrowUpRight aria-hidden className="size-4 text-muted-foreground" />
        </div>
      </Link>
    </li>
  );
}
