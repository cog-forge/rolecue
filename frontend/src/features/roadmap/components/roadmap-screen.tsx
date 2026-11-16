"use client";
import { EmptyState } from "@/components/ui/empty-state";
import { AiLoader } from "@/components/ui/ai-loader";
import { ErrorState } from "@/components/ui/error-state";
import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { readApiError } from "@/lib/i18n/api-errors";
import {
  ArrowLeft,
  ArrowRight,
  ChartGantt,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getRoleProfile } from "@/features/role-profile/api/requests";
import { roleProfileKeys } from "@/features/role-profile/api/keys";
import { setupHref } from "@/features/interview-setup/lib/setup-params";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { prepareRoadmap, roadmapKeys, type Roadmap } from "../api/roadmap";
import { useRoadmap } from "../lib/use-roadmap";
import { RoadmapGantt } from "./roadmap-gantt";

function errorCode(error: unknown) {
  return readApiError(error).code;
}
export function RoadmapUpdateNotice({ roadmap }: { roadmap: Roadmap }) {
  const t = useTranslations("candidate.roadmap");
  if (roadmap.update_state === "up_to_date") return null;
  return (
    <p
      role="status"
      className="rounded-lg border border-amber-600/25 bg-amber-500/5 px-4 py-3 text-sm leading-relaxed"
    >
      {t(`update.${roadmap.update_state}`, {
        count: roadmap.pending_result_count,
      })}
    </p>
  );
}
export function RoadmapScreen({ profileId }: { profileId: string }) {
  const t = useTranslations("candidate.roadmap");
  const user = useSessionUser();
  const cache = useQueryClient();
  const fmt = useFormatters();
  const errorMessage = useApiErrorMessage();
  const [confirming, setConfirming] = useState(false);
  const [generationBlocked, setGenerationBlocked] = useState(false);
  const prepare = useMutation({
    mutationFn: (action: "retry" | "regenerate") =>
      prepareRoadmap(profileId, action),
    onSettled: () => {
      setConfirming(false);
      void cache.invalidateQueries({
        queryKey: roadmapKeys.all(user.id, profileId),
      });
    },
    onError: (error) => {
      if (errorCode(error) === "ROADMAP_REGENERATE_NOT_ALLOWED")
        setGenerationBlocked(true);
    },
  });
  const query = useRoadmap(user.id, profileId, prepare.isPending);
  const profile = useQuery({
    queryKey: roleProfileKeys.detail(user.id, profileId),
    queryFn: ({ signal }) => getRoleProfile(profileId, signal),
    retry: false,
  });
  const data = query.data;
  const code = errorCode(prepare.error) ?? errorCode(query.error);
  const legacy =
    code === "ROADMAP_NOT_RETRYABLE" ||
    (data?.generation_status === "pending" && data.practice_started);
  const missing = code === "JD_NOT_FOUND";
  const denied = code === "FORBIDDEN" || code === "INVALID_TOKEN";
  const ready = data?.generation_status === "ready";
  const running =
    prepare.isPending ||
    (data?.generation_status === "generating" && !query.generationTimedOut);
  const canPrepare =
    profile.data?.status === "confirmed" &&
    !legacy &&
    !generationBlocked &&
    !denied &&
    !missing;
  return (
    <section className="mx-auto w-full max-w-6xl space-y-7 pb-8">
      <Link
        href={routes.roleProfiles.detail(profileId)}
        className="inline-flex min-h-11 items-center gap-2 text-sm text-muted-foreground underline-offset-4 hover:underline"
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {t("backToProfile")}
      </Link>
      <header className="flex flex-wrap items-end justify-between gap-5">
        <div className="space-y-3">
          <p className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <ChartGantt
              className="size-4 text-rolecue-brand"
              aria-hidden="true"
            />
            {profile.data?.title ?? t("eyebrow")}
          </p>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            {t("heading")}
          </h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            {t("intro")}
          </p>
        </div>
        <div className="flex w-full flex-wrap gap-2 sm:w-auto">
          <Button
            variant="outline"
            className="min-h-11"
            disabled={query.isFetching || running}
            onClick={() => {
              prepare.reset();
              void query.refetch();
            }}
          >
            <RefreshCw
              className={query.isFetching ? "motion-safe:animate-spin" : ""}
              aria-hidden="true"
            />
            {t("refresh")}
          </Button>
          {ready && !running && (
            <Button asChild className="h-auto min-h-11 whitespace-normal py-2">
              <Link
                href={setupHref(routes.interviews.new.interviewer, {
                  roleProfileId: profileId,
                })}
              >
                {t("practiceGeneral")}
                <ArrowRight aria-hidden="true" />
              </Link>
            </Button>
          )}
        </div>
      </header>
      {query.isPending && !data ? (
        <div role="status" aria-busy="true" className="space-y-4">
          <Skeleton className="h-32 rounded-xl" />
          <Skeleton className="h-96 rounded-xl" />
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : missing || denied ? (
        <ErrorState
          variant="compact"
          description={errorMessage(query.error, t("loadFailed"))}
        >
          <Button asChild variant="outline" className="min-h-11">
            <Link href={denied ? routes.login : routes.roleProfiles.root}>
              {t(denied ? "signIn" : "backToLibrary")}
            </Link>
          </Button>
        </ErrorState>
      ) : !data && query.isError ? (
        <ErrorState
          description={errorMessage(query.error, t("loadFailed"))}
          onRetry={() => void query.refetch()}
          isRetrying={query.isFetching}
          retryLabel={t("retry")}
        />
      ) : legacy ? (
        <div className="space-y-4 rounded-xl border p-6">
          <h2 className="text-lg font-semibold">{t("legacyTitle")}</h2>
          <p className="max-w-xl text-sm text-muted-foreground">
            {t("legacyDescription")}
          </p>
          <Button asChild className="min-h-11">
            <Link href={routes.roleProfiles.new}>{t("createProfile")}</Link>
          </Button>
        </div>
      ) : (
        <>
          {(query.error || prepare.error) && code !== "ROADMAP_GENERATING" && (
            <ErrorState
              variant="compact"
              description={errorMessage(
                prepare.error ?? query.error,
                t("loadFailed"),
              )}
            />
          )}
          {running ? (
            <AiLoader
              stage="generating"
              title={t("preparingTitle")}
              description={t("preparingDescription")}
            />
          ) : !ready ? (
            query.generationTimedOut || data?.generation_status === "failed" ? (
              <ErrorState
                title={t(
                  query.generationTimedOut ? "stalledTitle" : "failedTitle",
                )}
                description={t(
                  query.generationTimedOut
                    ? "stalledDescription"
                    : "failedDescription",
                )}
                onRetry={
                  canPrepare && data ? () => prepare.mutate("retry") : undefined
                }
                retryLabel={t("prepare")}
                isRetrying={prepare.isPending}
              />
            ) : (
              <EmptyState
                icons={[ChartGantt]}
                title={t("pendingTitle")}
                description={t("pendingDescription")}
              >
                {canPrepare && data && (
                  <Button
                    className="mt-4 min-h-11"
                    onClick={() => prepare.mutate("retry")}
                  >
                    <RotateCcw aria-hidden="true" />
                    {t("prepare")}
                  </Button>
                )}
                {profile.data && profile.data.status !== "confirmed" && (
                  <Button asChild className="mt-4 min-h-11">
                    <Link href={routes.roleProfiles.detail(profileId)}>
                      {t("confirmRequirements")}
                    </Link>
                  </Button>
                )}
              </EmptyState>
            )
          ) : (
            data && (
              <>
                <RoadmapUpdateNotice roadmap={data} />
                {data.items.length ? (
                  <>
                    <div className="grid gap-6 border-y py-6 sm:grid-cols-[1fr_auto] sm:items-center">
                      <div>
                        <div className="mb-3 flex items-end justify-between gap-3">
                          <p className="text-sm font-medium">
                            {t("progress", {
                              count: data.completed_items,
                              total: data.total_items,
                            })}
                          </p>
                          <span className="text-2xl font-semibold tracking-tight tabular-nums">
                            {data.progress_percent === null
                              ? "—"
                              : fmt.percent(data.progress_percent)}
                          </span>
                        </div>
                        <div
                          role="progressbar"
                          aria-label={t("progressLabel")}
                          aria-valuenow={data.progress_percent ?? undefined}
                          aria-valuemin={0}
                          aria-valuemax={100}
                          className="h-1.5 overflow-hidden rounded-full bg-muted"
                        >
                          <div
                            style={{ width: `${data.progress_percent ?? 0}%` }}
                            className="h-full rounded-full bg-rolecue-brand transition-[width] duration-(--duration-slow) motion-reduce:transition-none"
                          />
                        </div>
                      </div>
                      <p className="max-w-64 text-xs leading-relaxed text-muted-foreground">
                        {t("progressMeaning")}
                      </p>
                    </div>
                    <RoadmapGantt
                      key={data.items.map((item) => item.id).join()}
                      roadmap={data}
                    />
                  </>
                ) : (
                  <EmptyState
                    icons={[ChartGantt]}
                    title={t("emptyTitle")}
                    description={t("emptyDescription")}
                  />
                )}
                <div className="flex flex-wrap items-start justify-between gap-5">
                  <div className="max-w-3xl space-y-2">
                    <h2 className="text-sm font-semibold">{t("rulesTitle")}</h2>
                    <p className="text-xs leading-relaxed text-muted-foreground">
                      {t("rules", {
                        count: data.window_size,
                        threshold: data.achieved_threshold,
                      })}
                    </p>
                  </div>
                  {!data.practice_started && !generationBlocked && (
                    <Button
                      variant="ghost"
                      className="h-auto min-h-11 whitespace-normal py-2"
                      onClick={() => setConfirming(true)}
                    >
                      <RotateCcw aria-hidden="true" />
                      {t("regenerate")}
                    </Button>
                  )}
                </div>
              </>
            )
          )}
        </>
      )}
      <Dialog
        open={confirming}
        onOpenChange={(open) => {
          if (!prepare.isPending) setConfirming(open);
        }}
      >
        <DialogContent showCloseButton={!prepare.isPending}>
          <DialogHeader>
            <DialogTitle>{t("regenerateTitle")}</DialogTitle>
            <DialogDescription>{t("regenerateDescription")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              className="min-h-11"
              disabled={prepare.isPending}
              onClick={() => setConfirming(false)}
            >
              {t("cancel")}
            </Button>
            <Button
              className="min-h-11"
              disabled={prepare.isPending}
              onClick={() => prepare.mutate("regenerate")}
            >
              {prepare.isPending && (
                <LoaderCircle
                  className="motion-safe:animate-spin"
                  aria-hidden="true"
                />
              )}
              {t("regenerate")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
