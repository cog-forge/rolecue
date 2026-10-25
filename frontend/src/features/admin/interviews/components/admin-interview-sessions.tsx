"use client";

import { RefreshCw, Mic } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { ADMIN_INTERVIEWS_PAGE_SIZE } from "../api/interviews";
import { useAdminInterviews } from "../hooks/use-admin-interviews";
import { InterviewFilters } from "./interview-filters";
import { InterviewsTable } from "./interviews-table";

export function AdminInterviewSessions() {
  const { list, query, denied, applyFilters, changePage } =
    useAdminInterviews();
  const total = list.data?.total ?? 0;
  const filtered =
    query.type !== null || !!query.candidateId || !!query.jobPostingId;
  const rows = list.data?.items ?? [];
  return (
    <section className="min-w-0 space-y-7" aria-label="Interview sessions">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Operations
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Interview sessions
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Find interview sessions and review their operational details. Session
          content is not shown here.
        </p>
      </header>
      {denied ? (
        <div className="space-y-3 rounded-xl border p-5">
          <p role="alert">
            We couldn&apos;t confirm your administrator access. Interview data
            is hidden until access is verified.
          </p>
          <Button
            variant="outline"
            className="h-11"
            disabled={list.isFetching}
            onClick={() => void list.refetch()}
          >
            Retry access check
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
                ? "Loading interview sessions…"
                : `${total} ${filtered ? "matching " : ""}${total === 1 ? "interview session" : "interview sessions"}`}
            </p>
            <Button
              variant="ghost"
              className="h-11"
              disabled={list.isFetching}
              onClick={() => void list.refetch()}
            >
              <RefreshCw className="size-4" aria-hidden />
              Refresh
            </Button>
          </div>
          {list.error && (
            <div
              role="alert"
              className="flex flex-wrap items-center justify-between gap-3 rounded-xl border p-5"
            >
              <p className="text-sm">
                Unable to load interview sessions. Please try again.
              </p>
              <Button
                className="h-11"
                variant="outline"
                disabled={list.isFetching}
                onClick={() => void list.refetch()}
              >
                Retry
              </Button>
            </div>
          )}
          {list.isPending ? (
            <div
              role="status"
              aria-label="Loading interview sessions"
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
                    ? "No matching interview sessions"
                    : "No interview sessions yet"}
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  {filtered
                    ? "Try other filters or clear them."
                    : "Interview sessions will appear here once candidates start them."}
                </p>
              </div>
            )
          )}
          <DataTablePagination
            offset={(query.page - 1) * ADMIN_INTERVIEWS_PAGE_SIZE}
            pageSize={ADMIN_INTERVIEWS_PAGE_SIZE}
            total={total}
            disabled={list.isFetching}
            label="Interview sessions pagination"
            onPageChange={changePage}
          />
        </>
      )}
    </section>
  );
}
