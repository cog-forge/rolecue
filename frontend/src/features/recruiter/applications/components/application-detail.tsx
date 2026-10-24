"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  headerBandClass,
  Notice,
} from "@/features/recruiter/job-postings/components/notice";
import {
  apiError,
  isAccessDenied,
} from "@/features/recruiter/job-postings/utils/api-error";
import { cn } from "@/lib/utils";
import { applicationKeys, getApplicationDetail } from "../api/applications";
import type { ApplicationDetail as ApplicationDetailRecord } from "../schemas/application-detail-schema";
import { ApplicationStatus } from "./application-status";
import { CvLink } from "./cv-link";
import { ScreeningActions } from "./screening-actions";

// Same wording and time zone handling as the Applications list: the deadline is a moment, shown
// in the viewer's own time zone and named to avoid misreading.
const timeFormat = new Intl.DateTimeFormat("en", {
  year: "numeric",
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  timeZoneName: "short",
});

// The label follows the list's wording: an eligible Application shows the time left to interview.
function deadlineLabel(application: ApplicationDetailRecord) {
  if (application.status !== "interview_eligible") return "Interview deadline";
  return application.interview_eligible
    ? "Complete interview by"
    : "Deadline passed";
}

// One Recruiter's Application: the Candidate, the status and CV, and the CV screening decision.
export function ApplicationDetail({
  postingId,
  applicationId,
}: {
  postingId: string;
  applicationId: string;
}) {
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
        <Notice
          role="alert"
          tone="locked"
          title="We couldn't find this application"
        >
          <p className="text-muted-foreground">
            It may have been removed, or it belongs to another recruiter.
          </p>
          <Button asChild variant="outline" className="mt-2 h-11 bg-background">
            <Link href={routes.recruiter.applications}>
              Back to received applications
            </Link>
          </Button>
        </Notice>
      </section>
    );
  if (denied)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <Notice
          role="alert"
          tone="locked"
          title="You don't have access to this application"
        >
          <p className="text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            recruiter account.
          </p>
        </Notice>
      </section>
    );
  // A failed background refetch keeps the loaded Application and its decision controls on screen.
  if (query.isError && !query.data)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <Notice
          role="alert"
          tone="danger"
          title="We couldn't load this application"
        >
          <p className="text-muted-foreground">Please try again in a moment.</p>
          <Button
            className="mt-2 h-11 bg-background"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </Notice>
      </section>
    );

  const back = (
    <Link
      href={routes.recruiter.jobPosting(postingId)}
      className="inline-flex min-h-11 items-center gap-1 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <ArrowLeft className="size-4" aria-hidden />
      Back to the job posting
    </Link>
  );

  if (query.isPending)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        {back}
        <div role="status" className="space-y-3">
          <Skeleton className="h-10 w-2/3 rounded-md" />
          <Skeleton className="h-48 rounded-xl" />
          <span className="sr-only">Loading the application…</span>
        </div>
      </section>
    );

  const application = query.data;
  if (!application) return null;
  const { candidate, job_posting: posting } = application;
  return (
    <section className="mx-auto w-full max-w-3xl space-y-6">
      {back}
      <header className={cn(headerBandClass, "space-y-2")}>
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Application
        </p>
        <h1 className="text-3xl font-semibold tracking-tight [overflow-wrap:anywhere] sm:text-4xl">
          {candidate.name}
        </h1>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Applied for {posting.title}
        </p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Candidate</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <div className="min-w-0">
              <dt className="text-muted-foreground">Email</dt>
              <dd className="font-medium [overflow-wrap:anywhere]">
                {candidate.email}
              </dd>
            </div>
            <div className="min-w-0">
              <dt className="text-muted-foreground">Job posting</dt>
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
            <h2>Application</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-muted-foreground">Status</dt>
              <dd className="mt-1">
                <ApplicationStatus status={application.status} />
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Submitted</dt>
              <dd className="font-medium tabular-nums">
                <time
                  dateTime={application.submitted_at}
                  title={application.submitted_at}
                >
                  {timeFormat.format(new Date(application.submitted_at))}
                </time>
              </dd>
            </div>
            {application.interview_deadline && (
              <div>
                <dt className="text-muted-foreground">
                  {deadlineLabel(application)}
                </dt>
                <dd className="font-medium tabular-nums">
                  <time
                    dateTime={application.interview_deadline}
                    title={application.interview_deadline}
                  >
                    {timeFormat.format(
                      new Date(application.interview_deadline),
                    )}
                  </time>
                </dd>
              </div>
            )}
            {application.reject_reason && (
              <div className="sm:col-span-2">
                <dt className="text-muted-foreground">Reject reason</dt>
                <dd className="font-medium [overflow-wrap:anywhere]">
                  {application.reject_reason}
                </dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>CV</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CvLink
            postingId={postingId}
            applicationId={applicationId}
            cv={application.cv}
          />
        </CardContent>
      </Card>

      <ScreeningActions
        postingId={postingId}
        applicationId={applicationId}
        application={application}
      />
    </section>
  );
}
