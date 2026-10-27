"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getInterviewReport, reportKeys } from "../api/report";
import { apiStatus, isAccessDenied } from "../utils/api-status";
import { ReportBody } from "./report-body";

// One interview report for the user who may read it. The Go API decides visibility; a refused or
// missing report looks the same (404), so this screen does not reveal whether it exists.
export function ReportOverview({ reportId }: { reportId: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const validId = z.uuid().safeParse(reportId).success;
  const query = useQuery({
    queryKey: reportKeys.detail(user.id, reportId),
    queryFn: ({ signal }) => getInterviewReport(reportId, signal),
    enabled: validId,
    // Evaluation finishes on the server, so every visit asks again.
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  return (
    <section className="mx-auto w-full max-w-3xl space-y-6">
      <Link
        href={routes.history}
        className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        Back to interview history
      </Link>

      {!validId ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">This link isn&apos;t valid</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Check the address, or open the report from your interview history.
          </p>
        </div>
      ) : query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-48 rounded-xl" />
          <span className="sr-only">Loading your report…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border bg-muted/60 p-6">
          <h1 className="text-xl font-semibold">
            You don&apos;t have access to this report
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            candidate account.
          </p>
        </div>
      ) : apiStatus(query.error) === 404 ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">
            We couldn&apos;t find this report
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            It may have been removed, or it isn&apos;t yours.
          </p>
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={routes.history}>Open interview history</Link>
          </Button>
        </div>
      ) : query.isError ? (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <h1 className="text-xl font-semibold">
            We couldn&apos;t load this report
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
        <ReportBody
          report={query.data}
          checking={query.isFetching}
          onCheckAgain={() => void query.refetch()}
        />
      )}
    </section>
  );
}
