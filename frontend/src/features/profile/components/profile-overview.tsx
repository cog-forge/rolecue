"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useCallback, useEffect } from "react";
import { isAxiosError } from "axios";
import {
  useQuery,
  useMutation,
  useQueryClient,
  useIsFetching,
} from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useSessionUser,
  type SessionUser,
} from "@/features/auth/components/session-gate";
import { getMyProfile, updateMyProfile, profileKeys } from "../api/profile";
import { ProfileForm } from "./profile-form";
import { ProfileSummary } from "./profile-summary";
import { apiErrorMessage } from "@/lib/i18n/api-errors";
import { useTranslations } from "next-intl";

function denied(error: unknown) {
  return (
    isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
  );
}
export function ProfileOverview() {
  const errorsT = useTranslations("errors");
  const t = useTranslations("common.profile");
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = profileKeys.me(user.id);
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getMyProfile(signal),
    refetchOnMount: "always",
    refetchOnWindowFocus: true,
    retry: false,
  });
  const mutation = useMutation({
    mutationFn: updateMyProfile,
    retry: false,
    onSuccess: async (profile) => {
      await cache.cancelQueries({ queryKey: key });
      cache.setQueryData(key, profile);
      cache.setQueryData<SessionUser>(
        ["auth", "me"],
        (current) =>
          current && {
            ...current,
            full_name: profile.full_name,
            image: profile.image,
          },
      );
      void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
  const { refetch } = query;
  const { reset } = mutation;
  const checkingSession = useIsFetching({ queryKey: ["auth", "me"] }) > 0;
  const accessDenied = denied(query.error) || denied(mutation.error);
  const refreshAccess = useCallback(async () => {
    await cache.invalidateQueries({ queryKey: ["auth", "me"] });
    const session = cache.getQueryState<SessionUser>(["auth", "me"]);
    // Cached identity alone is insufficient: the auth gate must have completed
    // a successful recheck before the rejected profile flow can resume.
    if (
      session?.status !== "success" ||
      session.isInvalidated ||
      session.fetchStatus !== "idle" ||
      session.data?.id !== user.id
    )
      return;
    const fresh = await refetch();
    if (fresh.isSuccess) reset();
  }, [cache, refetch, reset, user.id]);
  useEffect(() => {
    if (accessDenied) void refreshAccess();
  }, [accessDenied, refreshAccess]);
  let saveError: string | null = null;
  if (mutation.error) {
    saveError = apiErrorMessage(errorsT, mutation.error, t("saveFailed"));
  }
  return (
    <section className="mx-auto w-full max-w-5xl space-y-8">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("header.eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("header.title")}
        </h1>
        <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
          {t("header.description")}
        </p>
      </header>
      {accessDenied ? (
        <div className="space-y-4">
          <p role="status">
            {checkingSession || query.isFetching
              ? t("access.checking")
              : t("access.failed")}
          </p>
          <Button
            variant="outline"
            disabled={checkingSession || query.isFetching}
            onClick={() => void refreshAccess()}
          >
            {t("access.retry")}
          </Button>
        </div>
      ) : query.isPending ? (
        <div
          role="status"
          aria-label={t("loadingLabel")}
          className="grid gap-6 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]"
        >
          <Skeleton className="h-80 rounded-xl" />
          <div className="space-y-6">
            <Skeleton className="h-64 rounded-xl" />
            <Skeleton className="h-12" />
          </div>
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : !query.data ? (
        <ErrorState
          title={t("loadFailed.title")}
          description={t("loadFailed.description")}
          onRetry={() => void query.refetch()}
          retryLabel={t("loadFailed.retry")}
          isRetrying={query.isFetching}
        ></ErrorState>
      ) : (
        <div className="grid items-start gap-6 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]">
          <ProfileSummary profile={query.data} />
          <div className="min-w-0 space-y-4">
            {query.error && (
              <p role="alert" className="rounded-xl border p-4 text-sm">
                {t("refreshFailed")}{" "}
                <Button
                  variant="link"
                  disabled={query.isFetching}
                  onClick={() => void query.refetch()}
                >
                  {t("refreshRetry")}
                </Button>
              </p>
            )}
            <ProfileForm
              profile={query.data}
              saving={mutation.isPending}
              error={saveError}
              saved={mutation.isSuccess}
              savedProfile={mutation.data}
              onSave={async (patch) => {
                try {
                  await mutation.mutateAsync(patch);
                } catch {
                  /* Mutation state exposes the error and retains the form draft. */
                }
              }}
            />
          </div>
        </div>
      )}
    </section>
  );
}
