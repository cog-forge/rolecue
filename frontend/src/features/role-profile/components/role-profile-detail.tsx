"use client";

import Link from "next/link";
import { isAxiosError } from "axios";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, LoaderCircle, RefreshCw } from "lucide-react";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import type { RoleProfile, RoleReview } from "../api/contract";
import { roleProfileKeys } from "../api/keys";
import {
  analyzeRoleProfile,
  confirmRoleProfile,
  getRoleProfile,
  roleError,
  saveRoleProfile,
} from "../api/requests";
import { OriginalRoleDocument } from "./original-role-document";
import { RoleProfileStatus } from "./role-profile-status";
import { RoleProfileSteps } from "./role-profile-steps";
import { RoleReviewForm } from "./role-review-form";
import { RoleReviewSummary } from "./role-review-summary";

type ReviewAction = {
  kind: "save" | "confirm";
  review: RoleReview;
};
export function RoleProfileDetail({ id }: { id: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = roleProfileKeys.detail(user.id, id);
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getRoleProfile(id, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const update = (profile: RoleProfile) => {
    cache.setQueryData(key, profile);
    void cache.invalidateQueries({ queryKey: roleProfileKeys.lists(user.id) });
  };
  const review = useMutation({
    mutationFn: async (action: ReviewAction) => {
      const saved = await saveRoleProfile(id, action.review);
      if (action.kind === "save") return saved;
      update(saved);
      return confirmRoleProfile(id);
    },
    onSuccess: update,
    onError: () => {
      void query.refetch();
    },
  });
  const prepare = useMutation({
    mutationFn: () => analyzeRoleProfile(id),
    onSuccess: update,
    onError: () => {
      void query.refetch();
    },
  });
  const busy = review.isPending || prepare.isPending;
  const profile = query.data;
  const error = review.error ?? prepare.error;
  const staleReview =
    isAxiosError(review.error) && review.error.response?.status === 409;
  if (query.isPending)
    return (
      <div
        role="status"
        aria-label="Loading Role Profile"
        className="mx-auto max-w-6xl space-y-6"
        aria-busy="true"
      >
        <Skeleton className="h-16 w-2/3" />
        <Skeleton className="h-80 rounded-2xl" />
      </div>
    );
  const denied =
    isAxiosError(query.error) &&
    [401, 403, 404].includes(query.error.response?.status ?? 0);
  if (!profile || denied)
    return (
      <section className="mx-auto max-w-6xl space-y-5">
        <h1 className="text-3xl font-semibold">Role Profile</h1>
        <p role="alert">{roleError(query.error)}</p>
        <Button
          variant="outline"
          className="h-11"
          onClick={() => void query.refetch()}
        >
          Try again
        </Button>
        <Link
          href={routes.roleProfiles.root}
          className="block text-sm underline"
        >
          Back to Role Profiles
        </Link>
      </section>
    );
  const reviewing = profile.status === "review";
  const analysis = profile.status === "draft" || profile.status === "analyzing";
  const running = profile.status === "analyzing";
  return (
    <section className="mx-auto w-full max-w-6xl space-y-8">
      <header className="space-y-4">
        <Link
          href={routes.roleProfiles.root}
          className="inline-flex min-h-11 items-center text-sm text-muted-foreground underline-offset-4 hover:underline"
        >
          Role Profiles
        </Link>
        <div className="flex flex-wrap items-center gap-3">
          <RoleProfileStatus status={profile.status} />
          <Button
            variant="ghost"
            className="h-11"
            disabled={query.isFetching || busy}
            onClick={() => {
              review.reset();
              prepare.reset();
              void query.refetch();
            }}
          >
            <RefreshCw aria-hidden="true" /> Refresh status
          </Button>
        </div>
        <h1 className="max-w-4xl text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
          {reviewing
            ? "Confirm the role before you practice."
            : analysis
              ? "Let’s find the role’s focus."
              : "Your Role Profile is confirmed."}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {reviewing
            ? "AI extraction is a starting point. Review, refine, and save what matters to you."
            : analysis
              ? "Your original job description is saved. Extract its technical requirements to begin your review."
              : "Your reviewed requirements, seniority and refinement note are saved for the next step."}
        </p>
        <RoleProfileSteps
          current={reviewing ? "review" : analysis ? "input" : "confirmed"}
        />
      </header>
      {query.error && (
        <p role="alert" className="text-sm text-destructive">
          Unable to refresh. Your current review is still here; try refreshing
          again.
        </p>
      )}
      {(!reviewing || staleReview) && error && (
        <p
          role="alert"
          className="rounded-xl border border-destructive/25 bg-destructive/5 p-4 text-sm"
        >
          {roleError(error)}
        </p>
      )}
      {reviewing ? (
        <RoleReviewForm
          key={profile.id}
          profile={profile}
          busy={busy}
          onSave={(values) =>
            review.mutateAsync({
              kind: "save",
              review: values,
            })
          }
          onConfirm={(values) =>
            review.mutateAsync({
              kind: "confirm",
              review: values,
            })
          }
        />
      ) : analysis ? (
        <div className="space-y-5 rounded-2xl border bg-card p-6 sm:p-8">
          <p role="status" className="text-sm text-muted-foreground">
            {busy || running
              ? "Analyzing technical requirements. You can refresh to check the saved status."
              : prepare.isError
                ? "Analysis didn’t finish. Your text is safe; try again."
                : "Ready to analyze your saved job description."}
          </p>
          <Button
            disabled={busy}
            className="h-11"
            onClick={() => prepare.mutate()}
          >
            {busy ? (
              <LoaderCircle
                className="motion-safe:animate-spin"
                aria-hidden="true"
              />
            ) : (
              <ArrowRight aria-hidden="true" />
            )}
            {running ? "Retry analysis" : "Analyze role"}
          </Button>
          <p className="text-xs text-muted-foreground">
            If an earlier attempt is still running, retry will keep that attempt
            and won’t start another.
          </p>
        </div>
      ) : (
        <RoleReviewSummary profile={profile} />
      )}
      <Accordion
        type="single"
        collapsible
        className="rounded-xl border bg-card px-5"
      >
        <AccordionItem value="original">
          <AccordionTrigger className="min-h-11 items-center">
            Original job description
          </AccordionTrigger>
          <AccordionContent>
            <p className="max-h-96 overflow-auto leading-relaxed whitespace-pre-wrap text-muted-foreground">
              {profile.rawText ||
                "Original text is not available for this legacy Role Profile."}
            </p>
          </AccordionContent>
        </AccordionItem>
      </Accordion>
      <OriginalRoleDocument profile={profile} />
    </section>
  );
}
