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
import type { Formatters } from "@/lib/i18n/formatters";
import { useFormatters } from "@/lib/i18n/use-formatters";

// Units and currency aren't defined in the schema yet, so numbers are shown as stored.
function salary(posting: ReviewPosting, fmt: Formatters) {
  const { salary_min: min, salary_max: max } = posting;
  if (min === null && max === null) return null;
  if (min !== null && max !== null)
    return `${fmt.number(min)} – ${fmt.number(max)}`;
  return min !== null ? `From ${fmt.number(min)}` : `Up to ${fmt.number(max!)}`;
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
              ? "Candidates can see it once the recruiter funds interview slots."
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
        void cache.invalidateQueries({
          queryKey: moderationKeys.lists(user.id),
        });
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
          <h1 className="text-xl font-semibold">
            You don&apos;t have access to this posting
          </h1>
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
            <h1 className="text-2xl font-semibold tracking-tight text-balance [overflow-wrap:anywhere] sm:text-[1.75rem] sm:leading-tight">
              {query.data.title}
            </h1>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm text-muted-foreground">
              {/* A pending posting is in the queue only once submitted; before that it is not
                  "Pending review" yet. */}
              {query.data.status === "pending" &&
              !query.data.review_submitted ? (
                <span>Still being prepared by the recruiter</span>
              ) : (
                <PostingStatus status={query.data.status} />
              )}
            </div>
          </header>

          {notice && (
            <p
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-destructive/30 p-3 text-sm"
            >
              <CircleAlert
                className="mt-0.5 size-4 shrink-0 text-destructive"
                aria-hidden
              />
              {notice}
            </p>
          )}

          <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
            <div className="space-y-6">
              <PostingDossier posting={query.data} />
              <Card>
                <CardContent className="pt-6">
                  <JdViewer
                    contentPath={query.data.content_path}
                    documentUrl={query.data.document_url}
                    rawText={query.data.raw_text}
                  />
                </CardContent>
              </Card>
            </div>
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
  const fmt = useFormatters();
  const count = posting.core_question_count;
  // A posting still being prepared cannot be decided; approval also needs questions.
  const notSubmitted = !posting.review_submitted;
  const approveBlocked = notSubmitted || count === 0;
  return (
    <section
      aria-labelledby="decision-heading"
      className="space-y-4 rounded-xl border p-5"
    >
      <h2 id="decision-heading" className="text-base font-semibold">
        Decision
      </h2>
      <div className="space-y-1">
        <p className="text-sm text-muted-foreground">Question bank</p>
        <p className="text-sm">
          <span className="text-2xl font-semibold tabular-nums">
            {fmt.number(count)}
          </span>{" "}
          {count === 1 ? "core question" : "core questions"}
        </p>
      </div>
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
          {approveBlocked && (
            <p id="approve-blocked" className="text-sm text-muted-foreground">
              {notSubmitted
                ? "The recruiter is still preparing this posting and has not submitted it for review."
                : "The recruiter must add core questions before this posting can be approved."}
            </p>
          )}
          <div className="flex flex-col gap-2">
            <Button
              className="h-10 rounded-full"
              disabled={approveBlocked}
              aria-describedby={approveBlocked ? "approve-blocked" : undefined}
              onClick={() => onDecide("approve")}
            >
              Approve
            </Button>
            <Button
              variant="outline"
              className="h-10 rounded-full"
              disabled={notSubmitted}
              aria-describedby={notSubmitted ? "approve-blocked" : undefined}
              onClick={() => onDecide("reject")}
            >
              Reject
            </Button>
          </div>
        </>
      )}
    </section>
  );
}

function DetailsCard({ posting }: { posting: ReviewPosting }) {
  const fmt = useFormatters();
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
      "Created",
      <time key="t" dateTime={posting.created_at}>
        {fmt.date(posting.created_at, "calendarDayTime")} UTC
      </time>,
    ],
    ["Company", posting.company_name ?? null],
    ["Seniority", posting.seniority_level ?? null],
    ["Location", posting.location],
    ["Employment type", posting.employment_type],
    ["Salary", salary(posting, fmt)],
    [
      "Interview slots",
      posting.interview_slot === null
        ? null
        : fmt.number(posting.interview_slot),
    ],
    [
      "Interview duration",
      posting.interview_duration === null
        ? null
        : `${fmt.number(posting.interview_duration)} minutes`,
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
            <div
              key={label}
              className="grid grid-cols-[8.5rem_minmax(0,1fr)] gap-3 py-2.5"
            >
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="[overflow-wrap:anywhere]">
                {value ?? (
                  <span className="text-muted-foreground">Not set</span>
                )}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

function PostingDossier({ posting }: { posting: ReviewPosting }) {
  const fmt = useFormatters();
  const evaluation = posting.evaluation;
  return (
    <>
      <section
        aria-labelledby="confirmed-requirements-heading"
        className="space-y-4 rounded-xl border p-5"
      >
        <h2
          id="confirmed-requirements-heading"
          className="text-base font-semibold"
        >
          Confirmed requirements
        </h2>
        <p className="text-sm text-muted-foreground">
          The recruiter confirmed these requirements for question generation.
        </p>
        {posting.requirements && posting.requirements.length > 0 ? (
          <ul className="divide-y text-sm">
            {posting.requirements.map((requirement, index) => (
              <li
                key={`${requirement.target}-${requirement.name}-${index}`}
                className="flex flex-wrap justify-between gap-2 py-2"
              >
                <span className="font-medium">{requirement.name}</span>
                <span className="text-muted-foreground capitalize">
                  {requirement.target.replaceAll("_", " ")} · {requirement.type}
                </span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-muted-foreground">
            No structured requirements are available for this posting. Review
            the original job description below.
          </p>
        )}
        {posting.refinement_note && (
          <div className="space-y-1 text-sm">
            <h3 className="font-medium">Question-generation guidance</h3>
            <p className="whitespace-pre-wrap text-muted-foreground">
              {posting.refinement_note}
            </p>
          </div>
        )}
      </section>
      <section
        aria-labelledby="review-evaluation-heading"
        className="space-y-4 rounded-xl border p-5"
      >
        <h2 id="review-evaluation-heading" className="text-base font-semibold">
          Evaluation criteria
        </h2>
        {evaluation ? (
          <>
            <p className="text-sm text-muted-foreground">
              {evaluation.using_defaults
                ? "Default criteria · equal weights"
                : "Custom evaluation criteria"}
              . These settings are read-only during moderation.
            </p>
            <dl className="divide-y text-sm">
              {evaluation.weights.map((weight) => (
                <div
                  key={weight.metric_id}
                  className="flex justify-between gap-4 py-2"
                >
                  <dt>{weight.name}</dt>
                  <dd className="tabular-nums">
                    {fmt.number(weight.percentage, {
                      maximumFractionDigits: 2,
                    })}
                    {evaluation.total > 0 ? "%" : " weight"}
                  </dd>
                </div>
              ))}
            </dl>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">
            Evaluation details are unavailable.
          </p>
        )}
      </section>
    </>
  );
}
