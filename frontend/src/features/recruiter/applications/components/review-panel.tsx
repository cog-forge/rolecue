"use client";

import { useEffect, useId } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import {
  apiError,
  isAccessDenied,
} from "@/features/recruiter/job-postings/utils/api-error";
import { applicationKeys, getApplicationDetail } from "../api/applications";
import {
  formatDeadline,
  formatSubmitted,
  isDeadlineMissed,
} from "../utils/application-dates";
import { Initials } from "./application-list";
import { ApplicationStatus } from "./application-status";
import { CvPreview } from "./cv-link";
import { FinalDecision } from "./final-decision";
import { InterviewResult } from "./interview-result";
import { ScreeningActions } from "./screening-actions";
import { useFormatters } from "@/lib/i18n/use-formatters";

const iconButton = "size-11";

// The review side of the inbox: one Application with its CV, the screening decision and, later,
// the interview result and final decision. Everything a decision needs stays on this panel.
export function ReviewPanel({
  postingId,
  applicationId,
  position,
  onPrevious,
  onNext,
  onDecided,
  caughtUp,
}: {
  postingId: string;
  applicationId: string;
  position: { index: number; count: number } | null;
  onPrevious?: () => void;
  onNext?: () => void;
  onDecided: (decision: "approved" | "rejected") => void;
  caughtUp: boolean;
}) {
  const fmt = useFormatters();
  const user = useSessionUser();
  const cache = useQueryClient();
  const headingId = useId();
  const query = useQuery({
    queryKey: applicationKeys.detail(user.id, postingId, applicationId),
    queryFn: ({ signal }) =>
      getApplicationDetail(postingId, applicationId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  const notFound = apiError(query.error).status === 404;
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const navigation = (
    <div className="flex shrink-0 items-center gap-1">
      <Button
        variant="ghost"
        size="icon"
        className={iconButton}
        aria-label="Previous application"
        disabled={!onPrevious}
        onClick={onPrevious}
      >
        <ChevronLeft aria-hidden />
      </Button>
      {position && (
        <span className="min-w-12 text-center text-xs text-muted-foreground tabular-nums">
          {position.index + 1} of {position.count}
        </span>
      )}
      <Button
        variant="ghost"
        size="icon"
        className={iconButton}
        aria-label="Next application"
        disabled={!onNext}
        onClick={onNext}
      >
        <ChevronRight aria-hidden />
      </Button>
      <Button asChild variant="ghost" size="icon" className={iconButton}>
        <Link
          href={routes.recruiter.application(postingId, applicationId)}
          aria-label="Open the application on its own page"
          title="Open on its own page"
        >
          <ExternalLink aria-hidden />
        </Link>
      </Button>
    </div>
  );

  if (notFound || denied || (query.isError && !query.data))
    return (
      <section className="space-y-4 rounded-xl border bg-card p-5">
        <div className="flex justify-end">{navigation}</div>
        <Notice
          role="alert"
          tone={notFound || denied ? "locked" : "danger"}
          title={
            notFound
              ? "We couldn't find this application"
              : denied
                ? "You don't have access to this application"
                : "We couldn't load this application"
          }
        >
          {!notFound && !denied && (
            <Button
              className="mt-2 h-11 bg-background"
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              Try again
            </Button>
          )}
        </Notice>
      </section>
    );

  if (query.isPending)
    return (
      <section
        role="status"
        className="space-y-4 rounded-xl border bg-card p-5"
      >
        <div className="flex items-center gap-3">
          <Skeleton className="size-9 rounded-full" />
          <Skeleton className="h-6 w-1/2 rounded-md" />
        </div>
        <Skeleton className="h-16 rounded-lg" />
        <Skeleton className="h-96 rounded-lg" />
        <span className="sr-only">Loading the application…</span>
      </section>
    );

  const application = query.data;
  const { candidate, job_posting: posting } = application;
  const missed = isDeadlineMissed(application);
  const shownStatus = missed ? "rejected" : application.status;
  const reasonText =
    application.reject_reason ?? (missed ? "Interview deadline passed" : null);
  const waitingForInterview =
    application.status === "interview_eligible" &&
    application.interview_eligible;

  return (
    <section
      aria-labelledby={headingId}
      className="space-y-5 rounded-xl border bg-card p-5"
    >
      <div className="flex items-start gap-3">
        <Initials name={candidate.name} />
        <div className="min-w-0 flex-1">
          <h2
            id={headingId}
            className="text-lg leading-tight font-semibold [overflow-wrap:anywhere]"
          >
            {candidate.name}
          </h2>
          <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
            {candidate.email}
          </p>
        </div>
        {navigation}
      </div>

      <dl className="grid gap-x-6 gap-y-3 rounded-lg bg-muted/40 p-4 text-sm sm:grid-cols-2 xl:grid-cols-4">
        <div>
          <dt className="text-xs text-muted-foreground">Status</dt>
          <dd className="mt-1">
            <ApplicationStatus status={shownStatus} />
          </dd>
        </div>
        <div className="min-w-0">
          <dt className="text-xs text-muted-foreground">Applied for</dt>
          <dd className="mt-1 font-medium [overflow-wrap:anywhere]">
            <Link
              href={routes.recruiter.jobPosting(postingId)}
              className="rounded-sm underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {posting.title}
            </Link>
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Submitted</dt>
          <dd className="mt-1 font-medium tabular-nums">
            <time
              dateTime={application.submitted_at}
              title={application.submitted_at}
            >
              {formatSubmitted(application.submitted_at, fmt)}
            </time>
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Interview slots</dt>
          <dd className="mt-1 font-medium tabular-nums">
            {posting.interview_slot === null
              ? "Not funded"
              : `${posting.cv_approvals} of ${posting.interview_slot} approved`}
          </dd>
        </div>
        {application.interview_deadline && (
          <div className="sm:col-span-2">
            <dt className="text-xs text-muted-foreground">
              {waitingForInterview
                ? "Complete interview by"
                : "Interview deadline"}
            </dt>
            <dd className="mt-1 font-medium tabular-nums">
              <time
                dateTime={application.interview_deadline}
                title={application.interview_deadline}
              >
                {formatDeadline(application.interview_deadline, fmt)}
              </time>
            </dd>
          </div>
        )}
        {reasonText && (
          <div className="sm:col-span-2">
            <dt className="text-xs text-muted-foreground">Reject reason</dt>
            <dd className="mt-1 font-medium [overflow-wrap:anywhere]">
              {reasonText}
            </dd>
          </div>
        )}
      </dl>

      {caughtUp && (
        <Notice tone="info" title="You're all caught up">
          <p className="text-muted-foreground">
            No other CVs on this page are waiting for review.
          </p>
        </Notice>
      )}

      <ScreeningActions
        postingId={postingId}
        applicationId={applicationId}
        application={application}
        layout="inline"
        onDecided={onDecided}
      />

      {waitingForInterview && (
        <p className="text-sm text-muted-foreground">
          Cleared to interview. The result appears here once the candidate
          completes it.
        </p>
      )}

      {application.interview && (
        <InterviewResult
          postingId={postingId}
          applicationId={applicationId}
          interview={application.interview}
        />
      )}

      {application.status === "interviewed" && (
        <FinalDecision
          postingId={postingId}
          applicationId={applicationId}
          application={application}
        />
      )}

      <section aria-label="CV" className="space-y-3 border-t pt-5">
        <h3 className="text-sm font-semibold">CV</h3>
        <CvPreview
          key={applicationId}
          postingId={postingId}
          applicationId={applicationId}
          cv={application.cv}
          candidateName={candidate.name}
        />
      </section>
    </section>
  );
}
