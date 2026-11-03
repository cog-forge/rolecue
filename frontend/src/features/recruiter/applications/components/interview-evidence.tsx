"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  applicationKeys,
  getApplicationRecording,
  getApplicationTranscript,
} from "../api/applications";
import type { ApplicationInterview } from "../schemas/application-detail-schema";
import { groupByQuestion } from "@/features/report/utils/question-groups";
import { formatDeadline } from "../utils/application-dates";
import { useApplicationErrorMessage } from "../utils/application-errors";
import { useFormatters } from "@/lib/i18n/use-formatters";

// Recording and transcript are evidence: nothing is fetched until the Recruiter asks, and no media
// starts on its own. Retention (decided 2026-11-03): a recording is kept for 90 days after the
// Application's final decision, then the System Handler deletes it and its status reads "deleted"
// (KAN-272). The Candidate sees that rule in the recording disclosure. Admin access to recordings is
// still an open product gate; this component does not decide it.

export function InterviewEvidence({
  postingId,
  applicationId,
  interview,
}: {
  postingId: string;
  applicationId: string;
  interview: ApplicationInterview;
}) {
  const t = useTranslations("recruiter.applications.evidence");
  return (
    <div className="space-y-6 border-t pt-6">
      <section aria-labelledby="interview-recording" className="space-y-3">
        <h3 id="interview-recording" className="text-sm font-semibold">
          {t("recording")}
        </h3>
        <p className="text-sm text-muted-foreground">
          {t(`recordStatus.${interview.record_status}`)}
        </p>
        {interview.record_status === "ready" && (
          <RecordingLink postingId={postingId} applicationId={applicationId} />
        )}
      </section>

      <section aria-labelledby="interview-transcript" className="space-y-3">
        <h3 id="interview-transcript" className="text-sm font-semibold">
          {t("transcript")}
        </h3>
        {interview.completed_at === null ? (
          <p className="text-sm text-muted-foreground">
            {t("transcriptLocked")}
          </p>
        ) : (
          <TranscriptToggle
            postingId={postingId}
            applicationId={applicationId}
          />
        )}
      </section>
    </div>
  );
}

// The read link expires after a few minutes, so it is requested each time the Recruiter asks.
function RecordingLink({
  postingId,
  applicationId,
}: {
  postingId: string;
  applicationId: string;
}) {
  const t = useTranslations("recruiter.applications.evidence");
  const [requested, setRequested] = useState(false);
  if (!requested)
    return (
      <Button
        variant="outline"
        className="h-11 bg-background"
        onClick={() => setRequested(true)}
      >
        {t("getLink")}
      </Button>
    );
  return (
    <RecordingLoader postingId={postingId} applicationId={applicationId} />
  );
}

function RecordingLoader({
  postingId,
  applicationId,
}: {
  postingId: string;
  applicationId: string;
}) {
  const t = useTranslations("recruiter.applications.evidence");
  const tCommon = useTranslations("common");
  const errorMessage = useApplicationErrorMessage();
  const fmt = useFormatters();
  const user = useSessionUser();
  const query = useQuery({
    queryKey: applicationKeys.recording(user.id, postingId, applicationId),
    queryFn: ({ signal }) =>
      getApplicationRecording(postingId, applicationId, signal),
    refetchOnMount: "always",
    retry: false,
  });

  if (query.isPending)
    return (
      <p role="status" className="text-sm text-muted-foreground">
        {t("gettingLink")}
      </p>
    );
  if (query.isError)
    return (
      <p role="alert" className="text-sm text-destructive">
        {errorMessage(query.error, t("linkFailed"))}
      </p>
    );
  const recording = query.data;
  if (!recording.url)
    return (
      <p className="text-sm text-muted-foreground">{t("linkUnavailable")}</p>
    );
  return (
    <div className="space-y-2">
      <Button asChild variant="outline" className="h-11 bg-background">
        <a href={recording.url} target="_blank" rel="noopener noreferrer">
          <ExternalLink aria-hidden />
          {t("openRecording")}
          <span className="sr-only"> {tCommon("labels.opensInNewTab")}</span>
        </a>
      </Button>
      {recording.expires_at && (
        <p className="text-xs text-muted-foreground">
          {t("linkExpires", {
            date: formatDeadline(recording.expires_at, fmt),
          })}
        </p>
      )}
    </div>
  );
}

function TranscriptToggle({
  postingId,
  applicationId,
}: {
  postingId: string;
  applicationId: string;
}) {
  const t = useTranslations("recruiter.applications.evidence");
  const [open, setOpen] = useState(false);
  return (
    <div className="space-y-3">
      <Button
        variant="outline"
        className="h-11 bg-background"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        {open ? t("hideTranscript") : t("showTranscript")}
      </Button>
      {open && (
        <TranscriptBody postingId={postingId} applicationId={applicationId} />
      )}
    </div>
  );
}

// Mounted only while open, so each opening reads the transcript from the server.
function TranscriptBody({
  postingId,
  applicationId,
}: {
  postingId: string;
  applicationId: string;
}) {
  const t = useTranslations("recruiter.applications.evidence");
  const errorMessage = useApplicationErrorMessage();
  const user = useSessionUser();
  const query = useQuery({
    queryKey: applicationKeys.transcript(user.id, postingId, applicationId),
    queryFn: ({ signal }) =>
      getApplicationTranscript(postingId, applicationId, signal),
    refetchOnMount: "always",
    retry: false,
  });

  if (query.isPending)
    return (
      <p role="status" className="text-sm text-muted-foreground">
        {t("transcriptLoading")}
      </p>
    );
  if (query.isError)
    return (
      <p role="alert" className="text-sm text-destructive">
        {errorMessage(query.error, t("transcriptFailed"))}
      </p>
    );
  const turns = query.data.turns;
  if (turns.length === 0)
    return <p className="text-sm text-muted-foreground">{t("noQuestions")}</p>;
  return (
    <ol className="space-y-4 text-sm">
      {groupByQuestion(turns).map(({ core, followUps }) => (
        <li key={core.position} className="space-y-1 border-l-2 pl-4">
          <p className="text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
            {t("question", { position: core.question_number })}
          </p>
          <p className="font-medium [overflow-wrap:anywhere]">
            {core.question}
          </p>
          <p className="text-muted-foreground [overflow-wrap:anywhere]">
            {core.answer ?? t("noAnswer")}
          </p>
          {followUps.length > 0 && (
            <ol className="mt-3 space-y-3 pl-4">
              {followUps.map((turn) => (
                <li key={turn.position} className="space-y-1">
                  <p className="text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                    {t("followUp")}
                  </p>
                  <p className="font-medium [overflow-wrap:anywhere]">
                    {turn.question}
                  </p>
                  <p className="text-muted-foreground [overflow-wrap:anywhere]">
                    {turn.answer ?? t("noAnswer")}
                  </p>
                </li>
              ))}
            </ol>
          )}
        </li>
      ))}
    </ol>
  );
}
