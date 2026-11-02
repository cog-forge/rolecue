"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import { PageHeader } from "@/components/layout/page-header";
import {
  apiError,
  isAccessDenied,
} from "@/features/recruiter/job-postings/utils/api-error";
import { applicationKeys, getApplicationDetail } from "../api/applications";
import {
  defaultApplicationFilters,
  inboxHref,
} from "../utils/application-filters";
import {
  formatDeadline,
  formatSubmitted,
  isDeadlineMissed,
} from "../utils/application-dates";
import type { ApplicationDetail as ApplicationDetailRecord } from "../schemas/application-detail-schema";
import { ApplicationStatus } from "./application-status";
import { CvPreview } from "./cv-link";
import { FinalDecision } from "./final-decision";
import { InterviewResult } from "./interview-result";
import { ScreeningActions } from "./screening-actions";
import { useFormatters } from "@/lib/i18n/use-formatters";

// The label follows the list's wording: an eligible Application shows the time left to interview.
// A missed deadline is shown as rejected, so its deadline is labelled as the former deadline.
function isWaitingForInterview(application: ApplicationDetailRecord) {
  return (
    application.status === "interview_eligible" &&
    application.interview_eligible
  );
}

// One Recruiter's Application: the Candidate, the status and CV, and the CV screening decision.
export function ApplicationDetail({
  postingId,
  applicationId,
}: {
  postingId: string;
  applicationId: string;
}) {
  const t = useTranslations("recruiter.applications");
  const tCommon = useTranslations("common");
  const fmt = useFormatters();
  const user = useSessionUser();
  const cache = useQueryClient();
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

  if (notFound)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <Notice role="alert" tone="locked" title={t("detail.notFoundTitle")}>
          <p className="text-muted-foreground">{t("detail.notFoundBody")}</p>
          <Button asChild variant="outline" className="mt-2 h-11 bg-background">
            <Link href={routes.recruiter.applications}>
              {t("detail.backToReceived")}
            </Link>
          </Button>
        </Notice>
      </section>
    );
  if (denied)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <Notice role="alert" tone="locked" title={t("detail.deniedTitle")}>
          <p className="text-muted-foreground">{t("detail.deniedBody")}</p>
        </Notice>
      </section>
    );
  // A failed background refetch keeps the loaded Application and its decision controls on screen.
  if (query.isError && !query.data)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <Notice role="alert" tone="danger" title={t("detail.errorTitle")}>
          <p className="text-muted-foreground">
            {tCommon("messages.tryAgainLater")}
          </p>
          <Button
            className="mt-2 h-11 bg-background"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            {tCommon("actions.tryAgain")}
          </Button>
        </Notice>
      </section>
    );

  // Back goes to the review inbox, narrowed to this posting with this Application open.
  const back = {
    href: inboxHref(
      { ...defaultApplicationFilters, posting: postingId },
      applicationId,
    ),
    label: t("detail.back"),
  };

  if (query.isPending)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <PageHeader
          back={back}
          title={<Skeleton className="h-8 w-2/3 rounded-md" />}
        />
        <div role="status" className="space-y-3">
          <Skeleton className="h-48 rounded-xl" />
          <span className="sr-only">{t("detail.loading")}</span>
        </div>
      </section>
    );

  const application = query.data;
  if (!application) return null;
  const { candidate, job_posting: posting } = application;
  const missed = isDeadlineMissed(application);
  const shownStatus = missed ? "rejected" : application.status;
  const reasonText =
    application.reject_reason ??
    (missed ? t("facts.deadlinePassedReason") : null);
  return (
    <section className="mx-auto w-full max-w-3xl space-y-6">
      <PageHeader
        back={back}
        title={candidate.name}
        meta={<span>{t("detail.appliedFor", { title: posting.title })}</span>}
      />

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("detail.candidateHeading")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <div className="min-w-0">
              <dt className="text-muted-foreground">{t("facts.email")}</dt>
              <dd className="font-medium [overflow-wrap:anywhere]">
                {candidate.email}
              </dd>
            </div>
            <div className="min-w-0">
              <dt className="text-muted-foreground">{t("facts.jobPosting")}</dt>
              <dd className="font-medium [overflow-wrap:anywhere]">
                <Link
                  href={routes.recruiter.jobPosting(postingId)}
                  className="rounded-sm underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  {posting.title}
                </Link>
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("detail.applicationHeading")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-muted-foreground">{t("facts.status")}</dt>
              <dd className="mt-1">
                <ApplicationStatus status={shownStatus} />
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">{t("facts.submitted")}</dt>
              <dd className="font-medium tabular-nums">
                <time
                  dateTime={application.submitted_at}
                  title={application.submitted_at}
                >
                  {formatSubmitted(application.submitted_at, fmt)}
                </time>
              </dd>
            </div>
            {application.interview_deadline && (
              <div>
                <dt className="text-muted-foreground">
                  {isWaitingForInterview(application)
                    ? t("facts.completeBy")
                    : t("facts.interviewDeadline")}
                </dt>
                <dd className="font-medium tabular-nums">
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
                <dt className="text-muted-foreground">
                  {t("facts.rejectReason")}
                </dt>
                <dd className="font-medium [overflow-wrap:anywhere]">
                  {reasonText}
                </dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("detail.cvHeading")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CvPreview
            key={applicationId}
            postingId={postingId}
            applicationId={applicationId}
            cv={application.cv}
            candidateName={candidate.name}
          />
        </CardContent>
      </Card>

      <ScreeningActions
        postingId={postingId}
        applicationId={applicationId}
        application={application}
      />

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
    </section>
  );
}
