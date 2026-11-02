"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import {
  ArrowUpRight,
  FileText,
  LoaderCircle,
  Plus,
  Trash2,
} from "lucide-react";
import {
  DataTable,
  type DataTableColumn,
} from "@/components/data-table/data-table";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import type { RoleSummary } from "../api/contract";
import { roleProfileKeys } from "../api/keys";
import {
  deleteRoleProfile,
  listRoleProfiles,
  ROLE_PROFILES_PAGE_SIZE,
} from "../api/requests";
import { seniorityLabel } from "../lib/labels";
import { useRoleError } from "../lib/use-role-error";
import { RoleProfileStatus } from "./role-profile-status";
import { useFormatters } from "@/lib/i18n/use-formatters";

export function RoleProfileLibrary() {
  const t = useTranslations("candidate.roleProfiles");
  const labels = useTranslations("candidate.roleProfiles.labels");
  const roleError = useRoleError();
  const fmt = useFormatters();
  const user = useSessionUser();
  const cache = useQueryClient();
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<RoleSummary | null>(null);
  const deleteTrigger = useRef<HTMLButtonElement | null>(null);
  const createLink = useRef<HTMLAnchorElement | null>(null);
  const query = useQuery({
    queryKey: roleProfileKeys.list(user.id, offset),
    queryFn: ({ signal }) => listRoleProfiles(offset, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const remove = useMutation({
    mutationFn: deleteRoleProfile,
    onSuccess: async (_, id) => {
      // The opener disappears when the list refetches; restore focus to a stable action.
      deleteTrigger.current = null;
      setSelected(null);
      if (offset > 0 && query.data?.items.length === 1)
        setOffset(Math.max(0, offset - ROLE_PROFILES_PAGE_SIZE));
      cache.removeQueries({ queryKey: roleProfileKeys.detail(user.id, id) });
      await cache.invalidateQueries({
        queryKey: roleProfileKeys.lists(user.id),
      });
    },
  });
  const columns: DataTableColumn<RoleSummary>[] = [
    {
      id: "role",
      header: t("library.columns.role"),
      cell: (profile) => (
        <Link
          href={routes.roleProfiles.detail(profile.id)}
          className="group flex min-h-11 flex-col justify-center rounded-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          <span className="flex items-center gap-2 font-semibold group-hover:underline group-hover:underline-offset-4">
            <span className="min-w-0 break-words">{profile.title}</span>
            <ArrowUpRight
              className="size-4 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
          </span>
          <span className="mt-1 text-sm text-muted-foreground">
            {profile.seniorityLevel
              ? seniorityLabel(labels, profile.seniorityLevel)
              : t("library.seniorityToReview")}
          </span>
        </Link>
      ),
    },
    {
      id: "status",
      header: t("library.columns.status"),
      className: "w-44 px-5 py-4 align-middle",
      cell: (profile) => <RoleProfileStatus status={profile.status} />,
    },
    {
      id: "created",
      header: t("library.columns.created"),
      className: "w-36 px-5 py-4 align-middle",
      cell: (profile) => (
        <time
          dateTime={profile.createdAt}
          className="text-xs text-muted-foreground"
        >
          {fmt.date(profile.createdAt, "dayMonthYear")}
        </time>
      ),
    },
    {
      id: "actions",
      header: t("library.columns.actions"),
      className: "w-16 px-5 py-4 align-middle",
      cell: (profile) => (
        <Button
          variant="ghost"
          size="icon"
          className="size-11 text-muted-foreground hover:text-destructive"
          aria-label={t("library.deleteLabel", { title: profile.title })}
          onClick={(event) => {
            deleteTrigger.current = event.currentTarget;
            remove.reset();
            setSelected(profile);
          }}
        >
          <Trash2 aria-hidden="true" />
        </Button>
      ),
    },
  ];
  return (
    <section className="mx-auto w-full max-w-6xl space-y-8">
      <header className="flex flex-wrap items-end justify-between gap-5">
        <div className="space-y-3">
          <p className="text-xs font-semibold tracking-widest text-workspace-accent uppercase">
            {t("library.eyebrow")}
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
            {t("library.heading")}
          </h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            {t("library.description")}
          </p>
        </div>
        <Button asChild className="h-auto min-h-11 py-2 whitespace-normal">
          <Link href={routes.roleProfiles.new} ref={createLink}>
            <Plus aria-hidden="true" /> {t("library.add")}
          </Link>
        </Button>
      </header>
      {query.isPending ? (
        <div
          role="status"
          aria-label={t("library.loading")}
          className="space-y-3"
        >
          <Skeleton className="h-24 rounded-xl" />
          <Skeleton className="h-24 rounded-xl" />
        </div>
      ) : query.error ? (
        <div role="alert" className="space-y-4 rounded-xl border p-6">
          <p>{roleError(query.error)}</p>
          <Button
            variant="outline"
            className="h-auto min-h-11 py-2 whitespace-normal"
            onClick={() => void query.refetch()}
          >
            {t("library.retry")}
          </Button>
        </div>
      ) : query.data.items.length === 0 ? (
        <div className="flex flex-col items-center rounded-2xl border bg-card px-6 py-16 text-center sm:py-24">
          <div className="mb-6 flex size-16 items-center justify-center rounded-2xl bg-workspace-accent/10">
            <FileText
              className="size-7 text-workspace-accent"
              aria-hidden="true"
            />
          </div>
          <h2 className="text-2xl font-semibold tracking-tight">
            {t("library.empty.title")}
          </h2>
          <p className="mt-3 max-w-md text-sm leading-relaxed text-muted-foreground">
            {t("library.empty.description")}
          </p>
          <Button
            asChild
            className="mt-7 h-auto min-h-11 py-2 whitespace-normal"
          >
            <Link href={routes.roleProfiles.new}>
              {t("library.empty.cta")} <ArrowUpRight aria-hidden="true" />
            </Link>
          </Button>
        </div>
      ) : (
        <>
          <DataTable
            rows={query.data.items}
            columns={columns}
            getRowId={(profile) => profile.id}
            label={t("library.tableLabel")}
          />
          {query.data.pagination.total > ROLE_PROFILES_PAGE_SIZE && (
            <DataTablePagination
              offset={offset}
              pageSize={ROLE_PROFILES_PAGE_SIZE}
              total={query.data.pagination.total}
              disabled={query.isFetching}
              label={t("library.pagesLabel")}
              onPageChange={(page) =>
                setOffset((page - 1) * ROLE_PROFILES_PAGE_SIZE)
              }
            />
          )}
        </>
      )}
      <Dialog
        open={Boolean(selected)}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setSelected(null);
        }}
      >
        <DialogContent
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            // Several rows open this dialog; return focus to the actual opener.
            if (deleteTrigger.current?.isConnected)
              deleteTrigger.current.focus();
            else createLink.current?.focus();
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("library.delete.title")}</DialogTitle>
            <DialogDescription>
              {t("library.delete.description", {
                title: selected?.title ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          {remove.error && (
            <p role="alert" className="text-sm text-destructive">
              {roleError(remove.error)}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              className="h-auto min-h-11 py-2 whitespace-normal"
              disabled={remove.isPending}
              onClick={() => setSelected(null)}
            >
              {t("library.delete.keep")}
            </Button>
            <Button
              variant="destructive"
              className="h-auto min-h-11 py-2 whitespace-normal"
              disabled={remove.isPending}
              onClick={() => {
                if (selected) remove.mutate(selected.id);
              }}
            >
              {remove.isPending && (
                <LoaderCircle
                  className="motion-safe:animate-spin"
                  aria-hidden="true"
                />
              )}
              {remove.isPending
                ? t("library.delete.pending")
                : t("library.delete.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
