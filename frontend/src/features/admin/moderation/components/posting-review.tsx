"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CircleAlert } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  approvePosting,
  getPosting,
  moderationKeys,
  rejectPosting,
} from "../api/moderation";
import type { ReviewPosting } from "../schemas/moderation-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { DecisionDialog, type Decision } from "./decision-dialog";
import { JdViewer } from "./jd-viewer";
import { PostingStatus } from "./posting-status";

const dateFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "UTC",
});
const numberFormat = new Intl.NumberFormat("en");

// Units and currency aren't defined in the schema yet, so numbers are shown as stored.
function salary(posting: ReviewPosting) {
  const { salary_min: min, salary_max: max } = posting;
  if (min === null && max === null) return null;
  if (min !== null && max !== null)
    return `${numberFormat.format(min)} – ${numberFormat.format(max)}`;
  return min !== null
    ? `From ${numberFormat.format(min)}`
    : `Up to ${numberFormat.format(max!)}`;
}

export function PostingReview({
  id,
  backQuery,
}: {
  id: string;
  backQuery: string;
}) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = moderationKeys.detail(user.id, id);
  const [decision, setDecision] = useState<Decision | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getPosting(id, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const decide = useMutation({
    mutationFn: (input: { decision: Decision; reason?: string }) =>
      input.decision === "approve"
        ? approvePosting(id)
        : rejectPosting(id, input.reason ?? ""),
    retry: false,
    onSuccess: async (posting, input) => {
      await cache.cancelQueries({ queryKey: key });
      cache.setQueryData(key, posting);
      void cache.invalidateQueries({ queryKey: moderationKeys.lists(user.id) });
      setDecision(null);
      setNotice(null);
      gooeyToast.success(
        input.decision === "approve" ? "Posting approved" : "Posting rejected",
        {
          description:
            input.decision === "approve"
              ? "It's now published to candidates."
              : "The recruiter can see your reason.",
        },
      );
    },
    onError: (error) => {
      const { status } = apiError(error);
      if (status === 409) {
        // Someone else decided first, or the bank is empty: show why and reload the truth.
        setDecision(null);
        setNotice(
          apiError(error).message ??
            "This posting can't be decided right now. We refreshed it for you.",
        );
        void cache.invalidateQueries({ queryKey: key });
        void cache.invalidateQueries({ queryKey: moderationKeys.lists(user.id) });
      }
    },
  });
  const denied = isAccessDenied(query.error) || isAccessDenied(decide.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const backHref = `${routes.admin.jobPostings}${backQuery ? `?${backQuery}` : ""}`;
  const dialogError =
    decision && decide.isError && apiError(decide.error).status !== 409
      ? (apiError(decide.error).message ??
        "We couldn't save your decision. Please try again.")
      : null;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <Link
        href={backHref}
        className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Back to moderation queue
      </Link>

      {query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-48 rounded-xl" />
          <Skeleton className="h-96 rounded-xl" />
          <span className="sr-only">Loading the job posting…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">You don&apos;t have access to this posting</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with an
            administrator account.
          </p>
        </div>
      ) : query.isError ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">
            {apiError(query.error).status === 404
              ? "This job posting doesn't exist"
              : "We couldn't load this job posting"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {apiError(query.error).status === 404
              ? "It may have been removed. Go back to the queue to pick another one."
              : "Please try again in a moment."}
          </p>
          {apiError(query.error).status !== 404 && (
            <Button
              className="mt-4 h-11"
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              Try again
            </Button>
          )}
        </div>
      ) : (
        <>
          <header className="space-y-2">
            <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
              Moderation review
            </p>
            <h1 className="text-3xl font-semibold tracking-tight [overflow-wrap:anywhere] sm:text-4xl">
              {query.data.title}
            </h1>
            <PostingStatus status={query.data.status} />
          </header>

          {notice && (
            <p
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-destructive/30 p-3 text-sm"
            >
              <CircleAlert className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden />
              {notice}
            </p>
          )}

          <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
            <Card>
              <CardContent className="pt-6">
                <JdViewer
                  contentPath={query.data.content_path}
                  documentUrl={query.data.document_url}
                  rawText={query.data.raw_text}
                />
              </CardContent>
            </Card>
            <div className="space-y-6">
              <DecisionPanel
                posting={query.data}
                onDecide={(next) => {
                  decide.reset();
                  setDecision(next);
                }}
              />
              <DetailsCard posting={query.data} />
            </div>
          </div>

          <DecisionDialog
            decision={decision}
            title={query.data.title}
            pending={decide.isPending}
            error={dialogError}
            onCancel={() => setDecision(null)}
            onApprove={() => decide.mutate({ decision: "approve" })}
            onReject={(reason) => decide.mutate({ decision: "reject", reason })}
          />
        </>
      )}
    </section>
  );
}

function DecisionPanel({
  posting,
  onDecide,
}: {
  posting: ReviewPosting;
  onDecide: (decision: Decision) => void;
}) {
  const count = posting.core_question_count;
  const noQuestions = count === 0;
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>Decision</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm">
          <span className="font-medium tabular-nums">
            {numberFormat.format(count)}
          </span>{" "}
          {count === 1 ? "core question" : "core questions"} in the question bank
        </p>
        {posting.status !== "pending" ? (
          <p className="text-sm text-muted-foreground">
            This posting has already been decided.
            {posting.status === "rejected" && posting.reject_reason && (
              <>
                {" "}
                Reason given to the recruiter:{" "}
                <span className="text-foreground [overflow-wrap:anywhere]">
                  “{posting.reject_reason}”
                </span>
              </>
            )}
          </p>
        ) : (
          <>
            {noQuestions && (
              <p id="approve-blocked" className="text-sm text-muted-foreground">
                The recruiter must add core questions before this posting can
                be approved.
              </p>
            )}
            <div className="flex flex-col gap-2">
              <Button
                className="h-11"
                disabled={noQuestions}
                aria-describedby={noQuestions ? "approve-blocked" : undefined}
                onClick={() => onDecide("approve")}
              >
                Approve
              </Button>
              <Button
                variant="outline"
                className="h-11"
                onClick={() => onDecide("reject")}
              >
                Reject
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

function DetailsCard({ posting }: { posting: ReviewPosting }) {
  const rows: [string, ReactNode][] = [
    [
      "Recruiter",
      <>
        {posting.recruiter.name}
        <span className="block text-xs text-muted-foreground [overflow-wrap:anywhere]">
          {posting.recruiter.email}
        </span>
      </>,
    ],
    [
      "Submitted",
      <time key="t" dateTime={posting.created_at}>
        {dateFormat.format(new Date(posting.created_at))} UTC
      </time>,
    ],
    ["Location", posting.location],
    ["Employment type", posting.employment_type],
    ["Salary", salary(posting)],
    [
      "Interview slots",
      posting.interview_slot === null
        ? null
        : numberFormat.format(posting.interview_slot),
    ],
    [
      "Interview duration",
      posting.interview_duration === null
        ? null
        : numberFormat.format(posting.interview_duration),
    ],
    ["3D interviewer", posting.required_avatar?.name ?? null],
    ["Voice profile", posting.required_voice?.name ?? null],
  ];
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>Posting details</h2>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="divide-y text-sm">
          {rows.map(([label, value]) => (
            <div key={label} className="grid grid-cols-[8.5rem_minmax(0,1fr)] gap-3 py-2.5">
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="[overflow-wrap:anywhere]">
                {value ?? <span className="text-muted-foreground">Not set</span>}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}
