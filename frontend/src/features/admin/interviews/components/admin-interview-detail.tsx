"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  adminInterviewErrorCode,
  adminInterviewKeys,
  getAdminInterview,
  isAdminInterviewAccessError,
  type AdminInterviewDetail as InterviewDetail,
} from "../api/interviews";

// Read-only operational view. Recording, transcript and answer content are never rendered here.
export function AdminInterviewDetail({ id }: { id: string }) {
  const t = useTranslations("admin.interviews");
  const tCommon = useTranslations("common");
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
    <section className="min-w-0 space-y-7" aria-label={t("detail.label")}>
      <Link
        href={routes.admin.interviews}
        className="inline-flex h-11 items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t("detail.back")}
      </Link>
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("detail.title")}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {t("detail.description")}
        </p>
      </header>
      {notFound ? (
        <div
          role="status"
          className="rounded-xl border bg-card px-5 py-16 text-center"
        >
          <h2 className="text-base font-medium">{t("detail.notFoundTitle")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("detail.notFoundDescription")}
          </p>
        </div>
      ) : denied ? (
        <div className="space-y-3 rounded-xl border p-5">
          <p role="alert">{t("accessDenied.message")}</p>
          <Button
            variant="outline"
            className="h-11"
            disabled={detail.isFetching}
            onClick={() => void detail.refetch()}
          >
            {t("accessDenied.retry")}
          </Button>
        </div>
      ) : detail.error ? (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-3 rounded-xl border p-5"
        >
          <p className="text-sm">{t("detail.loadFailed")}</p>
          <Button
            className="h-11"
            variant="outline"
            disabled={detail.isFetching}
            onClick={() => void detail.refetch()}
          >
            {tCommon("actions.retry")}
          </Button>
        </div>
      ) : detail.isPending ? (
        <div
          role="status"
          aria-label={t("detail.loadingLabel")}
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
  const t = useTranslations("admin.interviews.detail");
  const types = useTranslations("admin.interviews.type");
  const values = useTranslations("admin.interviews.values");
  const fmt = useFormatters();
  const fields = useTranslations("admin.interviews.detail.fields");
  const rows: [string, string][] = [
    [fields("id"), interview.id],
    [fields("type"), types(interview.type)],
    [fields("owner"), interview.owner.name || t("notProvided")],
    [fields("ownerEmail"), interview.owner.email],
    [
      fields("roleProfile"),
      interview.role_profile?.title ?? t("noRoleProfile"),
    ],
    [
      fields("jobPosting"),
      interview.job_posting ? interview.job_posting.title : t("noJobPosting"),
    ],
    [fields("applicationId"), interview.application_id ?? t("noApplication")],
    [fields("avatar"), interview.avatar?.name ?? t("none")],
    [fields("voice"), interview.voice?.name ?? t("none")],
    [fields("questions"), fmt.number(interview.question_count)],
    [fields("turns"), fmt.number(interview.turn_count)],
    [
      fields("ended"),
      interview.completed_at === null
        ? values("inProgress")
        : interview.end_reason
          ? values(`endReason.${interview.end_reason}`)
          : values("endReason.unknown"),
    ],
    [
      fields("score"),
      interview.score === null
        ? values("notScored")
        : fmt.number(interview.score),
    ],
    [
      fields("report"),
      interview.has_report ? values("available") : values("notAvailable"),
    ],
    [
      fields("recording"),
      interview.has_recording ? values("recorded") : values("notRecorded"),
    ],
  ];
  return (
    <dl className="divide-y rounded-xl border bg-card text-sm">
      {rows.map(([label, value]) => (
        <div
          key={label}
          className="grid gap-1 px-5 py-4 sm:grid-cols-[12rem_minmax(0,1fr)] sm:gap-4"
        >
          <dt className="text-muted-foreground">{label}</dt>
          <dd className="font-medium break-words">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
