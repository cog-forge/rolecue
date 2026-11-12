"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { applicationKeys, getInterviewReport } from "../api/applications";
import type {
  ApplicationInterview,
  InterviewStatus,
} from "../schemas/application-detail-schema";
import { formatDeadline } from "../utils/application-dates";
import { InterviewEvidence } from "./interview-evidence";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  GradedQuestionList,
  ScoringSummary,
} from "@/features/report/components/report-scoring";
import { formatScore } from "@/features/report/utils/scoring";

// The Candidate's interview result. The score and status come with the detail; the evaluation and how its
// overall was calculated (rubric criteria per question, session metrics and weights, KAN-337) are read
// from the report only when the Recruiter asks for it.
export function InterviewResult({
  postingId,
  applicationId,
  interview,
}: {
  postingId: string;
  applicationId: string;
  interview: ApplicationInterview;
}) {
  const t = useTranslations("recruiter.applications");
  const fmt = useFormatters();
  const [open, setOpen] = useState(false);
  const finished = interview.status !== "in_progress";

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("interview.heading")}</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <dl className="grid gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-muted-foreground">{t("facts.status")}</dt>
            <dd className="mt-1 font-medium">
              {t(`interview.status.${interview.status}`)}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t("interview.score")}</dt>
            <dd className="mt-1 font-medium tabular-nums">
              {interview.score !== null
                ? t("interview.scoreValue", {
                    score: formatScore(interview.score, fmt.number),
                  })
                : interview.score_outcome === "withheld"
                  ? t("interview.scoreWithheld")
                  : t("interview.notScored")}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t("interview.started")}</dt>
            <dd className="font-medium tabular-nums">
              <time
                dateTime={interview.started_at}
                title={interview.started_at}
              >
                {formatDeadline(interview.started_at, fmt)}
              </time>
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {t("interview.completed")}
            </dt>
            <dd className="font-medium tabular-nums">
              {interview.completed_at ? (
                <time
                  dateTime={interview.completed_at}
                  title={interview.completed_at}
                >
                  {formatDeadline(interview.completed_at, fmt)}
                </time>
              ) : (
                t("interview.notCompleted")
              )}
            </dd>
          </div>
          {interview.end_reason ? (
            <div>
              <dt className="text-muted-foreground">{t("interview.ended")}</dt>
              <dd className="mt-1 font-medium">
                {t(`interview.endReason.${interview.end_reason}`)}
              </dd>
            </div>
          ) : null}
        </dl>

        {finished ? (
          <div className="space-y-3">
            <Button
              variant="outline"
              className="h-11 bg-background"
              aria-expanded={open}
              onClick={() => setOpen((value) => !value)}
            >
              {open
                ? t("interview.hideEvaluation")
                : t("interview.showEvaluation")}
            </Button>
            {open && (
              <EvaluationBreakdown
                interviewId={interview.id}
                status={interview.status}
              />
            )}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            {t("interview.notFinished")}
          </p>
        )}

        <InterviewEvidence
          postingId={postingId}
          applicationId={applicationId}
          interview={interview}
        />
      </CardContent>
    </Card>
  );
}

// Mounted only while open, so each opening reads the report again (the evaluation may have finished).
function EvaluationBreakdown({
  interviewId,
  status,
}: {
  interviewId: string;
  status: InterviewStatus;
}) {
  const t = useTranslations("recruiter.applications.interview");
  const tCommon = useTranslations("common");
  const user = useSessionUser();
  const query = useQuery({
    queryKey: applicationKeys.report(user.id, interviewId),
    queryFn: ({ signal }) => getInterviewReport(interviewId, signal),
    refetchOnMount: "always",
    retry: false,
  });

  if (query.isPending)
    return (
      <p role="status" className="text-sm text-muted-foreground">
        {t("evaluationLoading")}
      </p>
    );
  if (query.isError)
    return (
      <div className="space-y-2">
        <p role="alert" className="text-sm text-destructive">
          {t("evaluationFailed")}
        </p>
        <Button
          variant="outline"
          className="h-11 bg-background"
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          {tCommon("actions.tryAgain")}
        </Button>
      </div>
    );

  const report = query.data;
  return (
    <div className="space-y-4 rounded-xl border p-4 text-sm">
      {report.injection_flagged && (
        <div
          role="note"
          data-testid="injection-flagged"
          className="rounded-lg border border-destructive/40 bg-destructive/5 p-3"
        >
          <p className="font-medium text-destructive">{t("flaggedTitle")}</p>
          <p className="mt-1 text-muted-foreground">{t("flaggedText")}</p>
        </div>
      )}
      {report.summary && (
        <div className="space-y-1">
          <p className="text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
            {t("aiGenerated")}
          </p>
          <p className="leading-relaxed [overflow-wrap:anywhere]">
            {report.summary}
          </p>
        </div>
      )}
      {report.scoring ? (
        <>
          <ScoringSummary report={report} />
          <GradedQuestionList
            graded={report.graded_questions ?? []}
            questions={report.questions}
          />
        </>
      ) : (
        <div className="space-y-2">
          <p className="text-muted-foreground">
            {status === "failed" ? t("noBreakdownFailed") : t("noBreakdownYet")}
          </p>
          {status !== "failed" && (
            <Button
              variant="outline"
              className="h-11 bg-background"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              {t("checkAgain")}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
