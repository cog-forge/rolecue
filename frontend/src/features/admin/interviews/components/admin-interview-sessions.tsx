"use client";

import { useTranslations } from "next-intl";
import { RefreshCw, Mic } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { ADMIN_INTERVIEWS_PAGE_SIZE } from "../api/interviews";
import { useAdminInterviews } from "../hooks/use-admin-interviews";
import { InterviewFilters } from "./interview-filters";
import { InterviewsTable } from "./interviews-table";

export function AdminInterviewSessions() {
  const t = useTranslations("admin.interviews");
  const tCommon = useTranslations("common");
  const { list, query, denied, applyFilters, changePage } =
    useAdminInterviews();
  const total = list.data?.total ?? 0;
  const filtered =
    query.type !== null || !!query.candidateId || !!query.jobPostingId;
  const rows = list.data?.items ?? [];
  return (
    <section className="min-w-0 space-y-7" aria-label={t("sessions.label")}>
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("sessions.title")}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {t("sessions.description")}
        </p>
      </header>
      {denied ? (
        <div className="space-y-3 rounded-xl border p-5">
          <p role="alert">{t("accessDenied.message")}</p>
          <Button
            variant="outline"
            className="h-11"
            disabled={list.isFetching}
            onClick={() => void list.refetch()}
          >
            {t("accessDenied.retry")}
          </Button>
        </div>
      ) : (
        <>
          <InterviewFilters
            query={query}
            disabled={list.isFetching}
            onApply={applyFilters}
          />
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p
              role="status"
              aria-live="polite"
              className="text-sm text-muted-foreground"
            >
              {list.isFetching
                ? t("sessions.loading")
                : t("sessions.count", { total, filtered: String(filtered) })}
            </p>
            <Button
              variant="ghost"
              className="h-11"
              disabled={list.isFetching}
              onClick={() => void list.refetch()}
            >
              <RefreshCw className="size-4" aria-hidden />
              {t("sessions.refresh")}
            </Button>
          </div>
          {list.error && (
            <div
              role="alert"
              className="flex flex-wrap items-center justify-between gap-3 rounded-xl border p-5"
            >
              <p className="text-sm">{t("sessions.loadFailed")}</p>
              <Button
                className="h-11"
                variant="outline"
                disabled={list.isFetching}
                onClick={() => void list.refetch()}
              >
                {tCommon("actions.retry")}
              </Button>
            </div>
          )}
          {list.isPending ? (
            <div
              role="status"
              aria-label={t("sessions.loadingLabel")}
              className="space-y-3 rounded-xl border p-5"
            >
              <Skeleton className="h-10" />
              {[0, 1, 2, 3].map((row) => (
                <Skeleton key={row} className="h-16" />
              ))}
            </div>
          ) : rows.length ? (
            <InterviewsTable rows={rows} />
          ) : (
            !list.error && (
              <div
                role="status"
                className="rounded-xl border bg-card px-5 py-16 text-center"
              >
                <Mic
                  className="mx-auto size-7 text-muted-foreground"
                  aria-hidden
                />
                <h2 className="mt-4 text-base font-medium">
                  {filtered
                    ? t("sessions.emptyFilteredTitle")
                    : t("sessions.emptyTitle")}
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  {filtered
                    ? t("sessions.emptyFilteredHint")
                    : t("sessions.emptyHint")}
                </p>
              </div>
            )
          )}
          <DataTablePagination
            offset={(query.page - 1) * ADMIN_INTERVIEWS_PAGE_SIZE}
            pageSize={ADMIN_INTERVIEWS_PAGE_SIZE}
            total={total}
            disabled={list.isFetching}
            label={t("sessions.paginationLabel")}
            onPageChange={changePage}
          />
        </>
      )}
    </section>
  );
}
