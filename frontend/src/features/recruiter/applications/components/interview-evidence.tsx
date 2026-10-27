"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  applicationKeys,
  getApplicationRecording,
  getApplicationTranscript,
} from "../api/applications";
import type {
  ApplicationInterview,
  RecordStatus,
} from "../schemas/application-detail-schema";
import { formatDeadline } from "../utils/application-dates";
import { applicationErrorMessage } from "../utils/application-errors";

// Recording and transcript are evidence: nothing is fetched until the Recruiter asks, and no media
// starts on its own. Retention, consent and Admin access to recordings are open product gates; this
// component does not decide them.
const recordText: Record<RecordStatus, string> = {
  none: "No recording was uploaded for this interview.",
  uploading: "The recording is still being uploaded.",
  processing: "The recording is still being processed.",
  ready: "A recording is available.",
  failed: "The recording could not be stored.",
};

export function InterviewEvidence({
  postingId,
  applicationId,
  interview,
}: {
  postingId: string;
  applicationId: string;
  interview: ApplicationInterview;
}) {
  return (
    <div className="space-y-6 border-t pt-6">
      <section aria-labelledby="interview-recording" className="space-y-3">
        <h3 id="interview-recording" className="text-sm font-semibold">
          Recording
        </h3>
        <p className="text-sm text-muted-foreground">
          {recordText[interview.record_status]}
        </p>
        {interview.record_status === "ready" && (
          <RecordingLink postingId={postingId} applicationId={applicationId} />
        )}
      </section>

      <section aria-labelledby="interview-transcript" className="space-y-3">
        <h3 id="interview-transcript" className="text-sm font-semibold">
          Transcript
        </h3>
        {interview.completed_at === null ? (
          <p className="text-sm text-muted-foreground">
            The transcript is available once the interview is completed.
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
  const [requested, setRequested] = useState(false);
  if (!requested)
    return (
      <Button
        variant="outline"
        className="h-11 bg-background"
        onClick={() => setRequested(true)}
      >
        Get the recording link
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
        Getting the recording link…
      </p>
    );
  if (query.isError)
    return (
      <p role="alert" className="text-sm text-destructive">
        {applicationErrorMessage(
          query.error,
          "We couldn't get the recording. Try again.",
        )}
      </p>
    );
  const recording = query.data;
  if (!recording.url)
    return (
      <p className="text-sm text-muted-foreground">
        The recording link is not available right now.
      </p>
    );
  return (
    <div className="space-y-2">
      <Button asChild variant="outline" className="h-11 bg-background">
        <a href={recording.url} target="_blank" rel="noopener noreferrer">
          <ExternalLink aria-hidden />
          Open the recording
          <span className="sr-only"> (opens in a new tab)</span>
        </a>
      </Button>
      {recording.expires_at && (
        <p className="text-xs text-muted-foreground">
          The link expires {formatDeadline(recording.expires_at)}.
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
  const [open, setOpen] = useState(false);
  return (
    <div className="space-y-3">
      <Button
        variant="outline"
        className="h-11 bg-background"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        {open ? "Hide transcript" : "Show transcript"}
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
        Loading the transcript…
      </p>
    );
  if (query.isError)
    return (
      <p role="alert" className="text-sm text-destructive">
        {applicationErrorMessage(
          query.error,
          "We couldn't load the transcript. Try again.",
        )}
      </p>
    );
  const turns = query.data.turns;
  if (turns.length === 0)
    return (
      <p className="text-sm text-muted-foreground">
        No questions were recorded.
      </p>
    );
  return (
    <ol className="space-y-4 text-sm">
      {turns.map((turn) => (
        <li key={turn.position} className="space-y-1 border-l-2 pl-4">
          <p className="text-xs font-semibold tracking-[0.14em] text-muted-foreground uppercase">
            Question {turn.position}
          </p>
          <p className="font-medium [overflow-wrap:anywhere]">
            {turn.question}
          </p>
          <p className="text-muted-foreground [overflow-wrap:anywhere]">
            {turn.answer ?? "No answer was recorded."}
          </p>
        </li>
      ))}
    </ol>
  );
}
