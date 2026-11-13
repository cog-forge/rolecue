"use client";

import { EmptyState } from "@/components/ui/empty-state";
import { TableSkeleton } from "@/components/ui/skeleton-presets";
import { ErrorState } from "@/components/ui/error-state";

import { useTranslations } from "next-intl";
import { RefreshCw, Mic } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { ADMIN_INTERVIEWS_PAGE_SIZE } from "../api/interviews";
import { useAdminInterviews } from "../hooks/use-admin-interviews";
import { InterviewFilters } from "./interview-filters";
import { InterviewsTable } from "./interviews-table";

export function AdminInterviewSessions() {
  const tCommon = useTranslations("common");
  const t = useTranslations("admin.interviews");
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
            <ErrorState
              variant="inline"
              description={t("sessions.loadFailed")}
              onRetry={() => void list.refetch()}
              isRetrying={list.isFetching}
              retryLabel={tCommon("actions.retry")}
            />
          )}
          {list.isPending ? (
            <TableSkeleton label={t("sessions.loadingLabel")} />
          ) : rows.length ? (
            <InterviewsTable rows={rows} />
          ) : (
            !list.error && (
              <EmptyState
                icons={[Mic]}
                title={
                  filtered
                    ? t("sessions.emptyFilteredTitle")
                    : t("sessions.emptyTitle")
                }
                description={
                  filtered
                    ? t("sessions.emptyFilteredHint")
                    : t("sessions.emptyHint")
                }
              />
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
