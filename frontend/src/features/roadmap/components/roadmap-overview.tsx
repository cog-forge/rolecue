"use client";
import type { UseQueryResult } from "@tanstack/react-query";
import { AiLoader } from "@/components/ui/ai-loader";
import { useFormatters } from "@/lib/i18n/use-formatters";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useQueries, useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import {
  ArrowRight,
  ArrowUpRight,
  ChartGantt,
  CircleCheck,
  Play,
  Plus,
} from "lucide-react";
import { LoadingMorph } from "@/components/ui/loading-morph";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { roleProfileKeys } from "@/features/role-profile/api/keys";
import {
  listRoleProfiles,
  ROLE_PROFILES_PAGE_SIZE,
} from "@/features/role-profile/api/requests";
import type { RoleSummary } from "@/features/role-profile/api/contract";
import { setupHref } from "@/features/interview-setup/lib/setup-params";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { getRoadmap, roadmapKeys, type Roadmap } from "../api/roadmap";
import { roadmapPollInterval } from "../lib/use-roadmap";
import { nextPracticeTopic } from "../lib/next-practice";
import { TopicScore, TopicStatus } from "./topic-status";
import { RoadmapUpdateNotice } from "./roadmap-screen";

export function RoadmapOverview() {
  const t = useTranslations("candidate.roadmapOverview");
  const common = useTranslations("common.actions");
  const errorMessage = useApiErrorMessage();
  const user = useSessionUser();
  const [offset, setOffset] = useState(0);
  const phases = useRef(new Map<string, { name: string; since: number }>());
  const profiles = useQuery({
    queryKey: roleProfileKeys.list(user.id, offset),
    queryFn: ({ signal }) => listRoleProfiles(offset, signal),
    retry: false,
    refetchOnMount: "always",
  });
  const confirmed =
    profiles.data?.items.filter((profile) => profile.status === "confirmed") ??
    [];
  const roadmaps = useQueries({
    queries: confirmed.map((profile) => ({
      queryKey: roadmapKeys.detail(user.id, profile.id),
      queryFn: ({ signal }: { signal: AbortSignal }) =>
        getRoadmap(profile.id, signal),
      retry: false,
      staleTime: 0,
      refetchOnWindowFocus: true,
      refetchIntervalInBackground: false,
      refetchInterval: (query: {
        state: { data: Roadmap | undefined; error: Error | null };
      }) => {
        if (query.state.error) return false;
        const data = query.state.data;
        const name =
          data?.generation_status === "generating"
            ? "generating"
            : (data?.update_state ?? "");
        let phase = phases.current.get(profile.id);
        if (phase?.name !== name) {
          phase = { name, since: Date.now() };
          phases.current.set(profile.id, phase);
        }
        return roadmapPollInterval(data, Date.now() - phase.since);
      },
    })),
  });
  const [expired, setExpired] = useState<ReadonlyMap<string, number>>(
    () => new Map(),
  );
  const generationClocks = confirmed
    .map((profile, index) =>
      roadmaps[index].data?.generation_status === "generating" &&
      !roadmaps[index].isError
        ? `${profile.id}:${roadmaps[index].dataUpdatedAt}`
        : "",
    )
    .filter(Boolean)
    .join("|");
  useEffect(() => {
    const timers = generationClocks
      .split("|")
      .filter(Boolean)
      .map((clock) => {
        const [id, stamp] = clock.split(":");
        const since = phases.current.get(id)?.since ?? Date.now();
        return setTimeout(
          () =>
            setExpired((previous) => new Map(previous).set(id, Number(stamp))),
          Math.max(0, 300_000 - (Date.now() - since)),
        );
      });
    return () => timers.forEach(clearTimeout);
  }, [generationClocks]);
  const ready = confirmed.flatMap((profile, index) => {
    const roadmap = roadmaps[index].data;
    return roadmap?.generation_status === "ready" && roadmap.items.length
      ? [{ profile, roadmap }]
      : [];
  });
  const recommendation =
    ready.find((entry) => nextPracticeTopic(entry.roadmap)) ?? ready[0];
  const loadingSuggestion =
    !recommendation && roadmaps.some((query) => query.isPending);
  const total = profiles.data?.pagination.total ?? 0;
  return (
    <section className="mx-auto w-full max-w-6xl space-y-9 pb-8">
      <header className="flex flex-wrap items-start justify-between gap-5">
        <div className="max-w-2xl space-y-3">
          <p className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <ChartGantt
              className="size-4 text-rolecue-brand"
              aria-hidden="true"
            />
            {t("eyebrow")}
          </p>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            {t("heading")}
          </h1>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {t("intro")}
          </p>
        </div>
        <Button
          asChild
          variant="outline"
          className="h-auto min-h-11 whitespace-normal py-2"
        >
          <Link href={routes.roleProfiles.new}>
            <Plus aria-hidden="true" />
            {t("createProfile")}
          </Link>
        </Button>
      </header>
      {profiles.isPending ? (
        <div role="status" aria-label={t("loading")} className="space-y-5">
          <Skeleton className="h-64 rounded-xl" />
          {[0, 1, 2].map((n) => (
            <Skeleton key={n} className="h-24 rounded-xl" />
          ))}
        </div>
      ) : profiles.isError ? (
        <ErrorState
          description={errorMessage(profiles.error, t("loadFailed"))}
          onRetry={() => void profiles.refetch()}
          isRetrying={profiles.isFetching}
        />
      ) : !total ? (
        <EmptyState
          icons={[ChartGantt]}
          title={t("emptyTitle")}
          description={t("emptyDescription")}
        >
          <Button asChild className="mt-4 min-h-11">
            <Link href={routes.roleProfiles.new}>{t("createProfile")}</Link>
          </Button>
        </EmptyState>
      ) : (
        <>
          <section aria-labelledby="roadmap-next-title" className="space-y-4">
            <h2
              id="roadmap-next-title"
              className="flex items-center gap-2 text-lg font-semibold"
            >
              <Play className="size-4" aria-hidden="true" />
              {t("continueTitle")}
            </h2>
            {recommendation ? (
              <PracticeSuggestion {...recommendation} />
            ) : loadingSuggestion ? (
              <LoadingMorph label={t("loadingSuggestion")} variant="compact" />
            ) : (
              <EmptyState
                title={t("noReadyTitle")}
                description={t("noReadyDescription")}
              />
            )}
          </section>
          <section
            aria-labelledby="roadmap-library-title"
            className="space-y-4"
          >
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h2 id="roadmap-library-title" className="text-lg font-semibold">
                {t("libraryTitle")}
              </h2>
              <p className="text-xs text-muted-foreground">{t("byProfile")}</p>
            </div>
            {profiles.data?.items.length === 0 && (
              <EmptyState
                title={t("pageEmptyTitle")}
                description={t("pageEmptyDescription")}
              >
                <Button
                  variant="outline"
                  className="mt-4 min-h-11"
                  onClick={() => setOffset(0)}
                >
                  {t("firstPage")}
                </Button>
              </EmptyState>
            )}
            <ul className="divide-y border-y" aria-label={t("libraryTitle")}>
              {profiles.data?.items.map((profile) => {
                const index = confirmed.findIndex(
                  (entry) => entry.id === profile.id,
                );
                const query = index >= 0 ? roadmaps[index] : undefined;
                const stalled =
                  query?.data?.generation_status === "generating" &&
                  expired.get(profile.id) === query.dataUpdatedAt;
                return (
                  <li key={profile.id} className="py-6">
                    <RoadmapRow
                      profile={profile}
                      query={query}
                      stalled={stalled}
                      onRetry={() => {
                        phases.current.delete(profile.id);
                        void query?.refetch();
                      }}
                    />
                  </li>
                );
              })}
            </ul>
            {total > ROLE_PROFILES_PAGE_SIZE &&
              Boolean(profiles.data?.items.length) && (
                <nav
                  aria-label={t("pagination")}
                  className="flex flex-wrap items-center justify-between gap-3"
                >
                  <p role="status" className="text-sm text-muted-foreground">
                    {t("page", {
                      current: Math.floor(offset / ROLE_PROFILES_PAGE_SIZE) + 1,
                      total: Math.ceil(total / ROLE_PROFILES_PAGE_SIZE),
                    })}
                  </p>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      className="min-h-11"
                      disabled={offset === 0}
                      onClick={() =>
                        setOffset(Math.max(0, offset - ROLE_PROFILES_PAGE_SIZE))
                      }
                    >
                      {common("previous")}
                    </Button>
                    <Button
                      variant="outline"
                      className="min-h-11"
                      disabled={offset + ROLE_PROFILES_PAGE_SIZE >= total}
                      onClick={() =>
                        setOffset(offset + ROLE_PROFILES_PAGE_SIZE)
                      }
                    >
                      {common("next")}
                    </Button>
                  </div>
                </nav>
              )}
          </section>
        </>
      )}
    </section>
  );
}

