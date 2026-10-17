"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { gooeyToast } from "goey-toast";
import {
  useSessionUser,
  type SessionUser,
} from "@/features/auth/components/session-gate";
import {
  ACCOUNTS_PAGE_SIZE,
  accountsKeys,
  isAdminAccessError,
  getAccountActionError,
  listAccounts,
  updateAccountLock,
  type AccountList,
  type AccountQuery,
  type AdminAccount,
  type AccountAction,
} from "../api/accounts";
import { useAccountClock } from "./use-account-clock";
import { isAccountLocked } from "../schemas/account-state";

const initialQuery: AccountQuery = {
  offset: 0,
  searchValue: "",
  filter: "all",
};

// Owns remote state, transition reconciliation and access recovery for the Accounts screen.
export function useAccountManagement() {
  const viewer = useSessionUser();
  const cache = useQueryClient();
  const [params, setParams] = useState(initialQuery);
  const [search, setSearch] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => {
      setParams((current) =>
        current.searchValue === search.trim()
          ? current
          : { ...current, offset: 0, searchValue: search.trim() },
      );
    }, 300);
    return () => clearTimeout(timer);
  }, [search]);
  const [selected, setSelected] = useState<AdminAccount | null>(null);
  const [viewId, setViewId] = useState<string | null>(null);
  const [needsRefresh, setNeedsRefresh] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [accessBlocked, setAccessBlocked] = useState(false);
  const submitting = useRef(false);
  const list = useQuery({
    queryKey: accountsKeys.list(viewer.id, params),
    queryFn: ({ signal }) => listAccounts(params, signal),
    enabled: viewer.role === "admin" && !accessBlocked,
    staleTime: 0,
    refetchOnMount: "always",
    refetchOnWindowFocus: true,
    retry: false,
  });
  const now = useAccountClock(list.data?.users ?? [], list.dataUpdatedAt);
  const searchPending = search.trim() !== params.searchValue;
  const { refetch } = list;
  useEffect(() => {
    if (
      params.filter === "locked" &&
      list.data?.users.some(
        (user) => user.banned && !isAccountLocked(user, now),
      )
    )
      void refetch();
  }, [now, params.filter, list.data, refetch]);
  const reconcile = async () => {
    const fresh = await refetch();
    if (!fresh.isSuccess) {
      setNeedsRefresh(true);
      return false;
    }
    setNeedsRefresh(false);
    if (selected) {
      const current = fresh.data.users.find((user) => user.id === selected.id);
      if (
        !current ||
        isAccountLocked(current, now) !== selected.banned ||
        current.role !== selected.role
      )
        setSelected(null);
    }
    return true;
  };
  const mutation = useMutation({
    mutationFn: updateAccountLock,
    retry: false,
    onMutate: async () => {
      setNotice(null);
      await cache.cancelQueries({ queryKey: accountsKeys.viewer(viewer.id) });
    },
    onSuccess: async (result, action) => {
      const { user, changed, notification } = result;
      await cache.cancelQueries({ queryKey: accountsKeys.viewer(viewer.id) });
      cache.setQueriesData<AccountList>(
        { queryKey: accountsKeys.lists(viewer.id) },
        (old) =>
          old && {
            ...old,
            users: old.users.map((row) => (row.id === user.id ? user : row)),
          },
      );
      await cache.invalidateQueries({
        queryKey: accountsKeys.viewer(viewer.id),
        refetchType: "none",
      });
      setSelected(null);
      gooeyToast.success(
        !changed
          ? "Account already has this access state"
          : action.kind === "lock"
            ? "Account locked"
            : "Account unlocked",
        {
          description: !changed
            ? "No additional change or notification email was needed."
            : action.kind === "lock"
              ? "The user's active sessions have been revoked."
              : "The user can sign in again. Previous sessions remain invalid.",
        },
      );
      const refreshed = await reconcile();
      const notices: string[] = [];
      if (notification?.status === "failed")
        notices.push(
          "The account was updated, but the notification email could not be sent. The access change is still in effect.",
        );
      if (!refreshed)
        notices.push(
          "We couldn't refresh the list. Refresh before making another change.",
        );
      if (notices.length) setNotice(notices.join(" "));
    },
    onError: async (error) => {
      if (isAdminAccessError(error)) return;
      if (
        !isAxiosError(error) ||
        !error.response ||
        error.response.status >= 500
      ) {
        setNeedsRefresh(true);
        setNotice(
          "We couldn't confirm the change. Review the refreshed account state before trying again.",
        );
        await cache.invalidateQueries({
          queryKey: accountsKeys.viewer(viewer.id),
          refetchType: "none",
        });
        await reconcile();
      }
    },
  });
  const { reset: resetMutation } = mutation;
  const denied =
    isAdminAccessError(list.error) || isAdminAccessError(mutation.error);
  const recoverAccess = useCallback(
    () =>
      cache
        .cancelQueries({ queryKey: accountsKeys.viewer(viewer.id) })
        .then(async () => {
          setAccessBlocked(true);
          setSelected(null);
          setViewId(null);
          cache.removeQueries({ queryKey: accountsKeys.viewer(viewer.id) });
          await cache.invalidateQueries({ queryKey: ["auth", "me"] });
          const session = cache.getQueryState<SessionUser>(["auth", "me"]);
          if (
            session?.status !== "success" ||
            session.isInvalidated ||
            session.fetchStatus !== "idle" ||
            session.data?.id !== viewer.id ||
            session.data.role !== "admin"
          )
            return;
          const fresh = await refetch();
          if (fresh.isSuccess) {
            resetMutation();
            setAccessBlocked(false);
          }
        }),
    [cache, viewer.id, refetch, resetMutation],
  );
  useEffect(() => {
    if (denied) void recoverAccess();
  }, [denied, recoverAccess]);

  if (list.isSuccess && params.offset > 0 && params.offset >= list.data.total) {
    setParams({
      ...params,
      offset:
        Math.max(0, Math.ceil(list.data.total / ACCOUNTS_PAGE_SIZE) - 1) *
        ACCOUNTS_PAGE_SIZE,
    });
  }
  const filtered = !!params.searchValue || params.filter !== "all";
  const actionsBlocked =
    searchPending ||
    list.isFetching ||
    !!list.error ||
    needsRefresh ||
    mutation.isPending;
  const total = list.data?.total ?? 0;
  return {
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
    accessDenied: denied || accessBlocked,
    refresh: reconcile,
    recoverAccess,
    changeSearch: (value: string) => {
      setSearch(value);
      setNotice(null);
    },
    changeQuery: (query: AccountQuery) => {
      setParams(query);
      setNotice(null);
    },
    changePage: (page: number) =>
      setParams({ ...params, offset: (page - 1) * ACCOUNTS_PAGE_SIZE }),
    clearFilters: () => {
      setSearch("");
      setParams(initialQuery);
    },
    details: {
      id: viewId,
      open: (id: string) => setViewId(id),
      close: () => setViewId(null),
    },
    lockDialog: {
      account: selected,
      pending: mutation.isPending,
      blocked: needsRefresh || !!list.error || list.isFetching,
      error: mutation.error ? getAccountActionError(mutation.error) : null,
      open: (user: AdminAccount) => {
        mutation.reset();
        setSelected({ ...user, banned: isAccountLocked(user, now) });
      },
      close: () => {
        setSelected(null);
        mutation.reset();
      },
      confirm: async (action: AccountAction) => {
        if (submitting.current) return;
        submitting.current = true;
        try {
          await mutation.mutateAsync(action);
        } catch {
          /* Mutation state presents the error. */
        } finally {
          submitting.current = false;
        }
      },
    },
  };
}
