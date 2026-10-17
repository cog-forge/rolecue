"use client";

import { useRef } from "react";
import { UsersRound, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { ACCOUNTS_PAGE_SIZE } from "../api/accounts";
import { useAccountManagement } from "../hooks/use-account-management";
import { AccountsTable } from "./accounts-table";
import { AccountsToolbar } from "./accounts-toolbar";
import { AccountLockDialog } from "./account-lock-dialog";
import { AccountDetailsSheet } from "./account-details-sheet";

export function AccountManagement() {
  const {
    list,
    params,
    search,
    now,
    total,
    filtered,
    searchPending,
    actionsBlocked,
    notice,
    needsRefresh,
    accessDenied,
    refresh,
    recoverAccess,
    changeSearch,
    changeQuery,
    changePage,
    clearFilters,
    details,
    lockDialog,
  } = useAccountManagement();
  const heading = useRef<HTMLHeadingElement>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  const restoreFocus = () => {
    if (trigger.current?.isConnected && !trigger.current.disabled)
      trigger.current.focus();
    else heading.current?.focus();
  };
  return (
    <section className="min-w-0 space-y-7" aria-label="Account management">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Governance
        </p>
        <h1
          ref={heading}
          tabIndex={-1}
          className="text-3xl font-semibold tracking-tight outline-none sm:text-4xl"
        >
          Accounts
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Manage access to RoleCue. Review accounts and lock or unlock Candidate
          and Recruiter access.
        </p>
      </header>
      {accessDenied ? (
        <div className="space-y-3 rounded-xl border p-5">
          <p role="alert">
            We couldn&apos;t confirm your administrator access. Account data is
            hidden until access is verified.
          </p>
          <Button
            variant="outline"
            className="h-11"
            onClick={() => void recoverAccess()}
          >
            Retry access check
          </Button>
        </div>
      ) : (
        <>
          <AccountsToolbar
            query={params}
            search={search}
            onSearchChange={changeSearch}
            disabled={lockDialog.pending}
            onChange={changeQuery}
          />
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p
              role="status"
              aria-live="polite"
              className="text-sm text-muted-foreground"
            >
              {searchPending || list.isFetching
                ? "Loading accounts…"
                : `${total} ${filtered ? "matching " : ""}${total === 1 ? "account" : "accounts"}`}
            </p>
            <Button
              variant="ghost"
              className="h-11"
              disabled={list.isFetching || lockDialog.pending}
              onClick={() => void refresh()}
            >
              <RefreshCw className="size-4" aria-hidden />
              Refresh
            </Button>
          </div>
          {notice && (
            <div
              role="status"
              className="space-y-3 rounded-xl border p-4 text-sm"
            >
              <p>{notice}</p>
              {needsRefresh && (
                <Button
                  className="h-11"
                  variant="outline"
                  disabled={list.isFetching}
                  onClick={() => void refresh()}
                >
                  Retry refresh
                </Button>
              )}
            </div>
          )}
          {list.error && (
            <div
              role="alert"
              className="flex flex-wrap items-center justify-between gap-3 rounded-xl border p-5"
            >
              <p className="text-sm">
                Unable to load accounts. Please try again.
              </p>
              <Button
                className="h-11"
                variant="outline"
                disabled={list.isFetching}
                onClick={() => void refresh()}
              >
                Retry
              </Button>
            </div>
          )}
          {list.isPending ? (
            <div
              role="status"
              aria-label="Loading accounts"
              className="space-y-3 rounded-xl border p-5"
            >
              <Skeleton className="h-10" />
              {[0, 1, 2, 3].map((row) => (
                <Skeleton key={row} className="h-16" />
              ))}
            </div>
          ) : list.data?.users.length ? (
            <AccountsTable
              users={list.data.users}
              checkedAt={now}
              disabled={actionsBlocked}
              onAction={(user, button) => {
                trigger.current = button;
                lockDialog.open(user);
              }}
              onView={(user, button) => {
                trigger.current = button;
                details.open(user.id);
              }}
            />
          ) : (
            !list.error && (
              <div
                role="status"
                className="rounded-xl border bg-card px-5 py-16 text-center"
              >
                <UsersRound
                  className="mx-auto size-7 text-muted-foreground"
                  aria-hidden
                />
                <h2 className="mt-4 text-base font-medium">
                  {filtered ? "No matching accounts" : "No accounts yet"}
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  {filtered
                    ? "Try another search or clear your filter."
                    : "Registered accounts will appear here."}
                </p>
                {filtered && (
                  <Button
                    variant="outline"
                    className="mt-5 h-11"
                    onClick={clearFilters}
                  >
                    Clear search and filter
                  </Button>
                )}
              </div>
            )
          )}
          <DataTablePagination
            offset={params.offset}
            pageSize={ACCOUNTS_PAGE_SIZE}
            total={total}
            disabled={actionsBlocked}
            label="Accounts pagination"
            onPageChange={changePage}
          />
        </>
      )}
      {details.id && !accessDenied && (
        <AccountDetailsSheet
          key={details.id}
          id={details.id}
          onClose={details.close}
          onRestoreFocus={restoreFocus}
          onAccessError={recoverAccess}
        />
      )}
      {lockDialog.account && !accessDenied && (
        <AccountLockDialog
          key={lockDialog.account.id}
          user={lockDialog.account}
          pending={lockDialog.pending}
          blocked={lockDialog.blocked}
          error={lockDialog.error}
          onCancel={lockDialog.close}
          onRestoreFocus={restoreFocus}
          onConfirm={lockDialog.confirm}
        />
      )}
    </section>
  );
}
