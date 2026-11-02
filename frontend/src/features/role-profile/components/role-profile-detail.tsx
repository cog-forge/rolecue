"use client";

import Link from "next/link";
import { isAxiosError } from "axios";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
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
  saveRoleProfile,
} from "../api/requests";
import { useRoleError } from "../lib/use-role-error";
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
  const t = useTranslations("candidate.roleProfiles.detail");
  const roleError = useRoleError();
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
        aria-label={t("loading")}
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
        <h1 className="text-3xl font-semibold">{t("heading")}</h1>
        <p role="alert">{roleError(query.error)}</p>
        <Button
          variant="outline"
          className="h-auto min-h-11 py-2 whitespace-normal"
          onClick={() => void query.refetch()}
        >
          {t("retry")}
        </Button>
        <Link
          href={routes.roleProfiles.root}
          className="block text-sm underline"
        >
          {t("backToLibrary")}
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
          {t("back")}
        </Link>
        <div className="flex flex-wrap items-center gap-3">
          <RoleProfileStatus status={profile.status} />
          <Button
            variant="ghost"
            className="h-auto min-h-11 py-2 whitespace-normal"
            disabled={query.isFetching || busy}
            onClick={() => {
              review.reset();
              prepare.reset();
              void query.refetch();
            }}
          >
            <RefreshCw aria-hidden="true" /> {t("refresh")}
          </Button>
        </div>
        <h1 className="max-w-4xl text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
          {t(
            `title.${reviewing ? "review" : analysis ? "analysis" : "confirmed"}`,
          )}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {t(
            `description.${reviewing ? "review" : analysis ? "analysis" : "confirmed"}`,
          )}
        </p>
        <RoleProfileSteps
          current={reviewing ? "review" : analysis ? "input" : "confirmed"}
        />
      </header>
      {query.error && (
        <p role="alert" className="text-sm text-destructive">
          {t("refreshFailed")}
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
              ? t("analyzing")
              : prepare.isError
                ? t("analysisFailed")
                : t("ready")}
          </p>
          <Button
            disabled={busy}
            className="h-auto min-h-11 py-2 whitespace-normal"
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
            {running ? t("retryAnalysis") : t("analyze")}
          </Button>
          <p className="text-xs text-muted-foreground">{t("retryHint")}</p>
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
            {t("originalTitle")}
          </AccordionTrigger>
          <AccordionContent>
            <p className="max-h-96 overflow-auto leading-relaxed whitespace-pre-wrap text-muted-foreground">
              {profile.rawText || t("originalMissing")}
            </p>
          </AccordionContent>
        </AccordionItem>
      </Accordion>
      <OriginalRoleDocument profile={profile} />
    </section>
  );
}
