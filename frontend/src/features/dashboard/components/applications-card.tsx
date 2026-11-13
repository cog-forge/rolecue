"use client";

import { useTranslations } from "next-intl";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { BriefcaseBusiness } from "lucide-react";
import { routes } from "@/config/routes";
import {
  candidateApplicationKeys,
  listCandidateApplications,
} from "@/features/candidate/applications/api/applications";
import type { CandidateApplication } from "@/features/candidate/applications/schemas/candidate-application-schema";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";

const PREVIEW_COUNT = 3;

// Reads the first page of the Candidate's Applications (the same list the Applications screen
// uses). Interview eligibility comes from `can_start_interview`, as the server sends it.
export function ApplicationsCard({ className }: { className?: string }) {
  const t = useTranslations("candidate.dashboard.applications");
  const user = useSessionUser();
  const query = useQuery({
    queryKey: candidateApplicationKeys.list(user.id, 1),
    queryFn: ({ signal }) => listCandidateApplications(1, signal),
    retry: false,
  });

  return (
    <DashboardCard
      title={t("title")}
      className={className}
      isLoading={query.isPending}
    >
      {query.isPending ? (
        <CardLoading label={t("loading")} />
      ) : query.isError ? (
        <CardError
          message={t("error")}
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : query.data.total === 0 ? (
        <EmptyState
          variant="plain"
          icons={[BriefcaseBusiness]}
          title={t("emptyTitle")}
          description={t("empty")}
          action={{ label: t("browse"), href: routes.jobs, variant: "outline" }}
          className="min-h-40 px-0! py-2!"
        />
      ) : (
        <ApplicationsBody
          total={query.data.total}
          items={query.data.items.slice(0, PREVIEW_COUNT)}
        />
      )}
    </DashboardCard>
  );
}

function ApplicationsBody({
  total,
  items,
}: {
  total: number;
  items: readonly CandidateApplication[];
}) {
  const ready = items.filter((application) => application.can_start_interview);
  const t = useTranslations("candidate.dashboard.applications");
  return (
    <div className="space-y-3">
      <p className="text-3xl font-semibold tracking-tight tabular-nums">
        {t("count", { count: total, countText: String(total) })}
      </p>
      {ready.length > 0 ? (
        <p className="text-sm text-muted-foreground">
          {t.rich("next", {
            title: ready[0].job_posting_title,
            name: (chunks) => (
              <span className="font-medium text-foreground">{chunks}</span>
            ),
          })}
        </p>
      ) : null}
      <ul className="divide-y">
        {items.map((application) => (
          <li
            key={application.id}
            className="flex items-center justify-between gap-3 py-3 text-sm"
          >
            <Link
              href={routes.application(application.id)}
              className="min-w-0 truncate font-medium underline-offset-4 hover:underline focus-visible:underline"
            >
              {application.job_posting_title}
            </Link>
            <span className="shrink-0 text-xs text-muted-foreground">
              {t(`status.${application.status}`)}
            </span>
          </li>
        ))}
      </ul>
      <Button asChild variant="link" className="h-11 px-0">
        <Link href={routes.applications}>{t("view")}</Link>
      </Button>
    </div>
  );
}