function PracticeSuggestion({
  profile,
  roadmap,
}: {
  profile: RoleSummary;
  roadmap: Roadmap;
}) {
  const t = useTranslations("candidate.roadmapOverview");
  const topicT = useTranslations("candidate.roadmap");
  const topic = nextPracticeTopic(roadmap);
  const generalHref = setupHref(routes.interviews.new.interviewer, {
    roleProfileId: profile.id,
  });
  return (
    <div className="space-y-5 rounded-2xl border bg-card p-5 sm:p-7">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0 space-y-2">
          <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
            {profile.title}
          </p>
          <h3 className="text-xl font-semibold tracking-tight [overflow-wrap:anywhere]">
            {topic?.title ?? t("keepPracticing")}
          </h3>
        </div>
        {topic ? (
          <TopicStatus item={topic} />
        ) : (
          <span className="flex items-center gap-2 text-sm text-emerald-700 dark:text-emerald-400">
            <CircleCheck aria-hidden="true" className="size-4" />
            {t("status.completed")}
          </span>
        )}
      </div>
      {topic ? (
        <div className="space-y-2">
          <TopicScore
            item={topic}
            windowSize={roadmap.window_size}
            threshold={roadmap.achieved_threshold}
          />
          <p className="text-sm text-muted-foreground">
            {topicT("evidence", {
              count: topic.evidence_question_count,
              total: roadmap.window_size,
            })}
          </p>
          <p className="text-sm text-muted-foreground">
            {t(
              topic.score !== null &&
                topic.evidence_question_count < roadmap.window_size
                ? "moreEvidence"
                : "focusHint",
            )}
          </p>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">{t("completedHint")}</p>
      )}
      <div className="flex flex-wrap gap-2">
        {topic && (
          <Button asChild className="h-auto min-h-11 whitespace-normal py-2">
            <Link
              href={setupHref(routes.interviews.new.interviewer, {
                roleProfileId: profile.id,
                focusItemId: topic.id,
              })}
            >
              {topicT("practiceTopic")}
              <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
        )}
        <Button
          asChild
          variant={topic ? "outline" : "default"}
          className="h-auto min-h-11 whitespace-normal py-2"
        >
          <Link href={generalHref}>{topicT("practiceGeneral")}</Link>
        </Button>
        <Button
          asChild
          variant="ghost"
          className="h-auto min-h-11 whitespace-normal py-2"
        >
          <Link href={routes.roleProfiles.roadmap(profile.id)}>
            {t("viewRoadmap")}
            <ArrowUpRight aria-hidden="true" />
          </Link>
        </Button>
      </div>
      <RoadmapUpdateNotice roadmap={roadmap} />
    </div>
  );
}

