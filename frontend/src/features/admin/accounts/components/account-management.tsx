"use client";

import { useRef } from "react";
import { UsersRound, RefreshCw } from "lucide-react";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("admin.accounts");
  const groups = useTranslations("common.shell.nav.groups");
  const tCommon = useTranslations("common");
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
    <section className="min-w-0 space-y-7" aria-label={t("label")}>
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {groups("governance")}
        </p>
        <h1
          ref={heading}
          tabIndex={-1}
          className="text-3xl font-semibold tracking-tight outline-none sm:text-4xl"
        >
          {t("title")}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {t("description")}
        </p>
      </header>
      {accessDenied ? (
        <div className="space-y-3 rounded-xl border p-5">
          <p role="alert">{t("accessDenied")}</p>
          <Button
            variant="outline"
            className="h-11"
            onClick={() => void recoverAccess()}
          >
            {t("retryAccess")}
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
                ? t("status.loading")
                : filtered
                  ? t("status.matchingCount", { count: total })
                  : t("status.count", { count: total })}
            </p>
            <Button
              variant="ghost"
              className="h-11"
              disabled={list.isFetching || lockDialog.pending}
              onClick={() => void refresh()}
            >
              <RefreshCw className="size-4" aria-hidden />
              {t("status.refresh")}
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
                  {t("status.retryRefresh")}
                </Button>
              )}
            </div>
          )}
          {list.error && (
            <div
              role="alert"
              className="flex flex-wrap items-center justify-between gap-3 rounded-xl border p-5"
            >
              <p className="text-sm">{t("status.loadFailed")}</p>
              <Button
                className="h-11"
                variant="outline"
                disabled={list.isFetching}
                onClick={() => void refresh()}
              >
                {tCommon("actions.retry")}
              </Button>
            </div>
          )}
          {list.isPending ? (
            <div
              role="status"
              aria-label={t("status.loadingLabel")}
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
                  {filtered ? t("empty.filteredTitle") : t("empty.title")}
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  {filtered ? t("empty.filteredHint") : t("empty.hint")}
                </p>
                {filtered && (
                  <Button
                    variant="outline"
                    className="mt-5 h-11"
                    onClick={clearFilters}
                  >
                    {t("empty.clear")}
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
            label={t("status.pagination")}
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
