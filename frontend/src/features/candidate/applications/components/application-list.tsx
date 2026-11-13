"use client";
import { EmptyState } from "@/components/ui/empty-state";

import { ErrorState } from "@/components/ui/error-state";

import { useEffect } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { ArrowRight, Inbox } from "lucide-react";
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
import { formatSubmitted, timeLeftText } from "../utils/application-text";
import { useNow } from "../utils/use-now";
import { ApplicationProgress } from "./application-progress";
import { CandidateApplicationStatusBadge } from "./application-status";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { useTranslations } from "next-intl";

// Page numbers come from the URL so reload and back keep the page. Anything not a positive whole
// number falls back to the first page.
function pageFrom(value: string | null) {
  const page = Number(value);
  return Number.isSafeInteger(page) && page > 0 ? page : 1;
}

// The Candidate's own Applications, newest first.
export function CandidateApplicationList() {
  const t = useTranslations("candidate.applications");
  const user = useSessionUser();
  const cache = useQueryClient();
  const params = useSearchParams();
  const page = pageFrom(params.get("page"));
  const now = useNow();

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
        <span className="sr-only">{t("list.loading")}</span>
      </div>
    );

  if (denied)
    return (
      <ErrorState
        title={t("list.deniedTitle")}
        description={t("list.deniedBody")}
      ></ErrorState>
    );

  if (query.isError)
    return (
      <ErrorState
        title={t("list.errorTitle")}
        description={t("list.errorBody")}
        onRetry={() => void query.refetch()}
        retryLabel={t("actions.tryAgain")}
        isRetrying={query.isFetching}
      ></ErrorState>
    );

  const { items, total, page_size: pageSize } = query.data;
  const firstRow = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const lastRow = Math.min(page * pageSize, total);

  if (items.length === 0 && page > 1)
    return (
      <EmptyState title={t("list.pastEnd")} description="">
        <Button
          asChild
          variant="outline"
          className="mt-4 h-auto min-h-11 whitespace-normal py-2"
        >
          <Link href={routes.applications}>{t("list.firstPage")}</Link>
        </Button>
      </EmptyState>
    );

  if (items.length === 0)
    return (
      <EmptyState
        icons={[Inbox]}
        title={t("list.emptyTitle")}
        description={t("list.emptyBody")}
      >
        <Button
          asChild
          variant="outline"
          className="mt-4 h-auto min-h-11 whitespace-normal py-2"
        >
          <Link href={routes.jobs}>{t("list.browse")}</Link>
        </Button>
      </EmptyState>
    );

  return (
    <div className="space-y-4">
      <ul
        aria-label={t("list.listLabel")}
        className="divide-y rounded-2xl border bg-card"
      >
        {items.map((application) => (
          <ApplicationItem
            key={application.id}
            application={application}
            now={now}
          />
        ))}
      </ul>
      <nav
        aria-label={t("list.paginationLabel")}
        className="flex flex-wrap items-center justify-between gap-3 text-sm"
      >
        <p className="text-muted-foreground tabular-nums">
          {t("list.showing", { first: firstRow, last: lastRow, total })}
        </p>
        <div className="flex gap-2">
          <PageButton
            label={t("list.previous")}
            href={`${routes.applications}?page=${page - 1}`}
            disabled={page <= 1}
          />
          <PageButton
            label={t("list.next")}
            href={`${routes.applications}?page=${page + 1}`}
            disabled={lastRow >= total}
          />
        </div>
      </nav>
    </div>
  );
}

// One Application: where it stands, and the one thing to do next.
function ApplicationItem({
  application,
  now,
}: {
  application: CandidateApplication;
  now: number;
}) {
  const fmt = useFormatters();
  const t = useTranslations("candidate.applications");
  const {
    status,
    interview_deadline: deadline,
    reject_reason: reason,
  } = application;
  const where = [application.company_name, application.job_posting_location]
    .filter(Boolean)
    .join(" · ");
  const resuming = application.interview?.status === "in_progress";
  return (
    <li className="grid gap-4 p-4 sm:px-5 md:grid-cols-[minmax(0,1fr)_minmax(0,16rem)_auto] md:items-center md:gap-6">
      <div className="min-w-0 space-y-1.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <Link
            href={applicationHref(application.id)}
            className="rounded-sm font-semibold underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {application.job_posting_title}
          </Link>
          <CandidateApplicationStatusBadge status={status} />
        </div>
        {where && (
          <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
            {where}
          </p>
        )}
        <p className="text-xs text-muted-foreground tabular-nums">
          {t.rich("list.submitted", {
            date: formatSubmitted(application.created_at, fmt),
            time: (chunks) => (
              <time dateTime={application.created_at}>{chunks}</time>
            ),
          })}
        </p>
        {status === "rejected" && reason && (
          <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
            {t("list.recruiterNote", { reason })}
          </p>
        )}
      </div>
      <ApplicationProgress application={application} />
      <div className="flex flex-col gap-1 md:items-end">
        {application.can_start_interview ? (
          <>
            <Button asChild className="h-11">
              <Link
                href={`${routes.interviews.new.preflight}?applicationId=${encodeURIComponent(application.id)}`}
              >
                {resuming ? t("actions.resume") : t("actions.start")}
                <ArrowRight aria-hidden />
              </Link>
            </Button>
            {deadline && (
              <p className="text-xs text-muted-foreground tabular-nums">
                {timeLeftText(deadline, now, fmt)}
              </p>
            )}
          </>
        ) : (
          <Button asChild variant="outline" className="h-11">
            <Link
              href={applicationHref(application.id)}
              aria-label={t("list.viewAria", {
                title: application.job_posting_title,
              })}
            >
              {t("list.viewApplication")}
            </Link>
          </Button>
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
      <Link href={href}>{label}</Link>
    </Button>
  );
}
