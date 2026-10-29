"use client";

import { useEffect } from "react";
import Link from "next/link";
import {
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ChevronRight, CircleCheck } from "lucide-react";
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
      query: pending,
      title: "CVs awaiting review",
      description: "Candidates applied and wait for your Approve or Reject.",
      action: "Review CVs",
      href: inboxHref("pending"),
    },
    {
      query: deadlines,
      title: "Interview deadlines in the next 24 hours",
      description: "Cleared candidates who must finish their interview soon.",
      action: "View candidates",
      href: inboxHref("interview_eligible"),
      explain: (todo) =>
        [
          todo.detail && `Earliest: ${formatDeadline(todo.detail, fmt)}.`,
          todo.atLeast &&
            "Only the first 100 cleared candidates were checked, so more may be due.",
        ]
          .filter(Boolean)
          .join(" "),
    },
    {
      query: rejected,
      title: "Rejected postings to fix",
      description: "An administrator sent these back. Edit and resubmit them.",
      action: "Fix postings",
      href: postingsHref("rejected"),
    },
    {
      query: unfunded,
      title: "Approved, waiting for funding",
      description: "Fund interview slots to publish these on the Job Board.",
      action: "Fund & publish",
      href: postingsHref("approved"),
    },
    {
      query: drafts,
      title: "Drafts to submit",
      description:
        "Postings you are still preparing. Check the questions and criteria, then submit.",
      action: "Continue drafts",
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
      <PageHeader
        title="Dashboard"
        description="What needs you now across your job postings and received applications."
      />

      {denied ? (
        <Notice
          role="alert"
          tone="locked"
          title="You don't have access to the dashboard"
        >
          <p className="text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            recruiter account.
          </p>
        </Notice>
      ) : allClear ? (
        <div className="rounded-xl border border-dashed p-10 text-center">
          <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <CircleCheck className="size-6" aria-hidden />
          </span>
          <h2 className="font-semibold">You&apos;re all caught up</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Nothing is waiting for you right now. New applications and decisions
            will show up here.
          </p>
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={routes.recruiter.jobPostingNew}>
              Create a job posting
            </Link>
          </Button>
        </div>
      ) : (
        <ul
          aria-label="Things that need your attention"
          className="divide-y rounded-xl border bg-card"
        >
          {todos.map((todo) => (
            <TodoRow key={todo.title} {...todo} />
          ))}
        </ul>
      )}

      <Glance live={live} closed={closed} />
    </section>
  );
}

type TodoRowProps = {
  query: UseQueryResult<Todo>;
  title: string;
  description: string;
  action: string;
  href: string;
  // Extra words that depend on the loaded numbers, shown under the description.
  explain?: (todo: Todo) => string;
};

function TodoRow({
  query,
  title,
  description,
  action,
  href,
  explain,
}: TodoRowProps) {
  const fmt = useFormatters();
  if (query.isPending)
    return (
      <li>
        <div role="status" className="flex items-center gap-4 px-4 py-4">
          <Skeleton className="h-8 w-12 rounded-md" />
          <Skeleton className="h-10 flex-1 rounded-md" />
          <span className="sr-only">Loading {title.toLowerCase()}…</span>
        </div>
      </li>
    );
  if (query.isError)
    return (
      <li className="flex flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <p className="font-medium">{title}</p>
          <p role="alert" className="text-sm text-destructive">
            We couldn&apos;t load this count.
          </p>
        </div>
        <Button
          variant="outline"
          className="h-11"
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          Try again
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
          aria-label={`${fmt.number(todo.count)}${todo.atLeast ? " or more" : ""}`}
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
  const fmt = useFormatters();
  if (!live.isSuccess || !closed.isSuccess) return null;
  const items = [
    { label: "Live", count: live.data.count, href: postingsHref("open") },
    {
      label: "Intake closed",
      count: closed.data.count,
      href: postingsHref("intake_closed"),
    },
  ];
  if (items.every((item) => item.count === 0)) return null;
  return (
    <section aria-labelledby="glance-heading" className="space-y-3">
      <h2 id="glance-heading" className="text-sm font-semibold">
        Your postings at a glance
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
