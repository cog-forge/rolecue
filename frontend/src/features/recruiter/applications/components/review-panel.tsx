"use client";

import { useEffect, useId } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("recruiter.applications");
  const tCommon = useTranslations("common");
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
        aria-label={t("panel.previous")}
        disabled={!onPrevious}
        onClick={onPrevious}
      >
        <ChevronLeft aria-hidden />
      </Button>
      {position && (
        <span className="min-w-12 text-center text-xs text-muted-foreground tabular-nums">
          {t("panel.position", {
            index: position.index + 1,
            count: position.count,
          })}
        </span>
      )}
      <Button
        variant="ghost"
        size="icon"
        className={iconButton}
        aria-label={t("panel.next")}
        disabled={!onNext}
        onClick={onNext}
      >
        <ChevronRight aria-hidden />
      </Button>
      <Button asChild variant="ghost" size="icon" className={iconButton}>
        <Link
          href={routes.recruiter.application(postingId, applicationId)}
          aria-label={t("panel.openOwnPageAria")}
          title={t("panel.openOwnPageTitle")}
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
              ? t("detail.notFoundTitle")
              : denied
                ? t("detail.deniedTitle")
                : t("detail.errorTitle")
          }
        >
          {!notFound && !denied && (
            <Button
              className="mt-2 h-11 bg-background"
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              {tCommon("actions.tryAgain")}
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
        <span className="sr-only">{t("detail.loading")}</span>
      </section>
    );

  const application = query.data;
  const { candidate, job_posting: posting } = application;
  const missed = isDeadlineMissed(application);
  const shownStatus = missed ? "rejected" : application.status;
  const reasonText =
    application.reject_reason ??
    (missed ? t("facts.deadlinePassedReason") : null);
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
          <dt className="text-xs text-muted-foreground">{t("facts.status")}</dt>
          <dd className="mt-1">
            <ApplicationStatus status={shownStatus} />
          </dd>
        </div>
        <div className="min-w-0">
          <dt className="text-xs text-muted-foreground">
            {t("facts.appliedFor")}
          </dt>
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
          <dt className="text-xs text-muted-foreground">
            {t("facts.submitted")}
          </dt>
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
          <dt className="text-xs text-muted-foreground">
            {t("facts.interviewSlots")}
          </dt>
          <dd className="mt-1 font-medium tabular-nums">
            {posting.interview_slot === null
              ? t("panel.slotsNotFunded")
              : t("panel.slotsApproved", {
                  approvals: posting.cv_approvals,
                  slots: posting.interview_slot,
                })}
          </dd>
        </div>
        {application.interview_deadline && (
          <div className="sm:col-span-2">
            <dt className="text-xs text-muted-foreground">
              {waitingForInterview
                ? t("facts.completeBy")
                : t("facts.interviewDeadline")}
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
            <dt className="text-xs text-muted-foreground">
              {t("facts.rejectReason")}
            </dt>
            <dd className="mt-1 font-medium [overflow-wrap:anywhere]">
              {reasonText}
            </dd>
          </div>
        )}
      </dl>

      {caughtUp && (
        <Notice tone="info" title={t("panel.caughtUpTitle")}>
          <p className="text-muted-foreground">{t("panel.caughtUpBody")}</p>
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
        <p className="text-sm text-muted-foreground">{t("panel.cleared")}</p>
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

      <section
        aria-label={t("detail.cvHeading")}
        className="space-y-3 border-t pt-5"
      >
        <h3 className="text-sm font-semibold">{t("detail.cvHeading")}</h3>
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
