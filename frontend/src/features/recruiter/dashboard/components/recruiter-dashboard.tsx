"use client";

import { useEffect } from "react";
import Link from "next/link";
import {
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ChevronRight, CircleCheck } from "lucide-react";
import { useTranslations } from "next-intl";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  applicationSearch,
  defaultApplicationFilters,
} from "@/features/recruiter/applications/utils/application-filters";
import { formatDeadline } from "@/features/recruiter/applications/utils/application-dates";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import { isAccessDenied } from "@/features/recruiter/job-postings/utils/api-error";
import {
  defaultListFilters,
  listSearch,
  type ListStatus,
} from "@/features/recruiter/job-postings/utils/list-filters";
import {
  countPendingApplications,
  countPostings,
  dashboardKeys,
  summarizeDeadlines,
} from "../api/dashboard";
import type { Todo } from "../schemas/dashboard-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

const postingsHref = (status: ListStatus) =>
  `${routes.recruiter.jobPostings}${listSearch({ ...defaultListFilters, status })}`;
const inboxHref = (status: "pending" | "interview_eligible") =>
  `${routes.recruiter.applications}${applicationSearch({ ...defaultApplicationFilters, status })}`;

// Counts go stale while the Recruiter works elsewhere, so they refresh when the tab regains focus.
const queryOptions = { refetchOnWindowFocus: true, retry: false } as const;

function usePostingCount(status: ListStatus) {
  const user = useSessionUser();
  return useQuery({
    queryKey: dashboardKeys.postings(user.id, status),
    queryFn: ({ signal }) => countPostings(status, signal),
    ...queryOptions,
  });
}

// What needs the Recruiter now: one row per kind of work, with how much there is and where to do it.
export function RecruiterDashboard() {
  const t = useTranslations("recruiter.dashboard");
  const fmt = useFormatters();
  const user = useSessionUser();
  const cache = useQueryClient();
  const pending = useQuery({
    queryKey: dashboardKeys.applications(user.id, "pending"),
    queryFn: ({ signal }) => countPendingApplications(signal),
    ...queryOptions,
  });
  const deadlines = useQuery({
    queryKey: dashboardKeys.deadlines(user.id),
    queryFn: ({ signal }) => summarizeDeadlines(signal),
    ...queryOptions,
  });
  const rejected = usePostingCount("rejected");
  const unfunded = usePostingCount("approved");
  const drafts = usePostingCount("preparing");
  const live = usePostingCount("open");
  const closed = usePostingCount("intake_closed");

  const todos: TodoRowProps[] = [
    {
      id: "pending",
      query: pending,
      title: t("todo.pending.title"),
      description: t("todo.pending.description"),
      action: t("todo.pending.action"),
      loading: t("todo.pending.loading"),
      href: inboxHref("pending"),
    },
    {
      id: "deadlines",
      query: deadlines,
      title: t("todo.deadlines.title"),
      description: t("todo.deadlines.description"),
      action: t("todo.deadlines.action"),
      loading: t("todo.deadlines.loading"),
      href: inboxHref("interview_eligible"),
      explain: (todo) =>
        [
          todo.detail &&
            t("todo.deadlines.earliest", {
              date: formatDeadline(todo.detail, fmt),
            }),
          todo.atLeast && t("todo.deadlines.partial"),
        ]
          .filter(Boolean)
          .join(" "),
    },
    {
      id: "rejected",
      query: rejected,
      title: t("todo.rejected.title"),
      description: t("todo.rejected.description"),
      action: t("todo.rejected.action"),
      loading: t("todo.rejected.loading"),
      href: postingsHref("rejected"),
    },
    {
      id: "unfunded",
      query: unfunded,
      title: t("todo.unfunded.title"),
      description: t("todo.unfunded.description"),
      action: t("todo.unfunded.action"),
      loading: t("todo.unfunded.loading"),
      href: postingsHref("approved"),
    },
    {
      id: "drafts",
      query: drafts,
      title: t("todo.drafts.title"),
      description: t("todo.drafts.description"),
      action: t("todo.drafts.action"),
      loading: t("todo.drafts.loading"),
      href: postingsHref("preparing"),
    },
  ];

  const denied = [...todos.map((todo) => todo.query), live, closed].some(
    (query) => isAccessDenied(query.error),
  );
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  // "All caught up" is only claimed when every count loaded and is exactly zero.
  const allClear = todos.every(
    ({ query }) =>
      query.isSuccess && query.data.count === 0 && !query.data.atLeast,
  );

  return (
    <section className="mx-auto w-full max-w-5xl space-y-8">
      <PageHeader title={t("title")} description={t("description")} />

      {denied ? (
        <Notice role="alert" tone="locked" title={t("denied.title")}>
          <p className="text-muted-foreground">{t("denied.description")}</p>
        </Notice>
      ) : allClear ? (
        <div className="rounded-xl border border-dashed p-10 text-center">
          <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <CircleCheck className="size-6" aria-hidden />
          </span>
          <h2 className="font-semibold">{t("allClear.title")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("allClear.description")}
          </p>
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={routes.recruiter.jobPostingNew}>
              {t("allClear.create")}
            </Link>
          </Button>
        </div>
      ) : (
        <ul
          aria-label={t("todosLabel")}
          className="divide-y rounded-xl border bg-card"
        >
          {todos.map((todo) => (
            <TodoRow key={todo.id} {...todo} />
          ))}
        </ul>
      )}

      <Glance live={live} closed={closed} />
    </section>
  );
}

