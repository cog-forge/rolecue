"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
import { cn } from "@/lib/utils";
import type { RoleSummary } from "../api/contract";
import { roleProfileKeys } from "../api/keys";
import {
  deleteRoleProfile,
  listRoleProfiles,
  ROLE_PROFILES_PAGE_SIZE,
  roleError,
} from "../api/requests";
import { RoleProfileStatus } from "./role-profile-status";

export function RoleProfileLibrary() {
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
      header: "Role",
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
          <span
            className={cn(
              "mt-1 text-sm text-muted-foreground",
              profile.seniorityLevel && "capitalize",
            )}
          >
            {profile.seniorityLevel ?? "Seniority to review"}
          </span>
        </Link>
      ),
    },
    {
      id: "status",
      header: "Status",
      className: "w-36 px-5 py-4 align-middle",
      cell: (profile) => <RoleProfileStatus status={profile.status} />,
    },
    {
      id: "created",
      header: "Created",
      className: "w-36 px-5 py-4 align-middle",
      cell: (profile) => (
        <time
          dateTime={profile.createdAt}
          className="text-xs text-muted-foreground"
        >
          {new Date(profile.createdAt).toLocaleDateString("en-GB", {
            day: "numeric",
            month: "short",
            year: "numeric",
          })}
        </time>
      ),
    },
    {
      id: "actions",
      header: "Actions",
      className: "w-16 px-5 py-4 align-middle",
      cell: (profile) => (
        <Button
          variant="ghost"
          size="icon"
          className="size-11 text-muted-foreground hover:text-destructive"
          aria-label={`Delete ${profile.title}`}
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
            Your practice context
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
            Role Profiles
          </h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            Keep your Role Profiles, review their requirements, and pick up
            where you left off. Your Role Profiles stay private.
          </p>
        </div>
        <Button asChild className="h-11">
          <Link href={routes.roleProfiles.new} ref={createLink}>
            <Plus aria-hidden="true" /> Add Role Profile
          </Link>
        </Button>
      </header>
      {query.isPending ? (
        <div
          role="status"
          aria-label="Loading Role Profiles"
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
            className="h-11"
            onClick={() => void query.refetch()}
          >
            Try again
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
            Your next role starts here.
          </h2>
          <p className="mt-3 max-w-md text-sm leading-relaxed text-muted-foreground">
            Create a Role Profile from a job description, review what matters,
            and shape the focus of your next practice interview.
          </p>
          <Button asChild className="mt-7 h-11">
            <Link href={routes.roleProfiles.new}>
              Bring in a role <ArrowUpRight aria-hidden="true" />
            </Link>
          </Button>
        </div>
      ) : (
        <>
          <DataTable
            rows={query.data.items}
            columns={columns}
            getRowId={(profile) => profile.id}
            label="Role Profiles"
          />
          {query.data.pagination.total > ROLE_PROFILES_PAGE_SIZE && (
            <DataTablePagination
              offset={offset}
              pageSize={ROLE_PROFILES_PAGE_SIZE}
              total={query.data.pagination.total}
              disabled={query.isFetching}
              label="Role Profile pages"
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
            <DialogTitle>Delete this Role Profile?</DialogTitle>
            <DialogDescription>
              {selected?.title} and its saved requirements will be removed. Role
              Profiles with interview history cannot be deleted.
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
              className="h-11"
              disabled={remove.isPending}
              onClick={() => setSelected(null)}
            >
              Keep profile
            </Button>
            <Button
              variant="destructive"
              className="h-11"
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
              {remove.isPending ? "Deleting…" : "Delete Role Profile"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
