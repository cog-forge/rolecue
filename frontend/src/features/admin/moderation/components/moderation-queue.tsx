"use client";
import { EmptyState } from "@/components/ui/empty-state";

import { ErrorState } from "@/components/ui/error-state";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronRight, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { cn } from "@/lib/utils";
import { listPostings, moderationKeys } from "../api/moderation";
import type { PostingSummary } from "../schemas/moderation-schema";
import { isAccessDenied } from "../utils/api-error";
import {
  QUEUE_PAGE_SIZE,
  queueSearch,
  type QueueFilters,
  type QueueStatus,
} from "../utils/queue-filters";
import { PageHeader } from "@/components/layout/page-header";
import { PostingStatus } from "./posting-status";
import { useFormatters } from "@/lib/i18n/use-formatters";

const tabs = [
  "pending",
  "open",
  "rejected",
  "all",
] as const satisfies readonly QueueStatus[];
const queueHref = (filters: QueueFilters) =>
  `${routes.admin.jobPostings}${queueSearch(filters)}`;

export function ModerationQueue({ filters }: { filters: QueueFilters }) {
  const t = useTranslations("admin.moderation.queue");
  const tCommon = useTranslations("common");
  const user = useSessionUser();
  const cache = useQueryClient();
  const query = useQuery({
    queryKey: moderationKeys.list(user.id, filters),
    queryFn: ({ signal }) => listPostings(filters, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const page = query.data;
  const total = page?.pagination.total ?? 0;
  const firstRow = total === 0 ? 0 : (filters.page - 1) * QUEUE_PAGE_SIZE + 1;
  const lastRow = Math.min(filters.page * QUEUE_PAGE_SIZE, total);
  const backQuery = queueSearch(filters).slice(1);

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <PageHeader title={t("title")} description={t("description")} />

      <div className="space-y-4">
        <div className="border-b">
          <nav
            aria-label={t("statusFilter")}
            className="-mb-px max-w-full overflow-x-auto"
          >
            <ul className="flex gap-5">
              {tabs.map((tab) => {
                const active = filters.status === tab;
                return (
                  <li key={tab}>
                    <Link
                      href={queueHref({
                        ...filters,
                        status: tab,
                        page: 1,
                      })}
                      scroll={false}
                      aria-current={active ? "page" : undefined}
                      className={cn(
                        "inline-flex min-h-11 items-center border-b-2 border-transparent text-sm whitespace-nowrap text-muted-foreground transition-colors duration-[var(--duration-quick)] outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
                        active &&
                          "border-foreground font-medium text-foreground",
                      )}
                    >
                      {tab === "all" ? tCommon("labels.all") : t(`tabs.${tab}`)}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </nav>
        </div>
        <div className="flex justify-end">
          <DateFilter key={`${filters.from}-${filters.to}`} filters={filters} />
        </div>
      </div>

      {filters.recruiterId && (
        <p className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-muted-foreground">{t("oneRecruiter")}</span>
          <Link
            href={queueHref({ ...filters, recruiterId: null, page: 1 })}
            scroll={false}
            className="inline-flex min-h-11 items-center gap-1 rounded-md px-2 font-medium underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <X className="size-4" aria-hidden />
            {t("showAllRecruiters")}
          </Link>
        </p>
      )}

      {query.isPending ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-14 rounded-md" />
          ))}
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <ErrorState
          title={t("denied.title")}
          description={t("denied.description")}
        ></ErrorState>
      ) : query.isError ? (
        <ErrorState
          title={t("loadFailed.title")}
          description={tCommon("messages.tryAgainLater")}
          onRetry={() => void query.refetch()}
          retryLabel={tCommon("actions.tryAgain")}
          isRetrying={query.isFetching}
        ></ErrorState>
      ) : page && page.items.length === 0 ? (
        <EmptyState
          title={t(`empty.${filters.status}`)}
          description={t("emptyHint")}
        ></EmptyState>
      ) : page ? (
        <>
          <div className="relative overflow-x-auto rounded-xl border">
            <table className="w-full min-w-[760px] text-left text-sm">
              <caption className="sr-only">
                {t("table.caption", {
                  first: firstRow,
                  last: lastRow,
                  total,
                })}
              </caption>
              <thead>
                <tr className="border-b text-xs text-muted-foreground">
                  <th scope="col" className="py-3 pr-4 pl-4 font-medium">
                    {t("table.posting")}
                  </th>
                  <th scope="col" className="py-3 pr-4 pl-4 font-medium">
                    {t("table.recruiter")}
                  </th>
                  <th scope="col" className="py-3 pr-4 pl-4 font-medium">
                    {t("table.created")}
                  </th>
                  <th scope="col" className="py-3 pr-4 pl-4 font-medium">
                    {t("table.status")}
                  </th>
                  <th scope="col" className="py-3 text-right font-medium">
                    <span className="sr-only">{t("table.action")}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {page.items.map((posting) => (
                  <QueueRow
                    key={posting.id}
                    posting={posting}
                    filters={filters}
                    backQuery={backQuery}
                  />
                ))}
              </tbody>
            </table>
          </div>
          <nav
            aria-label={tCommon("labels.pagination")}
            className="flex flex-wrap items-center justify-between gap-3 text-sm"
          >
            <p className="text-muted-foreground tabular-nums">
              {t("pagination.summary", {
                first: firstRow,
                last: lastRow,
                total,
              })}
            </p>
            <div className="flex gap-2">
              <PageLink
                label={tCommon("actions.previous")}
                href={queueHref({ ...filters, page: filters.page - 1 })}
                disabled={filters.page <= 1}
              />
              <PageLink
                label={tCommon("actions.next")}
                href={queueHref({ ...filters, page: filters.page + 1 })}
                disabled={lastRow >= total}
              />
            </div>
          </nav>
        </>
      ) : null}
    </section>
  );
}

function QueueRow({
  posting,
  filters,
  backQuery,
}: {
  posting: PostingSummary;
  filters: QueueFilters;
  backQuery: string;
}) {
  const t = useTranslations("admin.moderation.queue");
  const fmt = useFormatters();
  const created = new Date(posting.created_at);
  const reviewHref = `${routes.admin.jobPosting(posting.id)}${
    backQuery ? `?back=${encodeURIComponent(backQuery)}` : ""
  }`;
  return (
    <tr className="border-b align-middle last:border-b-0">
      <td className="py-3 pr-4 pl-4 font-medium [overflow-wrap:anywhere]">
        {posting.title}
      </td>
      <td className="py-3 pr-4">
        <Link
          href={queueHref({
            ...filters,
            recruiterId: posting.recruiter.id,
            page: 1,
          })}
          scroll={false}
          className="rounded-sm underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
          title={t("filterByRecruiter", { name: posting.recruiter.name })}
        >
          {posting.recruiter.name}
        </Link>
        <span className="block text-xs text-muted-foreground [overflow-wrap:anywhere]">
          {posting.recruiter.email}
        </span>
      </td>
      <td className="py-3 pr-4 whitespace-nowrap tabular-nums">
        <time dateTime={posting.created_at} title={posting.created_at}>
          {fmt.date(created, "calendarDay")}
        </time>
      </td>
      <td className="py-3 pr-4">
        <PostingStatus status={posting.status} />
      </td>
      <td className="py-3 pr-4 text-right">
        <Button
          asChild
          variant="outline"
          size="sm"
          className="h-10 rounded-full"
        >
          <Link
            href={reviewHref}
            aria-label={t("reviewLabel", { title: posting.title })}
          >
            {t("review")}
            <ChevronRight aria-hidden />
          </Link>
        </Button>
      </td>
    </tr>
  );
}

function PageLink({
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
      <Button variant="outline" className="h-10 rounded-full" disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className="h-10 rounded-full">
      <Link href={href} scroll={false}>
        {label}
      </Link>
    </Button>
  );
}

function DateFilter({ filters }: { filters: QueueFilters }) {
  const t = useTranslations("admin.moderation.dateFilter");
  const router = useRouter();
  const [from, setFrom] = useState(filters.from ?? "");
  const [to, setTo] = useState(filters.to ?? "");
  const invalid = Boolean(from && to && from > to);
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (invalid) return;
    router.push(
      queueHref({ ...filters, from: from || null, to: to || null, page: 1 }),
      { scroll: false },
    );
  };
  return (
    <form
      noValidate
      aria-label={t("label")}
      onSubmit={submit}
      className="flex flex-wrap items-end gap-2"
    >
      <div className="space-y-1">
        <label htmlFor="queue-from" className="text-xs font-medium">
          {t("from")}
        </label>
        <Input
          id="queue-from"
          type="date"
          value={from}
          max={to || undefined}
          onChange={(event) => setFrom(event.target.value)}
          className="h-10 rounded-full"
          aria-invalid={invalid}
          aria-describedby={invalid ? "queue-date-error" : undefined}
        />
      </div>
      <div className="space-y-1">
        <label htmlFor="queue-to" className="text-xs font-medium">
          {t("to")}
        </label>
        <Input
          id="queue-to"
          type="date"
          value={to}
          min={from || undefined}
          onChange={(event) => setTo(event.target.value)}
          className="h-10 rounded-full"
          aria-invalid={invalid}
          aria-describedby={invalid ? "queue-date-error" : undefined}
        />
      </div>
      <Button
        type="submit"
        variant="outline"
        className="h-10 rounded-full"
        disabled={invalid}
      >
        {t("apply")}
      </Button>
      {(filters.from || filters.to) && (
        <Button asChild variant="ghost" className="h-10 rounded-full">
          <Link
            href={queueHref({ ...filters, from: null, to: null, page: 1 })}
            scroll={false}
          >
            {t("clear")}
          </Link>
        </Button>
      )}
      {invalid && (
        <p
          id="queue-date-error"
          role="alert"
          className="w-full text-xs text-destructive"
        >
          {t("invalid")}
        </p>
      )}
    </form>
  );
}
