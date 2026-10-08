"use client";

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

function denied(error: unknown) {
  return (
    isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
  );
}
export function ProfileOverview() {
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
    const payload = isAxiosError<{ error?: { message?: unknown } }>(
      mutation.error,
    )
      ? mutation.error.response?.data
      : undefined;
    saveError =
      typeof payload?.error?.message === "string"
        ? payload.error.message
        : "We couldn't save your profile. Your changes are still here. Please try again.";
  }
  return (
    <section className="mx-auto w-full max-w-5xl space-y-8">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Your account
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Your profile
        </h1>
        <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
          A little about you. Keep your details current wherever you work in
          RoleCue.
        </p>
      </header>
      {accessDenied ? (
        <div className="space-y-4">
          <p role="status">
            {checkingSession || query.isFetching
              ? "Checking your account access…"
              : "We couldn't confirm your account access. Please try again."}
          </p>
          <Button
            variant="outline"
            disabled={checkingSession || query.isFetching}
            onClick={() => void refreshAccess()}
          >
            Retry
          </Button>
        </div>
      ) : query.isPending ? (
        <div
          role="status"
          aria-label="Loading your profile"
          className="grid gap-6 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]"
        >
          <Skeleton className="h-80 rounded-xl" />
          <div className="space-y-6">
            <Skeleton className="h-64 rounded-xl" />
            <Skeleton className="h-12" />
          </div>
          <span className="sr-only">Loading your profile…</span>
        </div>
      ) : !query.data ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">We couldn&apos;t load your profile</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please try again in a moment.
          </p>
          <Button
            className="mt-4"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Retry
          </Button>
        </div>
      ) : (
        <div className="grid items-start gap-6 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]">
          <ProfileSummary profile={query.data} />
          <div className="min-w-0 space-y-4">
            {query.error && (
              <p role="alert" className="rounded-xl border p-4 text-sm">
                Unable to refresh your profile. Your draft is still here.{" "}
                <Button
                  variant="link"
                  disabled={query.isFetching}
                  onClick={() => void query.refetch()}
                >
                  Retry
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
