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
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  candidateApplicationKeys,
  listCandidateApplications,
} from "../api/applications";
import type { CandidateApplication } from "../schemas/candidate-application-schema";
import { isAccessDenied } from "../utils/api-error";
import { applicationHref } from "../utils/application-links";
import { formatSubmitted, statusLabel } from "../utils/application-text";

// Page numbers come from the URL so reload and back keep the page. Anything not a positive whole
// number falls back to the first page.
function pageFrom(value: string | null) {
  const page = Number(value);
  return Number.isSafeInteger(page) && page > 0 ? page : 1;
}

// The Candidate's own Applications, newest first.
export function CandidateApplicationList() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const params = useSearchParams();
  const page = pageFrom(params.get("page"));

  const query = useQuery({
    queryKey: candidateApplicationKeys.list(user.id, page),
    queryFn: ({ signal }) => listCandidateApplications(page, signal),
    refetchOnMount: "always",
    retry: false,
    placeholderData: keepPreviousData,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  if (query.isPending)
    return (
      <div role="status" className="space-y-3">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-20 rounded-xl" />
        ))}
        <span className="sr-only">Loading your applications…</span>
      </div>
    );

  if (denied)
    return (
      <div role="alert" className="rounded-xl border bg-muted/60 p-6">
        <h2 className="font-semibold">
          You don&apos;t have access to applications
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Checking your account access… If this stays, sign in again with a
          candidate account.
        </p>
      </div>
    );

  if (query.isError)
    return (
      <div
        role="alert"
        className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
      >
        <h2 className="font-semibold">
          We couldn&apos;t load your applications
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
    );

  const { items, total, page_size: pageSize } = query.data;
  const firstRow = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const lastRow = Math.min(page * pageSize, total);

  if (items.length === 0 && page > 1)
    return (
      <div className="rounded-xl border p-8 text-center">
        <h2 className="font-semibold">
          This page is past the end of your list.
        </h2>
        <Button asChild variant="outline" className="mt-4 h-11">
          <Link href={routes.applications}>Go to the first page</Link>
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
          You haven&apos;t applied to any postings yet.
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Find an open posting on the Job Board and apply with your CV.
        </p>
        <Button asChild variant="outline" className="mt-4 h-11">
          <Link href={routes.jobs}>Browse the Job Board</Link>
        </Button>
      </div>
    );

  return (
    <div className="space-y-4">
      <ul className="divide-y rounded-2xl border bg-card">
        {items.map((application) => (
          <ApplicationItem key={application.id} application={application} />
        ))}
      </ul>
      <nav
        aria-label="Applications pagination"
        className="flex flex-wrap items-center justify-between gap-3 text-sm"
      >
        <p className="text-muted-foreground tabular-nums">
          Showing {firstRow}–{lastRow} of {total}
        </p>
        <div className="flex gap-2">
          <PageButton
            label="Previous"
            href={`${routes.applications}?page=${page - 1}`}
            disabled={page <= 1}
          />
          <PageButton
            label="Next"
            href={`${routes.applications}?page=${page + 1}`}
            disabled={lastRow >= total}
          />
        </div>
      </nav>
    </div>
  );
}

function ApplicationItem({
  application,
}: {
  application: CandidateApplication;
}) {
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 p-4 sm:px-5">
      <div className="min-w-0 space-y-1">
        <Link
          href={applicationHref(application.id)}
          className="rounded-sm font-medium underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {application.job_posting_title}
        </Link>
        <p className="text-xs text-muted-foreground tabular-nums">
          Submitted{" "}
          <time dateTime={application.created_at}>
            {formatSubmitted(application.created_at)}
          </time>
        </p>
      </div>
      <p className="text-sm font-medium">{statusLabel[application.status]}</p>
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
      <Link href={href}>{label}</Link>
    </Button>
  );
}
