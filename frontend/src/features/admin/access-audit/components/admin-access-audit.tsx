"use client";

import { EmptyState } from "@/components/ui/empty-state";
import { TableSkeleton } from "@/components/ui/skeleton-presets";
import { ErrorState } from "@/components/ui/error-state";

import { useId, useState, type FormEvent } from "react";
import { useTranslations } from "next-intl";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw, ScrollText } from "lucide-react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import {
  DataTable,
  type DataTableColumn,
} from "@/components/data-table/data-table";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  ACCESS_AUDIT_PAGE_SIZE,
  accessAuditKeys,
  initialAccessAuditQuery,
  isAccessAuditDenied,
  listAccessAudit,
  type AccessAuditEntry,
  type AccessAuditQuery,
} from "../api/access-audit";

// KAN-296: who read a recording or a transcript, and when. Admin only; the content itself is never shown.
export function AdminAccessAudit() {
  const tCommon = useTranslations("common");
  const t = useTranslations("admin.accessAudit");
  const fmt = useFormatters();
  const ids = useId();
  const viewer = useSessionUser();
  const [query, setQuery] = useState<AccessAuditQuery>(initialAccessAuditQuery);
  const [draft, setDraft] = useState({ recruiterId: "", applicationId: "" });
  const [invalid, setInvalid] = useState<{
    recruiterId: boolean;
    applicationId: boolean;
  }>({ recruiterId: false, applicationId: false });
  const list = useQuery({
    queryKey: accessAuditKeys.list(viewer.id, query),
    queryFn: ({ signal }) => listAccessAudit(query, signal),
    enabled: viewer.role === "admin",
    staleTime: 0,
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessAuditDenied(list.error);
  const rows = list.data?.items ?? [];
  const total = list.data?.total ?? 0;
  const filtered = query.recruiterId !== null || query.applicationId !== null;

  const apply = (event: FormEvent) => {
    event.preventDefault();
    const recruiterId = draft.recruiterId.trim();
    const applicationId = draft.applicationId.trim();
    const bad = {
      recruiterId:
        recruiterId !== "" && !z.uuid().safeParse(recruiterId).success,
      applicationId:
        applicationId !== "" && !z.uuid().safeParse(applicationId).success,
    };
    setInvalid(bad);
    if (bad.recruiterId || bad.applicationId) return;
    setQuery({
      page: 1,
      recruiterId: recruiterId || null,
      applicationId: applicationId || null,
    });
  };
  const clear = () => {
    setDraft({ recruiterId: "", applicationId: "" });
    setInvalid({ recruiterId: false, applicationId: false });
    setQuery(initialAccessAuditQuery);
  };

  const columns: DataTableColumn<AccessAuditEntry>[] = [
    {
      id: "time",
      header: t("table.time"),
      cell: (row) => fmt.date(row.created_at, "moment"),
    },
    {
      id: "action",
      header: t("table.action"),
      cell: (row) => t(`action.${row.action}`),
    },
    {
      id: "recruiter",
      header: t("table.recruiter"),
      cell: (row) => (
        <span className="break-all font-mono text-xs">{row.recruiter_id}</span>
      ),
    },
    {
      id: "application",
      header: t("table.application"),
      cell: (row) => (
        <span className="break-all font-mono text-xs">
          {row.application_id}
        </span>
      ),
    },
    {
      id: "interview",
      header: t("table.interview"),
      cell: (row) => (
        <span className="break-all font-mono text-xs">{row.interview_id}</span>
      ),
    },
  ];

  return (
    <section className="min-w-0 space-y-7" aria-label={t("label")}>
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("title")}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {t("description")}
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
          <form
            aria-label={t("filters.label")}
            noValidate
            onSubmit={apply}
            className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-start"
          >
            {(
              [
                ["recruiterId", t("filters.recruiterId")],
                ["applicationId", t("filters.applicationId")],
              ] as const
            ).map(([name, label]) => (
              <div key={name} className="flex flex-col gap-2">
                <label
                  htmlFor={`${ids}-${name}`}
                  className="text-xs font-medium"
                >
                  {label}
                </label>
                <Input
                  id={`${ids}-${name}`}
                  className="h-11 bg-background shadow-none"
                  autoComplete="off"
                  disabled={list.isFetching}
                  value={draft[name]}
                  aria-invalid={invalid[name] ? true : undefined}
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      [name]: event.target.value,
                    }))
                  }
                />
                {invalid[name] && (
                  <p role="alert" className="text-xs text-destructive">
                    {t("filters.invalidId")}
                  </p>
                )}
              </div>
            ))}
            <div className="flex flex-wrap gap-2 sm:pt-6">
              <Button type="submit" className="h-11" disabled={list.isFetching}>
                {t("filters.apply")}
              </Button>
              <Button
                type="button"
                variant="outline"
                className="h-11"
                disabled={list.isFetching}
                onClick={clear}
              >
                {t("filters.clear")}
              </Button>
            </div>
          </form>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p
              role="status"
              aria-live="polite"
              className="text-sm text-muted-foreground"
            >
              {list.isFetching
                ? t("loading")
                : t("count", { total, filtered: String(filtered) })}
            </p>
            <Button
              variant="ghost"
              className="h-11"
              disabled={list.isFetching}
              onClick={() => void list.refetch()}
            >
              <RefreshCw className="size-4" aria-hidden />
              {t("refresh")}
            </Button>
          </div>
          {list.error && (
            <ErrorState
              variant="inline"
              description={t("loadFailed")}
              onRetry={() => void list.refetch()}
              isRetrying={list.isFetching}
              retryLabel={tCommon("actions.retry")}
            />
          )}
          {list.isPending ? (
            <TableSkeleton label={t("loadingLabel")} />
          ) : rows.length ? (
            <DataTable
              rows={rows}
              columns={columns}
              getRowId={(row) => row.id}
              label={t("table.label")}
            />
          ) : (
            !list.error && (
              <EmptyState
                icons={[ScrollText]}
                title={filtered ? t("emptyFilteredTitle") : t("emptyTitle")}
                description={filtered ? t("emptyFilteredHint") : t("emptyHint")}
              />
            )
          )}
          <DataTablePagination
            offset={(query.page - 1) * ACCESS_AUDIT_PAGE_SIZE}
            pageSize={ACCESS_AUDIT_PAGE_SIZE}
            total={total}
            disabled={list.isFetching}
            label={t("paginationLabel")}
            onPageChange={(page) =>
              setQuery((current) => ({ ...current, page }))
            }
          />
        </>
      )}
    </section>
  );
}
