"use client";
import { EmptyState } from "@/components/ui/empty-state";

import { ErrorState } from "@/components/ui/error-state";

import { useTranslations } from "next-intl";

import { useEffect } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Inbox } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { formatInterviewTime } from "@/features/report/utils/report-text";
import { formatScore } from "@/features/report/utils/scoring";
import { historyKeys, listInterviewHistory } from "../api/history";
import { HISTORY_PAGE_SIZE, type HistoryItem } from "../schemas/history-schema";
import { isAccessDenied } from "../utils/api-status";
import {
  historyHref,
  historyTypeFilters,
  parseHistoryFilters,
  type HistoryFilters,
  type HistoryTypeFilter,
} from "../utils/history-params";
import { useFormatters } from "@/lib/i18n/use-formatters";

// The Candidate's own interviews, newest first. Type and page come from the URL, so reload and
// back keep the view.
export function InterviewHistory() {
  const t = useTranslations("candidate.history");
  const user = useSessionUser();
  const cache = useQueryClient();
  const params = useSearchParams();
  const filters = parseHistoryFilters(params);

  const query = useQuery({
    queryKey: historyKeys.list(user.id, filters),
    queryFn: ({ signal }) => listInterviewHistory(filters, signal),
    refetchOnMount: "always",
    retry: false,
    placeholderData: keepPreviousData,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  return (
    <section className="mx-auto w-full max-w-4xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("heading")}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {t("description")}
        </p>
      </header>

      <TypeFilter filters={filters} />

      {query.isPending || query.isPlaceholderData ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-20 rounded-xl" />
          ))}
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <ErrorState
          title={t("denied")}
          description={t("checking")}
        ></ErrorState>
      ) : query.isError ? (
        <ErrorState
          title={t("error")}
          description={t("tryLater")}
          onRetry={() => void query.refetch()}
          retryLabel={t("retry")}
          isRetrying={query.isFetching}
        ></ErrorState>
      ) : (
        <HistoryResults
          filters={filters}
          items={query.data.items}
          total={query.data.total}
        />
      )}
    </section>
  );
}

function TypeFilter({ filters }: { filters: HistoryFilters }) {
  const t = useTranslations("candidate.history");
  const current: HistoryTypeFilter = filters.type ?? "all";
  return (
    <nav aria-label={t("filter")}>
      <ul className="flex flex-wrap gap-2">
        {historyTypeFilters.map((type) => {
          const active = type === current;
          return (
            <li key={type}>
              <Link
                href={historyHref({
                  type: type === "all" ? undefined : type,
                  page: 1,
                })}
                scroll={false}
                aria-current={active ? "page" : undefined}
                className={
                  active
                    ? "inline-flex min-h-11 items-center rounded-full border border-workspace-accent bg-workspace-accent/10 px-4 text-sm font-medium text-workspace-accent"
                    : "inline-flex min-h-11 items-center rounded-full border px-4 text-sm font-medium text-muted-foreground hover:text-foreground"
                }
              >
                {t(`types.${type}`)}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

function HistoryResults({
  filters,
  items,
  total,
}: {
  filters: HistoryFilters;
  items: HistoryItem[];
  total: number;
}) {
  const t = useTranslations("candidate.history");
  const page = filters.page;
  const firstRow = total === 0 ? 0 : (page - 1) * HISTORY_PAGE_SIZE + 1;
  const lastRow = Math.min(page * HISTORY_PAGE_SIZE, total);

  if (items.length === 0 && page > 1)
    return (
      <EmptyState title={t("pastEnd")} description="">
        <Button asChild variant="outline" className="mt-4 h-11">
          <Link href={historyHref({ type: filters.type, page: 1 })}>
            {t("firstPage")}
          </Link>
        </Button>
      </EmptyState>
    );

  if (items.length === 0)
    return (
      <EmptyState
        icons={[Inbox]}
        title={
          filters.type === "practice"
            ? t("emptyPractice")
            : filters.type === "recruitment"
              ? t("emptyRecruitment")
              : t("empty")
        }
        description={t("emptyDescription")}
      >
        <Button asChild variant="outline" className="mt-4 h-11">
          <Link href={routes.dashboard}>{t("dashboard")}</Link>
        </Button>
      </EmptyState>
    );

  return (
    <div className="space-y-4">
      <ul
        aria-label={t("list")}
        className="divide-y rounded-2xl border bg-card"
      >
        {items.map((item) => (
          <HistoryRow key={item.id} item={item} />
        ))}
      </ul>
      <nav
        aria-label={t("pagination")}
        className="flex flex-wrap items-center justify-between gap-3 text-sm"
      >
        <p className="text-muted-foreground tabular-nums">
          {t("showing", {
            first: String(firstRow),
            last: String(lastRow),
            total: String(total),
          })}
        </p>
        <div className="flex gap-2">
          <PageButton
            label={t("previous")}
            href={historyHref({ type: filters.type, page: page - 1 })}
            disabled={page <= 1}
          />
          <PageButton
            label={t("next")}
            href={historyHref({ type: filters.type, page: page + 1 })}
            disabled={lastRow >= total}
          />
        </div>
      </nav>
    </div>
  );
}

function HistoryRow({ item }: { item: HistoryItem }) {
  const t = useTranslations("candidate.history");
  const fmt = useFormatters();
  const title = item.job_posting_title ?? item.role_profile_title;
  return (
    <li className="flex flex-wrap items-start justify-between gap-3 p-4 sm:px-5">
      <div className="min-w-0 space-y-1">
        <Link
          href={routes.reports.detail(item.id)}
          className="rounded-sm font-medium underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {title}
        </Link>
        <p className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground tabular-nums">
          <Badge variant="outline">{t(`reportType.${item.type}`)}</Badge>
          <time dateTime={item.started_at}>
            {formatInterviewTime(item.started_at, fmt)}
          </time>
        </p>
        {item.application_id ? (
          <Link
            href={routes.application(item.application_id)}
            className="inline-flex min-h-11 items-center rounded-sm text-sm font-medium text-workspace-accent underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {t("application")}
          </Link>
        ) : null}
      </div>
      <div className="space-y-1 text-right text-sm">
        <p className="font-medium">{t(`status.${item.status}`)}</p>
        {item.type === "practice" && (
          <p className="text-muted-foreground tabular-nums">
            {item.score !== null
              ? t("scoreValue", { score: formatScore(item.score, fmt.number) })
              : item.score_outcome === "withheld"
                ? t("scoreWithheld")
                : t("noScore")}
          </p>
        )}
      </div>
    </li>
  );
}

function PageButton({
  label,
  href,
  disabled,
}: {
  label: string;
  href: string;
  disabled: boolean;
}) {
  if (disabled)
    return (
      <Button variant="outline" className="h-11" disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className="h-11">
      <Link href={href} scroll={false}>
        {label}
      </Link>
    </Button>
  );
}
