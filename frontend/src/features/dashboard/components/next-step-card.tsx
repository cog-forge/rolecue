"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ArrowRight, CalendarClock } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  candidateApplicationKeys,
  listCandidateApplications,
} from "@/features/candidate/applications/api/applications";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { PracticeQuery } from "../api/dashboard";
import { recentPractice } from "../utils/practice-summary";
import { DashboardCard, CardError, CardLoading } from "./dashboard-card";

export function NextStepCard({
  practice,
  className,
}: {
  practice: PracticeQuery;
  className?: string;
}) {
  const t = useTranslations("candidate.dashboard.nextStep");
  const user = useSessionUser();
  const fmt = useFormatters();
  const applications = useQuery({
    queryKey: candidateApplicationKeys.list(user.id, 1),
    queryFn: ({ signal }) => listCandidateApplications(1, signal),
    retry: false,
  });
  const ready = applications.data?.items
    .filter((item) => item.can_start_interview)
    .sort(
      (a, b) =>
        (a.interview_deadline ? Date.parse(a.interview_deadline) : Infinity) -
        (b.interview_deadline ? Date.parse(b.interview_deadline) : Infinity),
    )[0];
  const ongoing = recentPractice(practice.data?.items ?? []).find(
    (item) => item.status === "in_progress",
  );
  return (
    <DashboardCard
      isLoading={applications.isPending}
      title={t("title")}
      className={className}
      tone="accent"
      action={
        applications.data &&
        applications.data.total > applications.data.items.length ? (
          <Button asChild variant="link" className="h-11 px-0 text-xs">
            <Link href={routes.applications}>{t("allApplications")}</Link>
          </Button>
        ) : undefined
      }
    >
      {applications.isPending ? (
        <CardLoading label={t("loading")} />
      ) : applications.isError ? (
        <CardError
          message={t("error")}
          disabled={applications.isFetching}
          onRetry={() => void applications.refetch()}
        />
      ) : (
        <div className="space-y-4">
          <Badge
            variant="outline"
            className="border-rolecue-brand/20 bg-background/70 text-foreground"
          >
            {ready ? t("invited") : ongoing ? t("ongoing") : t("practiceLabel")}
          </Badge>
          <p className="text-xl leading-snug font-semibold tracking-tight break-words">
            {ready
              ? ready.job_posting_title
              : ongoing
                ? ongoing.role_profile_title
                : t("practiceTitle")}
          </p>
          {ready?.company_name && (
            <p className="text-sm text-muted-foreground">
              {ready.company_name}
            </p>
          )}
          <p className="text-sm leading-relaxed text-muted-foreground">
            {ready
              ? t("invitedDescription")
              : ongoing
                ? t("ongoingDescription")
                : t("practiceDescription")}
          </p>
          {ready?.interview_deadline && (
            <p className="flex items-start gap-2 text-xs leading-relaxed">
              <CalendarClock
                aria-hidden
                className="mt-0.5 size-4 shrink-0 text-rolecue-brand"
              />
              {t("deadline", {
                date: fmt.date(ready.interview_deadline, "moment"),
              })}
            </p>
          )}
          <Button
            asChild
            variant="outline"
            className="h-auto min-h-11 w-full justify-between gap-3 bg-background py-3 text-left whitespace-normal"
          >
            <Link
              href={
                ready
                  ? routes.application(ready.id)
                  : ongoing
                    ? routes.reports.detail(ongoing.id)
                    : routes.interviews.new.jobDescription
              }
            >
              {ready
                ? t("openApplication")
                : ongoing
                  ? t("openSession")
                  : t("practiceAction")}
              <ArrowRight aria-hidden className="size-4 shrink-0" />
            </Link>
          </Button>
        </div>
      )}
    </DashboardCard>
  );
}
