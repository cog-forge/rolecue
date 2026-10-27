"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  adminInterviewErrorCode,
  adminInterviewKeys,
  getAdminInterview,
  isAdminInterviewAccessError,
  type AdminInterviewDetail as InterviewDetail,
} from "../api/interviews";

// Read-only operational view. Recording, transcript and answer content are never rendered here.
export function AdminInterviewDetail({ id }: { id: string }) {
  const viewer = useSessionUser();
  const validId = z.uuid().safeParse(id).success;
  const detail = useQuery({
    queryKey: adminInterviewKeys.detail(viewer.id, id),
    queryFn: ({ signal }) => getAdminInterview(id, signal),
    enabled: validId && viewer.role === "admin",
    staleTime: 0,
    retry: false,
  });
  const notFound =
    !validId || adminInterviewErrorCode(detail.error) === "INTERVIEW_NOT_FOUND";
  const denied = isAdminInterviewAccessError(detail.error);

  return (
    <section
      className="min-w-0 space-y-7"
      aria-label="Interview session detail"
    >
      <Link
        href={routes.admin.interviews}
        className="inline-flex h-11 items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Back to interview sessions
      </Link>
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Operations
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Interview session
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Operational details only. Recordings, transcripts and answers are not
          available in this view.
        </p>
      </header>
      {notFound ? (
        <div
          role="status"
          className="rounded-xl border bg-card px-5 py-16 text-center"
        >
          <h2 className="text-base font-medium">Interview not found</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            This interview session does not exist or was removed.
          </p>
        </div>
      ) : denied ? (
        <div className="space-y-3 rounded-xl border p-5">
          <p role="alert">
            We couldn&apos;t confirm your administrator access. Interview data
            is hidden until access is verified.
          </p>
          <Button
            variant="outline"
            className="h-11"
            disabled={detail.isFetching}
            onClick={() => void detail.refetch()}
          >
            Retry access check
          </Button>
        </div>
      ) : detail.error ? (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-3 rounded-xl border p-5"
        >
          <p className="text-sm">
            Unable to load this interview session. Please try again.
          </p>
          <Button
            className="h-11"
            variant="outline"
            disabled={detail.isFetching}
            onClick={() => void detail.refetch()}
          >
            Retry
          </Button>
        </div>
      ) : detail.isPending ? (
        <div
          role="status"
          aria-label="Loading interview session"
          className="space-y-3 rounded-xl border p-5"
        >
          <Skeleton className="h-10" />
          {[0, 1, 2, 3].map((row) => (
            <Skeleton key={row} className="h-12" />
          ))}
        </div>
      ) : (
        <InterviewDetailFields interview={detail.data} />
      )}
    </section>
  );
}

function InterviewDetailFields({ interview }: { interview: InterviewDetail }) {
  const rows: [string, string][] = [
    ["Interview ID", interview.id],
    ["Type", interview.type === "practice" ? "Practice" : "Recruitment"],
    ["Owner", interview.owner.name || "Not provided"],
    ["Owner email", interview.owner.email],
    ["Role Profile", interview.role_profile?.title ?? "None (recruitment)"],
    [
      "Job Posting",
      interview.job_posting
        ? interview.job_posting.title
        : "None (practice interview)",
    ],
    ["Application ID", interview.application_id ?? "None (practice interview)"],
    ["Avatar", interview.avatar?.name ?? "None"],
    ["Voice", interview.voice?.name ?? "None"],
    ["Questions", String(interview.question_count)],
    ["Turns", String(interview.turn_count)],
    [
      "Score",
      interview.score === null ? "Not scored" : String(interview.score),
    ],
    ["Report", interview.has_report ? "Available" : "Not available"],
    ["Recording", interview.has_recording ? "Recorded" : "Not recorded"],
  ];
  return (
    <dl className="divide-y rounded-xl border bg-card text-sm">
      {rows.map(([label, value]) => (
        <div
          key={label}
          className="grid gap-1 px-5 py-4 sm:grid-cols-[12rem_minmax(0,1fr)] sm:gap-4"
        >
          <dt className="text-muted-foreground">{label}</dt>
          <dd className="break-words font-medium">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