type TodoRowProps = {
  id: string;
  query: UseQueryResult<Todo>;
  title: string;
  description: string;
  action: string;
  // Screen-reader text while the count loads.
  loading: string;
  href: string;
  // Extra words that depend on the loaded numbers, shown under the description.
  explain?: (todo: Todo) => string;
};

function TodoRow({
  query,
  title,
  description,
  action,
  loading,
  href,
  explain,
}: TodoRowProps) {
  const t = useTranslations("recruiter.dashboard.todo");
  const tCommon = useTranslations("common");
  const fmt = useFormatters();
  if (query.isPending)
    return (
      <li>
        <div role="status" className="flex items-center gap-4 px-4 py-4">
          <Skeleton className="h-8 w-12 rounded-md" />
          <Skeleton className="h-10 flex-1 rounded-md" />
          <span className="sr-only">{loading}</span>
        </div>
      </li>
    );
  if (query.isError)
    return (
      <li className="flex flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <p className="font-medium">{title}</p>
          <p role="alert" className="text-sm text-destructive">
            {t("loadFailed")}
          </p>
        </div>
        <Button
          variant="outline"
          className="h-11"
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          {tCommon("actions.tryAgain")}
        </Button>
      </li>
    );
  const todo = query.data;
  // Nothing to do in a row means nothing to show, unless the count is only a lower bound.
  if (todo.count === 0 && !todo.atLeast) return null;
  const extra = explain?.(todo);
  return (
    <li className="flex flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:gap-6">
      <div className="flex min-w-0 items-start gap-4">
        <p
          className="min-w-12 text-2xl leading-8 font-semibold text-rolecue-brand tabular-nums dark:text-sky-300"
          aria-label={
            todo.atLeast
              ? t("atLeast", { count: fmt.number(todo.count) })
              : fmt.number(todo.count)
          }
        >
          {fmt.number(todo.count)}
          {todo.atLeast && "+"}
        </p>
        <div className="min-w-0 space-y-1">
          <p className="font-medium">{title}</p>
          <p className="text-sm text-muted-foreground">{description}</p>
          {extra && <p className="text-sm text-muted-foreground">{extra}</p>}
        </div>
      </div>
      <Button
        asChild
        variant="outline"
        className="h-11 shrink-0 self-start sm:self-center"
      >
        <Link href={href}>
          {action}
          <ChevronRight aria-hidden />
        </Link>
      </Button>
    </li>
  );
}

// Two quiet numbers for postings that need nothing from the Recruiter; hidden while loading or when
// either count is unavailable, since the to-do list above is what matters.
function Glance({
  live,
  closed,
}: {
  live: UseQueryResult<Todo>;
  closed: UseQueryResult<Todo>;
}) {
  const t = useTranslations("recruiter.dashboard.glance");
  const fmt = useFormatters();
  if (!live.isSuccess || !closed.isSuccess) return null;
  const items = [
    { label: t("live"), count: live.data.count, href: postingsHref("open") },
    {
      label: t("intakeClosed"),
      count: closed.data.count,
      href: postingsHref("intake_closed"),
    },
  ];
  if (items.every((item) => item.count === 0)) return null;
  return (
    <section aria-labelledby="glance-heading" className="space-y-3">
      <h2 id="glance-heading" className="text-sm font-semibold">
        {t("title")}
      </h2>
      <ul className="grid gap-3 sm:grid-cols-2">
        {items.map((item) => (
          <li key={item.label}>
            <Link
              href={item.href}
              className="flex min-h-11 items-center justify-between gap-3 rounded-xl border px-4 py-3 outline-none transition-colors hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <span className="text-sm text-muted-foreground">
                {item.label}
              </span>
              <span className="text-lg font-semibold tabular-nums">
                {fmt.number(item.count)}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
