"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import {
  candidateApplicationKeys,
  listCandidateApplications,
} from "@/features/candidate/applications/api/applications";
import type { CandidateApplication } from "@/features/candidate/applications/schemas/candidate-application-schema";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { applicationStatusLabel } from "../utils/dashboard-text";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";

const PREVIEW_COUNT = 3;

// Reads the first page of the Candidate's Applications (the same list the Applications screen
// uses). Interview eligibility comes from `can_start_interview`, as the server sends it.
export function ApplicationsCard() {
  const user = useSessionUser();
  const query = useQuery({
    queryKey: candidateApplicationKeys.list(user.id, 1),
    queryFn: ({ signal }) => listCandidateApplications(1, signal),
    retry: false,
  });

  return (
    <DashboardCard title="Applications">
      {query.isPending ? (
        <CardLoading label="Loading your applications…" />
      ) : query.isError ? (
        <CardError
          message="We couldn't load your applications."
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : query.data.total === 0 ? (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            You haven&apos;t applied to any postings yet.
          </p>
          <Button asChild variant="outline" className="h-11 bg-background">
            <Link href={routes.jobs}>Browse the Job Board</Link>
          </Button>
        </div>
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
  return (
    <div className="space-y-3">
      <p className="text-2xl font-semibold tabular-nums">
        {total} {total === 1 ? "application" : "applications"}
      </p>
      {ready.length > 0 ? (
        <p className="text-sm text-muted-foreground">
          Next step: start the interview for{" "}
          <span className="font-medium text-foreground">
            {ready[0].job_posting_title}
          </span>
          .
        </p>
      ) : null}
      <ul className="divide-y rounded-lg border">
        {items.map((application) => (
          <li
            key={application.id}
            className="flex items-center justify-between gap-3 px-3 py-2 text-sm"
          >
            <Link
              href={routes.application(application.id)}
              className="min-w-0 truncate font-medium underline-offset-4 hover:underline focus-visible:underline"
            >
              {application.job_posting_title}
            </Link>
            <span className="shrink-0 text-xs text-muted-foreground">
              {applicationStatusLabel[application.status]}
            </span>
          </li>
        ))}
      </ul>
      <Button asChild variant="link" className="h-11 px-0">
        <Link href={routes.applications}>View all applications</Link>
      </Button>
    </div>
  );
}
