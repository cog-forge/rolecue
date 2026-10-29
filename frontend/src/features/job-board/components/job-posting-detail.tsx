"use client";

import { useEffect, type ReactNode } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import {
  ArrowLeft,
  Building2,
  CircleCheck,
  Clock,
  ExternalLink,
  MapPin,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { CvApplyForm } from "@/features/candidate/applications/components/cv-apply-form";
import { isHttpsProfileUrl } from "@/features/profile/schemas/profile-schema";
import { getJobBoardPosting, jobBoardKeys } from "../api/job-board";
import type {
  BoardPostingDetail,
  BoardRequirement,
} from "../schemas/job-board-schema";
import { apiStatus, isAccessDenied } from "../utils/api-error";
import {
  fullDate,
  levelText,
  publicationText,
  salaryText,
  targetLabel,
} from "../utils/format";

// Only a website the profile rules accept (https, a host, no credentials) becomes a link.
function safeWebsite(value: string | null) {
  return value && isHttpsProfileUrl(value) ? new URL(value) : null;
}

export function JobPostingDetail({
  id,
  backQuery,
}: {
  id: string;
  backQuery: string;
}) {
  const user = useSessionUser();
  const cache = useQueryClient();
  // Go rejects an ID that is not a UUID, so a mistyped link is not sent at all.
  const validId = z.uuid().safeParse(id).success;
  const query = useQuery({
    queryKey: jobBoardKeys.detail(user.id, id),
    queryFn: ({ signal }) => getJobBoardPosting(id, signal),
    enabled: validId,
    // "Already applied" and intake can change at any time, so always ask again.
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);
  // Go answers the same 404 for a posting that is missing, not open or closed to intake.
  const unavailable = apiStatus(query.error) === 404;
  const backHref = `${routes.jobs}${backQuery ? `?${backQuery}` : ""}`;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <Link
        href={backHref}
        className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Back to Job Board
      </Link>

      {!validId ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">This link isn&apos;t valid</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Check the address, or find the posting on the Job Board.
          </p>
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={backHref}>Browse open postings</Link>
          </Button>
        </div>
      ) : query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-6 w-1/2" />
          <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
            <Skeleton className="h-96 rounded-xl" />
            <Skeleton className="h-40 rounded-xl" />
          </div>
          <span className="sr-only">Loading the job posting…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border bg-muted/60 p-6">
          <h1 className="text-xl font-semibold">
            You don&apos;t have access to this posting
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            candidate account.
          </p>
        </div>
      ) : unavailable ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">
            This posting is no longer available
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            It stopped accepting applications or was removed. Other open
            postings are waiting on the Job Board.
          </p>
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={backHref}>Browse open postings</Link>
          </Button>
        </div>
      ) : query.isError ? (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <h1 className="text-xl font-semibold">
            We couldn&apos;t load this job posting
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
        <PostingContent
          posting={query.data}
          canApply={user.role === "candidate"}
        />
      )}
    </section>
  );
}

