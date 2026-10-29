"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { dashboardKeys, listPracticeInterviews } from "../api/dashboard";
import type { InterviewHistoryItem } from "../schemas/dashboard-schema";
import { formatDate, interviewStatusLabel } from "../utils/dashboard-text";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";
import { useFormatters } from "@/lib/i18n/use-formatters";

const PREVIEW_COUNT = 3;

// Shows the latest practice sessions. Scores and statuses are displayed as the server reports
// them; no score is computed here.
export function PracticeActivityCard() {
  const user = useSessionUser();
  const query = useQuery({
    queryKey: dashboardKeys.practice(user.id),
    queryFn: ({ signal }) => listPracticeInterviews(PREVIEW_COUNT, signal),
    retry: false,
  });

  return (
    <DashboardCard title="Practice activity">
      {query.isPending ? (
        <CardLoading label="Loading your practice activity…" />
      ) : query.isError ? (
        <CardError
          message="We couldn't load your practice activity."
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : query.data.total === 0 ? (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            No practice sessions yet. Start one to see your feedback here.
          </p>
          <Button asChild variant="outline" className="h-11 bg-background">
            <Link href={routes.interviews.new.jobDescription}>
              Start a practice interview
            </Link>
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-2xl font-semibold tabular-nums">
            {query.data.total} {query.data.total === 1 ? "session" : "sessions"}
          </p>
          <ul className="divide-y rounded-lg border">
            {query.data.items.map((item) => (
              <PracticeRow key={item.id} item={item} />
            ))}
          </ul>
          <Button asChild variant="link" className="h-11 px-0">
            <Link href={routes.history}>View all interview history</Link>
          </Button>
        </div>
      )}
    </DashboardCard>
  );
}

function PracticeRow({ item }: { item: InterviewHistoryItem }) {
  const fmt = useFormatters();
  return (
    <li className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
      <div className="min-w-0">
        <Link
          href={routes.reports.detail(item.id)}
          className="block truncate font-medium underline-offset-4 hover:underline focus-visible:underline"
        >
          {item.role_profile_title}
        </Link>
        <p className="text-xs text-muted-foreground">
          {formatDate(item.started_at, fmt)} ·{" "}
          {interviewStatusLabel[item.status]}
        </p>
      </div>
      {item.score !== null ? (
        <span className="shrink-0 text-sm font-semibold tabular-nums">
          {item.score}/100
        </span>
      ) : null}
    </li>
  );
}
