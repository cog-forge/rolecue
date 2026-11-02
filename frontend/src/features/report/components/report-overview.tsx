"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("candidate.report");
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
        {t("backToHistory")}
      </Link>

      {!validId ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">{t("invalidTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("invalidText")}
          </p>
        </div>
      ) : query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-48 rounded-xl" />
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border bg-muted/60 p-6">
          <h1 className="text-xl font-semibold">{t("deniedTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("accessChecking")}
          </p>
        </div>
      ) : apiStatus(query.error) === 404 ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">{t("notFoundTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("notFoundText")}
          </p>
          <Button
            asChild
            variant="outline"
            className="mt-4 h-auto min-h-11 py-2 whitespace-normal"
          >
            <Link href={routes.history}>{t("openHistory")}</Link>
          </Button>
        </div>
      ) : query.isError ? (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <h1 className="text-xl font-semibold">{t("errorTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("errorText")}</p>
          <Button
            className="mt-4 h-auto min-h-11 py-2 whitespace-normal bg-background"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            {t("retry")}
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