function PostingContent({
  posting,
  canApply,
}: {
  posting: BoardPostingDetail;
  canApply: boolean;
}) {
  const website = safeWebsite(posting.company_website);
  const salary = salaryText(posting);
  const required = posting.requirements.filter((r) => r.type === "required");
  const preferred = posting.requirements.filter((r) => r.type === "preferred");
  const facts: { label: string; value: ReactNode }[] = [
    { label: "Level", value: levelText[posting.seniority_level] },
    ...(posting.employment_type
      ? [{ label: "Employment type", value: posting.employment_type }]
      : []),
    ...(salary ? [{ label: "Salary", value: salary }] : []),
    ...(posting.interview_duration
      ? [
          {
            label: "Interview length",
            value: `${posting.interview_duration} minutes`,
          },
        ]
      : []),
  ];

  return (
    <div className="space-y-6 duration-300 animate-in fade-in motion-reduce:animate-none">
      <header className="space-y-3">
        <h1 className="text-3xl font-semibold tracking-tight [overflow-wrap:anywhere] sm:text-4xl">
          {posting.title}
        </h1>
        <p className="flex flex-wrap gap-x-5 gap-y-1 text-sm text-muted-foreground">
          {website ? (
            <a
              href={website.href}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-11 items-center gap-1.5 rounded-sm font-medium text-foreground underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 sm:min-h-0"
            >
              <Building2 className="size-4 shrink-0" aria-hidden />
              {/* Without a company name, the website's host still says whose posting it is. */}
              {posting.company_name ?? website.hostname}
              <ExternalLink className="size-3.5 shrink-0" aria-hidden />
              <span className="sr-only">(opens in a new tab)</span>
            </a>
          ) : (
            posting.company_name && (
              <span className="inline-flex items-center gap-1.5 font-medium text-foreground [overflow-wrap:anywhere]">
                <Building2 className="size-4 shrink-0" aria-hidden />
                {posting.company_name}
              </span>
            )
          )}
          {posting.location && (
            <span className="inline-flex items-center gap-1.5 [overflow-wrap:anywhere]">
              <MapPin className="size-4 shrink-0" aria-hidden />
              {posting.location}
            </span>
          )}
          <span className="inline-flex items-center gap-1.5">
            <Clock className="size-4 shrink-0" aria-hidden />
            <time
              dateTime={posting.published_at ?? posting.created_at}
              title={fullDate(posting.published_at ?? posting.created_at)}
            >
              {publicationText(posting)}
            </time>
          </span>
        </p>
      </header>

      <div className="grid items-start gap-6 lg:grid-cols-[1fr_20rem]">
        <div className="min-w-0 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>
                <h2>Requirements</h2>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              {posting.requirements.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  This posting doesn&apos;t list specific requirements. Read the
                  job description below.
                </p>
              ) : (
                <>
                  <RequirementGroup title="Required" items={required} />
                  <RequirementGroup title="Nice to have" items={preferred} />
                </>
              )}
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>
                <h2>Job description</h2>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {/* Recruiter text is untrusted: render it as text, never as HTML. */}
              {posting.raw_text.trim() ? (
                <p className="text-sm leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">
                  {posting.raw_text}
                </p>
              ) : (
                <p className="text-sm text-muted-foreground">
                  The recruiter hasn&apos;t added a job description.
                </p>
              )}
            </CardContent>
          </Card>
        </div>

        <aside className="space-y-4 lg:sticky lg:top-6">
          <Card>
            <CardContent className="space-y-4">
              <dl className="space-y-3 text-sm">
                {facts.map((fact) => (
                  <div key={fact.label} className="flex justify-between gap-4">
                    <dt className="text-muted-foreground">{fact.label}</dt>
                    <dd className="text-right font-medium [overflow-wrap:anywhere]">
                      {fact.value}
                    </dd>
                  </div>
                ))}
              </dl>
              {canApply && (
                <ApplyPanel
                  applied={posting.already_applied}
                  postingId={posting.id}
                />
              )}
            </CardContent>
          </Card>
        </aside>
      </div>
    </div>
  );
}

function RequirementGroup({
  title,
  items,
}: {
  title: string;
  items: BoardRequirement[];
}) {
  if (items.length === 0) return null;
  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold tracking-[0.08em] text-muted-foreground uppercase">
        {title}
      </h3>
      <ul className="divide-y rounded-lg border">
        {items.map((item, i) => (
          <li
            key={i}
            className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 px-3 py-2.5 text-sm"
          >
            <span className="font-medium [overflow-wrap:anywhere]">
              {item.name}
            </span>
            <span className="text-xs text-muted-foreground">
              {targetLabel(item.target)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

// aria-disabled instead of disabled keeps the button in the tab order, so its note is announced.
const unavailableButton =
  "h-11 w-full aria-disabled:cursor-not-allowed aria-disabled:opacity-50";

// Candidates apply with a CV; the upload flow lives in the candidate applications feature.
function ApplyPanel({
  applied,
  postingId,
}: {
  applied: boolean;
  postingId: string;
}) {
  if (!applied) return <CvApplyForm postingId={postingId} />;
  return (
    <div className="space-y-3 border-t pt-4">
      <Button
        type="button"
        className={unavailableButton}
        aria-disabled="true"
        aria-describedby="apply-note"
      >
        <CircleCheck aria-hidden />
        Applied
      </Button>
      <p
        id="apply-note"
        role="status"
        className="flex items-start gap-2 text-sm"
      >
        <span
          className="mt-1.5 size-2 shrink-0 rounded-full bg-emerald-500"
          aria-hidden
        />
        You already applied to this posting.
      </p>
      <Link
        href={routes.applications}
        className="inline-flex min-h-11 items-center text-sm font-medium text-workspace-accent underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        View my applications
      </Link>
    </div>
  );
}
