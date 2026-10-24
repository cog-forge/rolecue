"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  ListChecks,
  LoaderCircle,
  RotateCcw,
  Scale,
} from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { ApplicationList } from "@/features/recruiter/applications/components/application-list";
import {
  getPosting,
  jobPostingKeys,
  resubmitPosting,
  updatePosting,
} from "../api/job-postings";
import {
  postingLocks,
  postingPatch,
  postingValues,
  type JobPosting,
} from "../schemas/job-posting-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { JdDocumentLink } from "./jd-document-link";
import { levelText, PostingForm, targetText } from "./posting-form";
import { headerBandClass, Notice } from "./notice";
import { PostingStatus } from "./posting-status";

// Codes after which the cached posting is stale: its status or question bank changed elsewhere.
// Refetching shows the server's truth and the matching locks.
const staleCodes = new Set([
  "JOB_POSTING_NOT_EDITABLE",
  "JOB_POSTING_TEXT_LOCKED",
  "JOB_POSTING_REQUIREMENTS_LOCKED",
  "JOB_POSTING_NOT_REJECTED",
]);
const dateFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});

export function JobPostingDetail({ postingId }: { postingId: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = jobPostingKeys.detail(user.id, postingId);
  const [notice, setNotice] = useState<string | null>(null);
  // Remounts the form with the saved values after each successful save.
  const [version, setVersion] = useState(0);
  // Validation is async, so two quick clicks can both reach onSubmit before isPending flips.
  const saving = useRef(false);

  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getPosting(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const applySaved = (saved: JobPosting) => {
    cache.setQueryData(key, saved);
    void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
    setVersion((value) => value + 1);
  };
  const onStale = (error: unknown) => {
    if (isAccessDenied(error))
      void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    const { code, message } = apiError(error);
    if (!code || !staleCodes.has(code)) return;
    setNotice(`${message ?? "This posting changed."} We refreshed it for you.`);
    // Remount the form on the fresh posting: the edited values were made against stale locks and
    // would be sent (and refused) again on every save.
    void cache
      .refetchQueries({ queryKey: key })
      .then(() => setVersion((value) => value + 1));
  };

  const save = useMutation({
    mutationFn: (patch: Parameters<typeof updatePosting>[1]) =>
      updatePosting(postingId, patch),
    retry: false,
    onSuccess: (saved) => {
      applySaved(saved);
      setNotice(null);
      gooeyToast.success("Changes saved");
    },
    onError: onStale,
  });
  const resubmit = useMutation({
    mutationFn: () => resubmitPosting(postingId),
    retry: false,
    onSuccess: (saved) => {
      applySaved(saved);
      setNotice(null);
      gooeyToast.success("Sent for review again", {
        description: "An administrator reviews the corrected posting.",
      });
    },
    onError: onStale,
  });

  const back = (
    <Link
      href={routes.recruiter.jobPostings}
      className="inline-flex min-h-11 items-center gap-1 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <ArrowLeft className="size-4" aria-hidden />
      Back to my job postings
    </Link>
  );

  if (query.isPending)
    return (
      <section className="mx-auto w-full max-w-4xl space-y-6">
        {back}
        <div role="status" className="space-y-3">
          <Skeleton className="h-10 w-2/3 rounded-md" />
          <Skeleton className="h-64 rounded-xl" />
          <span className="sr-only">Loading the job posting…</span>
        </div>
      </section>
    );
  if (query.isError) {
    // 400 is a malformed ID in the URL; retrying cannot help, just like a missing posting.
    const notFound = [400, 404].includes(apiError(query.error).status);
    return (
      <section className="mx-auto w-full max-w-4xl space-y-6">
        {back}
        <div
          role="alert"
          className={cn(
            "rounded-2xl border p-6 duration-300 animate-in fade-in motion-reduce:animate-none",
            denied || notFound
              ? "bg-muted/50"
              : "border-destructive/30 bg-destructive/5",
          )}
        >
          <h1 className="text-xl font-semibold">
            {denied
              ? "You don't have access to job postings"
              : notFound
                ? "We couldn't find this job posting"
                : "We couldn't load this job posting"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {denied
              ? "Checking your account access… If this stays, sign in again with a recruiter account."
              : notFound
                ? "It may have been removed, or it belongs to another recruiter."
                : "Please try again in a moment."}
          </p>
          {!denied && !notFound && (
            <Button
              className="mt-4 h-11 bg-background"
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              Try again
            </Button>
          )}
        </div>
      </section>
    );
  }

  const posting = query.data;
  const locks = postingLocks(posting);
  const saveError =
    save.isError && !staleCodes.has(apiError(save.error).code ?? "")
      ? isAccessDenied(save.error)
        ? "You don't have access to edit this posting. Checking your account access…"
        : `${apiError(save.error).message ?? "We couldn't save your changes."} Your changes are still here.`
      : null;
  const resubmitError =
    resubmit.isError && !staleCodes.has(apiError(resubmit.error).code ?? "")
      ? (apiError(resubmit.error).message ??
        "We couldn't send the posting for review.")
      : null;

  return (
    <section className="mx-auto w-full max-w-4xl space-y-6">
      {back}
      <header
        className={cn(
          headerBandClass,
          "flex flex-wrap items-start justify-between gap-4",
        )}
      >
        <div className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
            Job posting
          </p>
          <h1 className="text-3xl font-semibold tracking-tight [overflow-wrap:anywhere] sm:text-4xl">
            {posting.title}
          </h1>
          <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
            <PostingStatus status={posting.status} />
            <span>
              Created{" "}
              <time dateTime={posting.created_at} title={posting.created_at}>
                {dateFormat.format(new Date(posting.created_at))}
              </time>
            </span>
          </p>
          <p className="text-sm text-muted-foreground">
            Interview slots:{" "}
            <span className="font-medium text-foreground tabular-nums">
              {posting.interview_slot === null
                ? "not funded yet"
                : posting.interview_slot}
            </span>
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild variant="outline" className="h-11 bg-background">
            <Link href={routes.recruiter.jobPostingQuestions(posting.id)}>
              <ListChecks aria-hidden />
              Question bank
              {posting.core_question_count > 0 && (
                <span className="tabular-nums">
                  ({posting.core_question_count})
                </span>
              )}
            </Link>
          </Button>
          <Button asChild variant="outline" className="h-11 bg-background">
            <Link href={routes.recruiter.jobPostingEvaluation(posting.id)}>
              <Scale aria-hidden />
              Evaluation criteria
            </Link>
          </Button>
        </div>
      </header>

      {notice && (
        <Notice role="status" tone="info">
          <p>{notice}</p>
        </Notice>
      )}

      {posting.status === "rejected" && (
        <Notice tone="danger" title="An administrator rejected this posting">
          {posting.reject_reason && (
            <p className="font-medium">Reason: {posting.reject_reason}</p>
          )}
          <p className="text-muted-foreground">
            Correct it below, save your changes, then send it for review again.
          </p>
        </Notice>
      )}
      {posting.status === "pending" && (
        <Notice tone="info">
          <p>
            Waiting for an administrator to review it. You can still edit it
            until then.
          </p>
        </Notice>
      )}

      {locks.editable ? (
        <PostingForm
          key={version}
          mode="edit"
          posting={posting}
          defaultValues={postingValues(posting)}
          locks={locks}
          submitting={save.isPending}
          submitError={saveError}
          onSubmit={(values) => {
            const patch = postingPatch(values, posting);
            if (saving.current || Object.keys(patch).length === 0) return;
            saving.current = true;
            save.mutate(patch, {
              onSettled: () => {
                saving.current = false;
              },
            });
          }}
          extraActions={({ dirty }) =>
            posting.status === "rejected" ? (
              <>
                <Button
                  type="button"
                  variant="outline"
                  className="h-11"
                  disabled={dirty || resubmit.isPending || save.isPending}
                  aria-busy={resubmit.isPending}
                  aria-describedby="posting-resubmit-hint"
                  onClick={() => resubmit.mutate()}
                >
                  {resubmit.isPending ? (
                    <LoaderCircle
                      className="size-4 animate-spin motion-reduce:animate-none"
                      aria-hidden
                    />
                  ) : (
                    <RotateCcw aria-hidden />
                  )}
                  {resubmit.isPending ? "Sending…" : "Send for review again"}
                </Button>
                <span
                  id="posting-resubmit-hint"
                  className="w-full text-xs text-muted-foreground sm:w-auto"
                >
                  {dirty
                    ? "Save your changes before sending it for review."
                    : ""}
                </span>
                {resubmitError && (
                  <p role="alert" className="w-full text-sm text-destructive">
                    {resubmitError}
                  </p>
                )}
              </>
            ) : null
          }
        />
      ) : (
        <ReadOnlyPosting posting={posting} />
      )}

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Applications</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {posting.status === "pending" || posting.status === "rejected" ? (
            <p className="text-sm text-muted-foreground">
              Candidates can apply once an administrator approves this posting.
            </p>
          ) : (
            // The list reads its filters from the URL on the client.
            <Suspense>
              <ApplicationList postingId={posting.id} />
            </Suspense>
          )}
        </CardContent>
      </Card>
    </section>
  );
}

function ReadOnlyPosting({ posting }: { posting: JobPosting }) {
  const details: [string, string][] = [
    ["Seniority", levelText[posting.seniority_level]],
    ["Location", posting.location ?? "Not set"],
    ["Employment type", posting.employment_type ?? "Not set"],
    [
      "Salary",
      posting.salary_min === null && posting.salary_max === null
        ? "Not set"
        : `${posting.salary_min ?? "…"} – ${posting.salary_max ?? "…"}`,
    ],
    [
      "Interview length",
      posting.interview_duration === null
        ? "Not set"
        : `${posting.interview_duration} minutes`,
    ],
    ["Interviewer model", posting.required_avatar?.name ?? "Not set"],
    ["Interviewer voice", posting.required_voice?.name ?? "Not set"],
    [
      "Candidates allowed into interview",
      posting.interview_slot === null
        ? "Not set"
        : String(posting.interview_slot),
    ],
  ];
  return (
    <div className="space-y-6">
      <Notice tone="locked">
        <p>This posting was approved, so it can no longer be edited.</p>
      </Notice>
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Details</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            {details.map(([label, value]) => (
              <div key={label}>
                <dt className="text-muted-foreground">{label}</dt>
                <dd className="font-medium [overflow-wrap:anywhere]">
                  {value}
                </dd>
              </div>
            ))}
          </dl>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Requirements</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="divide-y rounded-xl border bg-muted/20 text-sm">
            {posting.requirements.map((row) => (
              <li
                key={`${row.name}-${row.target}`}
                className="flex flex-wrap justify-between gap-2 p-3"
              >
                <span className="font-medium [overflow-wrap:anywhere]">
                  {row.name}
                </span>
                <span className="text-muted-foreground">
                  {targetText[row.target]} ·{" "}
                  {row.type === "required" ? "Required" : "Preferred"}
                </span>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Job description</h2>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {posting.has_document && <JdDocumentLink postingId={posting.id} />}
          <div className="max-h-96 overflow-y-auto text-sm leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">
            {posting.raw_text}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
