"use client";

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
import {
  formatInterviewTime,
  reportStatusLabel,
  reportTypeLabel,
  scoreText,
} from "@/features/report/utils/report-text";
import { historyKeys, listInterviewHistory } from "../api/history";
import { HISTORY_PAGE_SIZE, type HistoryItem } from "../schemas/history-schema";
import { isAccessDenied } from "../utils/api-status";
import {
  historyHref,
  historyTypeFilters,
  historyTypeLabel,
  parseHistoryFilters,
  type HistoryFilters,
  type HistoryTypeFilter,
} from "../utils/history-params";

// The Candidate's own interviews, newest first. Type and page come from the URL, so reload and
// back keep the view.
export function InterviewHistory() {
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
          Practice
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Interview history
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Your past interviews, newest first. Open one to see its report.
        </p>
      </header>

      <TypeFilter filters={filters} />

      {query.isPending ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-20 rounded-xl" />
          ))}
          <span className="sr-only">Loading your interview history…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border bg-muted/60 p-6">
          <h2 className="font-semibold">
            You don&apos;t have access to interview history
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            candidate account.
          </p>
        </div>
      ) : query.isError ? (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <h2 className="font-semibold">
            We couldn&apos;t load your interview history
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please try again in a moment.
          </p>
          <Button
            className="mt-4 h-11 bg-background"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </div>
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
  const current: HistoryTypeFilter = filters.type ?? "all";
  return (
    <nav aria-label="Filter interviews by type">
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
                {historyTypeLabel[type]}
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
  const page = filters.page;
  const firstRow = total === 0 ? 0 : (page - 1) * HISTORY_PAGE_SIZE + 1;
  const lastRow = Math.min(page * HISTORY_PAGE_SIZE, total);

  if (items.length === 0 && page > 1)
    return (
      <div className="rounded-xl border p-8 text-center">
        <h2 className="font-semibold">
          This page is past the end of your history.
        </h2>
        <Button asChild variant="outline" className="mt-4 h-11">
          <Link href={historyHref({ type: filters.type, page: 1 })}>
            Go to the first page
          </Link>
        </Button>
      </div>
    );

  if (items.length === 0)
    return (
      <div className="rounded-2xl border border-dashed bg-muted/30 p-10 text-center">
        <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-workspace-accent/10 text-workspace-accent">
          <Inbox className="size-6" aria-hidden />
        </span>
        <h2 className="font-semibold">
          {filters.type
            ? `You have no ${historyTypeLabel[filters.type].toLowerCase()} interviews yet.`
            : "You have no interviews yet."}
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Your practice and recruitment interviews will be listed here.
        </p>
        <Button asChild variant="outline" className="mt-4 h-11">
          <Link href={routes.dashboard}>Go to the dashboard</Link>
        </Button>
      </div>
    );

  return (
    <div className="space-y-4">
      <ul
        aria-label="Your interviews"
        className="divide-y rounded-2xl border bg-card"
      >
        {items.map((item) => (
          <HistoryRow key={item.id} item={item} />
        ))}
      </ul>
      <nav
        aria-label="Interview history pagination"
        className="flex flex-wrap items-center justify-between gap-3 text-sm"
      >
        <p className="text-muted-foreground tabular-nums">
          Showing {firstRow}–{lastRow} of {total}
        </p>
        <div className="flex gap-2">
          <PageButton
            label="Previous"
            href={historyHref({ type: filters.type, page: page - 1 })}
            disabled={page <= 1}
          />
          <PageButton
            label="Next"
            href={historyHref({ type: filters.type, page: page + 1 })}
            disabled={lastRow >= total}
          />
        </div>
      </nav>
    </div>
  );
}

function HistoryRow({ item }: { item: HistoryItem }) {
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
          <Badge variant="outline">{reportTypeLabel[item.type]}</Badge>
          <time dateTime={item.started_at}>
            {formatInterviewTime(item.started_at)}
          </time>
        </p>
        {item.application_id ? (
          <Link
            href={routes.application(item.application_id)}
            className="inline-flex min-h-11 items-center rounded-sm text-sm font-medium text-workspace-accent underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            View application
          </Link>
        ) : null}
      </div>
      <div className="space-y-1 text-right text-sm">
        <p className="font-medium">{reportStatusLabel[item.status]}</p>
        <p className="text-muted-foreground tabular-nums">
          {scoreText(item.score)}
        </p>
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
