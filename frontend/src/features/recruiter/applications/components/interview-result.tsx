"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
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

const interviewStatusText: Record<InterviewStatus, string> = {
  in_progress: "In progress",
  pending: "Awaiting evaluation",
  completed: "Evaluated",
  failed: "Evaluation failed",
};

// The Candidate's interview result. The score and status come with the detail; the evaluation
// breakdown is read from the report only when the Recruiter asks for it.
export function InterviewResult({
  postingId,
  applicationId,
  interview,
}: {
  postingId: string;
  applicationId: string;
  interview: ApplicationInterview;
}) {
  const [open, setOpen] = useState(false);
  const finished = interview.status !== "in_progress";

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>Interview</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <dl className="grid gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-muted-foreground">Status</dt>
            <dd className="mt-1 font-medium">
              {interviewStatusText[interview.status]}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Score</dt>
            <dd className="mt-1 font-medium tabular-nums">
              {interview.score === null
                ? "Not scored yet"
                : `${interview.score}/100`}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Started</dt>
            <dd className="font-medium tabular-nums">
              <time
                dateTime={interview.started_at}
                title={interview.started_at}
              >
                {formatDeadline(interview.started_at)}
              </time>
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Completed</dt>
            <dd className="font-medium tabular-nums">
              {interview.completed_at ? (
                <time
                  dateTime={interview.completed_at}
                  title={interview.completed_at}
                >
                  {formatDeadline(interview.completed_at)}
                </time>
              ) : (
                "Not completed yet"
              )}
            </dd>
          </div>
        </dl>

        {finished ? (
          <div className="space-y-3">
            <Button
              variant="outline"
              className="h-11 bg-background"
              aria-expanded={open}
              onClick={() => setOpen((value) => !value)}
            >
              {open ? "Hide evaluation" : "Show evaluation"}
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
            The Candidate has not finished the interview yet.
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
        Loading the evaluation…
      </p>
    );
  if (query.isError)
    return (
      <div className="space-y-2">
        <p role="alert" className="text-sm text-destructive">
          We couldn&apos;t load the evaluation. Try again.
        </p>
        <Button
          variant="outline"
          className="h-11 bg-background"
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          Try again
        </Button>
      </div>
    );

  const report = query.data;
  const breakdown = report.breakdown;
  return (
    <div className="space-y-4 rounded-xl border p-4 text-sm">
      {report.summary && (
        <p className="leading-relaxed [overflow-wrap:anywhere]">
          {report.summary}
        </p>
      )}
      {breakdown && breakdown.length > 0 ? (
        <dl className="grid gap-3 sm:grid-cols-2">
          {breakdown.map((metric) => (
            <div key={metric.metric_id} className="min-w-0">
              <dt className="text-muted-foreground [overflow-wrap:anywhere]">
                {metric.name}
              </dt>
              <dd className="font-medium tabular-nums">{metric.score}/100</dd>
            </div>
          ))}
        </dl>
      ) : (
        <div className="space-y-2">
          <p className="text-muted-foreground">
            {status === "failed"
              ? "The evaluation failed, so there is no breakdown."
              : "The breakdown is not ready yet."}
          </p>
          {status !== "failed" && (
            <Button
              variant="outline"
              className="h-11 bg-background"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              Check again
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
