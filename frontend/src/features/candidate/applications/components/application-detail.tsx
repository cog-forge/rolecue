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
import { CompanyInterviewer } from "./company-interviewer";
import { ApplicationProgress } from "./application-progress";
import { CandidateApplicationStatusBadge } from "./application-status";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { useTranslations } from "next-intl";

// One Application for the Candidate who submitted it. It never shows a transcript, audio or video.
export function CandidateApplicationDetail({ id }: { id: string }) {
  const t = useTranslations("candidate.applications.detail");
  const tActions = useTranslations("candidate.applications.actions");
  const user = useSessionUser();
  const cache = useQueryClient();
  const validId = z.uuid().safeParse(id).success;
  const query = useQuery({
    queryKey: candidateApplicationKeys.detail(user.id, id),
    queryFn: ({ signal }) => getCandidateApplication(id, signal),
    enabled: validId,
    // Status and the deadline move on the recruiter's side, so always ask again.
    refetchOnMount: "always",
    refetchInterval: 30_000,
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
        {t("back")}
      </Link>

      {!validId ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">{t("invalidTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("invalidBody")}
          </p>
        </div>
      ) : query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-48 rounded-xl" />
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border bg-muted/60 p-6">
          <h1 className="text-xl font-semibold">{t("deniedTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("deniedBody")}
          </p>
        </div>
      ) : notFound ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">{t("notFoundTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("notFoundBody")}
          </p>
          <Button
            asChild
            variant="outline"
            className="mt-4 h-auto min-h-11 whitespace-normal py-2"
          >
            <Link href={routes.applications}>{t("back")}</Link>
          </Button>
        </div>
      ) : query.isError ? (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <h1 className="text-xl font-semibold">{t("errorTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("errorBody")}</p>
          <Button
            className="mt-4 h-11 bg-background"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            {tActions("tryAgain")}
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
  const t = useTranslations("candidate.applications.detail");
  const tActions = useTranslations("candidate.applications.actions");
  const now = useNow();
  const user = useSessionUser();
  const tInterviewer = useTranslations("candidate.applications.interviewer");
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
          {t("eyebrow")}
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
            <Fact label={t("facts.status")}>
              <CandidateApplicationStatusBadge status={status} />
            </Fact>
            <Fact label={t("facts.submitted")}>
              <time dateTime={application.created_at}>
                {formatSubmitted(application.created_at, fmt)}
              </time>
            </Fact>
            {deadline && (
              <Fact label={t("facts.deadline")}>
                <time dateTime={deadline}>{formatDeadline(deadline, fmt)}</time>
              </Fact>
            )}
            {application.interview && (
              <Fact label={t("facts.result")}>
                {interviewResultText(application.interview, t)}
              </Fact>
            )}
            {application.can_start_interview && deadline && (
              <Fact label={t("facts.timeLeft")}>
                <span className="tabular-nums">
                  {timeLeftText(deadline, now, fmt)}
                </span>
              </Fact>
            )}
          </dl>

          <StatusMessage
            status={status}
            passed={passed}
            reason={reason}
            t={t}
          />
        </CardContent>
      </Card>

      {application.blocker && (
        <p role="status" className="text-sm text-muted-foreground">
          {tInterviewer(`blockers.${application.blocker}`)}
        </p>
      )}
      {application.can_start_interview && !passed && (
        <CompanyInterviewer userId={user.id} applicationId={application.id} />
      )}
      {application.can_start_interview && !passed && (
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
              {resuming ? tActions("resume") : tActions("start")}
              <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
          <p
            id="start-interview-note"
            className="text-sm text-muted-foreground"
          >
            {resuming ? t("resumeNote") : t("startNote")}
          </p>
        </div>
      )}
    </div>
  );
}

type DetailTranslator = ReturnType<
  typeof useTranslations<"candidate.applications.detail">
>;

function StatusMessage({
  status,
  passed,
  reason,
  t,
}: {
  status: CandidateApplication["status"];
  passed: boolean;
  reason: string | null;
  t: DetailTranslator;
}) {
  let text: string;
  if (status === "pending") text = t("message.pending");
  else if (status === "interview_eligible")
    text = passed ? t("message.eligiblePassed") : t("message.eligible");
  else if (status === "interviewed") text = t("message.interviewed");
  else if (status === "approved") text = t("message.approved");
  else text = t("message.rejected");

  return (
    <div className="space-y-2 border-t pt-4 text-sm">
      <p role="status">{text}</p>
      {status === "rejected" && reason && (
        <Note>{t("reason", { reason })}</Note>
      )}
    </div>
  );
}

// Status and score only: the transcript and recording are never shown to the Candidate.
function interviewResultText(
  interview: NonNullable<CandidateApplication["interview"]>,
  t: DetailTranslator,
): string {
  if (interview.status === "completed" && interview.score !== null)
    return t("result.score", { score: interview.score });
  if (interview.status === "failed") return t("result.unavailable");
  if (interview.status === "pending") return t("result.evaluating");
  return t("result.inProgress");
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
