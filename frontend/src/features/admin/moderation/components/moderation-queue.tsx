"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
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
import { PostingStatus } from "./posting-status";

const tabs: { status: QueueStatus; label: string }[] = [
  { status: "pending", label: "Pending" },
  { status: "open", label: "Approved" },
  { status: "rejected", label: "Rejected" },
  { status: "all", label: "All" },
];
const emptyText: Record<QueueStatus, string> = {
  pending: "No postings are waiting for review.",
  open: "No approved postings match these filters.",
  rejected: "No rejected postings match these filters.",
  intake_closed: "No postings with closed intake match these filters.",
  closed: "No closed postings match these filters.",
  all: "No postings match these filters.",
};
const dateFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});
const queueHref = (filters: QueueFilters) =>
  `${routes.admin.jobPostings}${queueSearch(filters)}`;

export function ModerationQueue({ filters }: { filters: QueueFilters }) {
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
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Moderation
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Job Posting Moderation
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Review what recruiters submit. Only postings you approve are published
          to candidates.
        </p>
      </header>

      <div className="flex flex-wrap items-end justify-between gap-4">
        <nav aria-label="Filter by status">
          <ul className="inline-flex flex-wrap gap-1 rounded-lg bg-muted p-1">
            {tabs.map((tab) => {
              const active = filters.status === tab.status;
              return (
                <li key={tab.status}>
                  <Link
                    href={queueHref({
                      ...filters,
                      status: tab.status,
                      page: 1,
                    })}
                    scroll={false}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "inline-flex min-h-11 items-center rounded-md px-4 text-sm font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
                      active && "bg-background text-foreground shadow-sm",
                    )}
                  >
                    {tab.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>
        <DateFilter key={`${filters.from}-${filters.to}`} filters={filters} />
      </div>

      {filters.recruiterId && (
        <p className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-muted-foreground">Showing one recruiter.</span>
          <Link
            href={queueHref({ ...filters, recruiterId: null, page: 1 })}
            scroll={false}
            className="inline-flex min-h-11 items-center gap-1 rounded-md px-2 font-medium underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <X className="size-4" aria-hidden />
            Show all recruiters
          </Link>
        </p>
      )}

      {query.isPending ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-14 rounded-md" />
          ))}
          <span className="sr-only">Loading job postings…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">
            You don&apos;t have access to moderation
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with an
            administrator account.
          </p>
        </div>
      ) : query.isError ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">
            We couldn&apos;t load the job postings
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please try again in a moment.
          </p>
          <Button
            className="mt-4 h-11"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </div>
      ) : page && page.items.length === 0 ? (
        <div className="rounded-xl border p-8 text-center">
          <h2 className="font-semibold">{emptyText[filters.status]}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            New submissions appear here as recruiters send them.
          </p>
        </div>
      ) : page ? (
        <>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-left text-sm">
              <caption className="sr-only">
                Job postings, {firstRow}–{lastRow} of {total}
              </caption>
              <thead>
                <tr className="border-b text-xs tracking-[0.08em] text-muted-foreground uppercase">
                  <th scope="col" className="py-3 pr-4 font-medium">
                    Job posting
                  </th>
                  <th scope="col" className="py-3 pr-4 font-medium">
                    Recruiter
                  </th>
                  <th scope="col" className="py-3 pr-4 font-medium">
                    Submitted
                  </th>
                  <th scope="col" className="py-3 pr-4 font-medium">
                    Status
                  </th>
                  <th scope="col" className="py-3 text-right font-medium">
                    <span className="sr-only">Action</span>
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
            aria-label="Pagination"
            className="flex flex-wrap items-center justify-between gap-3 text-sm"
          >
            <p className="text-muted-foreground tabular-nums">
              Showing {firstRow}–{lastRow} of {total}
            </p>
            <div className="flex gap-2">
              <PageLink
                label="Previous"
                href={queueHref({ ...filters, page: filters.page - 1 })}
                disabled={filters.page <= 1}
              />
              <PageLink
                label="Next"
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
  const created = new Date(posting.created_at);
  const reviewHref = `${routes.admin.jobPosting(posting.id)}${
    backQuery ? `?back=${encodeURIComponent(backQuery)}` : ""
  }`;
  return (
    <tr className="border-b align-top last:border-b-0">
      <td className="py-3 pr-4 font-medium [overflow-wrap:anywhere]">
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
          title={`Show only postings by ${posting.recruiter.name}`}
        >
          {posting.recruiter.name}
        </Link>
        <span className="block text-xs text-muted-foreground [overflow-wrap:anywhere]">
          {posting.recruiter.email}
        </span>
      </td>
      <td className="py-3 pr-4 whitespace-nowrap tabular-nums">
        <time dateTime={posting.created_at} title={posting.created_at}>
          {dateFormat.format(created)}
        </time>
      </td>
      <td className="py-3 pr-4">
        <PostingStatus status={posting.status} />
      </td>
      <td className="py-3 text-right">
        <Button asChild variant="outline" size="sm" className="h-9">
          <Link href={reviewHref} aria-label={`Review ${posting.title}`}>
            Review
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

function DateFilter({ filters }: { filters: QueueFilters }) {
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
      aria-label="Filter by submitted date"
      onSubmit={submit}
      className="flex flex-wrap items-end gap-2"
    >
      <div className="space-y-1">
        <label htmlFor="queue-from" className="text-xs font-medium">
          Submitted from
        </label>
        <Input
          id="queue-from"
          type="date"
          value={from}
          max={to || undefined}
          onChange={(event) => setFrom(event.target.value)}
          className="h-11"
          aria-invalid={invalid}
          aria-describedby={invalid ? "queue-date-error" : undefined}
        />
      </div>
      <div className="space-y-1">
        <label htmlFor="queue-to" className="text-xs font-medium">
          to
        </label>
        <Input
          id="queue-to"
          type="date"
          value={to}
          min={from || undefined}
          onChange={(event) => setTo(event.target.value)}
          className="h-11"
          aria-invalid={invalid}
          aria-describedby={invalid ? "queue-date-error" : undefined}
        />
      </div>
      <Button
        type="submit"
        variant="outline"
        className="h-11"
        disabled={invalid}
      >
        Apply
      </Button>
      {(filters.from || filters.to) && (
        <Button asChild variant="ghost" className="h-11">
          <Link
            href={queueHref({ ...filters, from: null, to: null, page: 1 })}
            scroll={false}
          >
            Clear dates
          </Link>
        </Button>
      )}
      {invalid && (
        <p
          id="queue-date-error"
          role="alert"
          className="w-full text-xs text-destructive"
        >
          The start date must be on or before the end date.
        </p>
      )}
    </form>
  );
}