function RoadmapRow({
  profile,
  query,
  stalled,
  onRetry,
}: {
  profile: RoleSummary;
  query?: UseQueryResult<Roadmap>;
  stalled: boolean;
  onRetry: () => void;
}) {
  const t = useTranslations("candidate.roadmapOverview");
  const errorMessage = useApiErrorMessage();
  const fmt = useFormatters();
  const data = query?.data;
  const ready = data?.generation_status === "ready";
  const state = stalled
    ? "stalled"
    : data?.generation_status !== "ready"
      ? data?.generation_status
      : data.learning_status;
  return (
    <article className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_auto] lg:items-center">
      <div className="min-w-0 space-y-2">
        <h3 className="text-base font-medium [overflow-wrap:anywhere]">
          {profile.title}
        </h3>
        <p className="text-sm text-muted-foreground">
          {profile.status !== "confirmed"
            ? t("needsConfirmation")
            : query?.isError && !data
              ? t("unavailableProgress")
              : state
                ? t(`status.${state}`)
                : t("loading")}
        </p>
      </div>
      <div className="min-w-0">
        {query?.isPending ? (
          <LoadingMorph variant="compact" label={t("loading")} />
        ) : query?.isError && !data ? (
          <ErrorState
            variant="compact"
            description={errorMessage(query.error, t("loadFailed"))}
            onRetry={onRetry}
            isRetrying={query.isFetching}
          />
        ) : ready ? (
          <div className="space-y-2">
            <p className="flex flex-wrap justify-between gap-2 text-sm tabular-nums">
              <span>
                {t("progress", {
                  count: data.completed_items,
                  total: data.total_items,
                })}
              </span>
              <span>
                {data.progress_percent === null
                  ? t("unavailableProgress")
                  : fmt.percent(data.progress_percent, 0)}
              </span>
            </p>
            {data.progress_percent !== null && (
              <div
                role="progressbar"
                aria-label={t("progressLabel", { title: profile.title })}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={data.progress_percent}
                className="h-1.5 overflow-hidden rounded-full bg-muted"
              >
                <div
                  className="h-full rounded-full bg-rolecue-brand transition-[width] duration-(--duration-slow) motion-reduce:transition-none"
                  style={{ width: `${data.progress_percent}%` }}
                />
              </div>
            )}
          </div>
        ) : data?.generation_status === "generating" && !stalled ? (
          <AiLoader
            stage="generating"
            variant="compact"
            title={t("status.generating")}
          />
        ) : null}
        {query?.isError && data && (
          <ErrorState
            variant="compact"
            description={t("refreshFailed")}
            onRetry={onRetry}
            isRetrying={query.isFetching}
          />
        )}
        {data && <RoadmapUpdateNotice roadmap={data} />}
      </div>
      <Button
        asChild
        variant="outline"
        className="h-auto min-h-11 justify-self-start whitespace-normal py-2"
      >
        <Link
          href={
            profile.status === "confirmed"
              ? routes.roleProfiles.roadmap(profile.id)
              : routes.roleProfiles.detail(profile.id)
          }
        >
          {profile.status !== "confirmed"
            ? t("reviewProfile")
            : ready
              ? t("viewRoadmap")
              : t("viewStatus")}
          <ArrowUpRight aria-hidden="true" />
        </Link>
      </Button>
    </article>
  );
}
