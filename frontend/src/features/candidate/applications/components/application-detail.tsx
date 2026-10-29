"use client";

import { useEffect, type ReactNode } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  candidateApplicationKeys,
  getCandidateApplication,
} from "../api/applications";
import type { CandidateApplication } from "../schemas/candidate-application-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import {
  formatDeadline,
  formatSubmitted,
  isDeadlinePassed,
  timeLeftText,
} from "../utils/application-text";
import { useNow } from "../utils/use-now";
import { ApplicationProgress } from "./application-progress";
import { CandidateApplicationStatusBadge } from "./application-status";
import { useFormatters } from "@/lib/i18n/use-formatters";

// One Application for the Candidate who submitted it. It never shows a transcript, audio or video.
export function CandidateApplicationDetail({ id }: { id: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const validId = z.uuid().safeParse(id).success;
  const query = useQuery({
    queryKey: candidateApplicationKeys.detail(user.id, id),
    queryFn: ({ signal }) => getCandidateApplication(id, signal),
    enabled: validId,
    // Status and the deadline move on the recruiter's side, so always ask again.
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);
  const notFound = apiError(query.error).status === 404;

  return (
    <section className="mx-auto w-full max-w-3xl space-y-6">
      <Link
        href={routes.applications}
        className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Back to my applications
      </Link>

      {!validId ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">This link isn&apos;t valid</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Check the address, or open the application from your list.
          </p>
        </div>
      ) : query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-48 rounded-xl" />
          <span className="sr-only">Loading your application…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border bg-muted/60 p-6">
          <h1 className="text-xl font-semibold">
            You don&apos;t have access to this application
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            candidate account.
          </p>
        </div>
      ) : notFound ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">
            We couldn&apos;t find this application
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            It may have been removed, or it isn&apos;t yours.
          </p>
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={routes.applications}>Back to my applications</Link>
          </Button>
        </div>
      ) : query.isError ? (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <h1 className="text-xl font-semibold">
            We couldn&apos;t load this application
          </h1>
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
      ) : (
        <ApplicationContent application={query.data} />
      )}
    </section>
  );
}

function ApplicationContent({
  application,
}: {
  application: CandidateApplication;
}) {
  const fmt = useFormatters();
  const now = useNow();
  const {
    status,
    interview_deadline: deadline,
    reject_reason: reason,
  } = application;
  const resuming = application.interview?.status === "in_progress";
  const passed =
    status === "interview_eligible" &&
    deadline !== null &&
    isDeadlinePassed(deadline, now);

  return (
    <div className="space-y-6 duration-300 animate-in fade-in motion-reduce:animate-none">
      <header className="space-y-3">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Application
        </p>
        <h1 className="text-3xl font-semibold tracking-tight [overflow-wrap:anywhere] sm:text-4xl">
          {application.job_posting_title}
        </h1>
        {application.company_name && (
          <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
            {[application.company_name, application.job_posting_location]
              .filter(Boolean)
              .join(" · ")}
          </p>
        )}
        <ApplicationProgress application={application} className="pt-2" />
      </header>

      <Card>
        <CardContent className="space-y-5">
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <Fact label="Status">
              <CandidateApplicationStatusBadge status={status} />
            </Fact>
            <Fact label="Submitted">
              <time dateTime={application.created_at}>
                {formatSubmitted(application.created_at, fmt)}
              </time>
            </Fact>
            {deadline && (
              <Fact label="Interview deadline">
                <time dateTime={deadline}>{formatDeadline(deadline, fmt)}</time>
              </Fact>
            )}
            {application.interview && (
              <Fact label="Interview result">
                {interviewResultText(application.interview)}
              </Fact>
            )}
            {application.can_start_interview && deadline && (
              <Fact label="Time left">
                <span className="tabular-nums">
                  {timeLeftText(deadline, now, fmt)}
                </span>
              </Fact>
            )}
          </dl>

          <StatusMessage status={status} passed={passed} reason={reason} />
        </CardContent>
      </Card>

      {application.can_start_interview && (
        <div className="space-y-3">
          {/* Navigation only: the start (or resume) happens after the microphone check. */}
          <Button
            asChild
            className="h-11"
            aria-describedby="start-interview-note"
          >
            <Link
              href={`${routes.interviews.new.preflight}?applicationId=${encodeURIComponent(application.id)}`}
            >
              {resuming ? "Resume interview" : "Start interview"}
              <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
          <p
            id="start-interview-note"
            className="text-sm text-muted-foreground"
          >
            {resuming
              ? "Pick up where you left off. Resuming uses no new interview slot."
              : "Free for you. Complete it before the deadline."}
          </p>
        </div>
      )}
    </div>
  );
}

function StatusMessage({
  status,
  passed,
  reason,
}: {
  status: CandidateApplication["status"];
  passed: boolean;
  reason: string | null;
}) {
  let text: string;
  if (status === "pending")
    text =
      "Your CV is waiting for review. This page updates when the recruiter decides.";
  else if (status === "interview_eligible")
    text = passed
      ? "Deadline passed. The interview window for this application has closed."
      : "Your CV was approved. Complete your interview before the deadline.";
  else if (status === "interviewed")
    text = "Your interview is complete. The recruiter will review it next.";
  else if (status === "approved")
    text = "The recruiter approved your application.";
  else text = "This application was not selected for this role.";

  return (
    <div className="space-y-2 border-t pt-4 text-sm">
      <p role="status">{text}</p>
      {status === "rejected" && reason && <Note>Reason: {reason}</Note>}
    </div>
  );
}

// Status and score only: the transcript and recording are never shown to the Candidate.
function interviewResultText(
  interview: NonNullable<CandidateApplication["interview"]>,
): string {
  if (interview.status === "completed" && interview.score !== null)
    return `${interview.score} / 100`;
  if (interview.status === "failed") return "Not available";
  if (interview.status === "pending") return "Being evaluated";
  return "In progress";
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-medium [overflow-wrap:anywhere]">{children}</dd>
    </div>
  );
}

function Note({ children }: { children: ReactNode }) {
  return (
    <p className="text-muted-foreground [overflow-wrap:anywhere]">{children}</p>
  );
}
