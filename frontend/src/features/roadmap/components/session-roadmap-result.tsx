"use client";
import { LoadingMorph } from "@/components/ui/loading-morph";
import { ErrorState } from "@/components/ui/error-state";
import Link from "next/link";
import { useQueries, useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getInterviewSession } from "@/features/interview-setup/api/start-session";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { getSessionTopicEvidence, roadmapKeys } from "../api/roadmap";
import { useRoadmap } from "../lib/use-roadmap";
import { RoadmapUpdateNotice } from "./roadmap-screen";
import { TopicStatus, TopicScore } from "./topic-status";

export function SessionRoadmapResult({ sessionId }: { sessionId: string }) {
  const user = useSessionUser();
  const t = useTranslations("candidate.roadmap");
  const errorMessage = useApiErrorMessage();
  const session = useQuery({
    queryKey: ["roadmap-session", user.id, sessionId],
    queryFn: ({ signal }) => getInterviewSession(sessionId, signal),
    retry: false,
  });
  if (session.isPending)
    return <LoadingMorph variant="compact" label={t("loadingResult")} />;
  if (session.isError)
    return (
      <ErrorState
        variant="compact"
        description={errorMessage(session.error, t("loadFailed"))}
        onRetry={() => void session.refetch()}
        isRetrying={session.isFetching}
        retryLabel={t("retry")}
      />
    );
  if (session.data.type !== "practice" || !session.data.role_profile_id)
    return null;
  return (
    <TopicResults
      profileId={session.data.role_profile_id}
      sessionId={sessionId}
    />
  );
}
function TopicResults({
  profileId,
  sessionId,
}: {
  profileId: string;
  sessionId: string;
}) {
  const t = useTranslations("candidate.roadmap");
  const user = useSessionUser();
  const errorMessage = useApiErrorMessage();
  const roadmap = useRoadmap(user.id, profileId);
  const data = roadmap.data;
  const topics = data?.generation_status === "ready" ? data.items : [];
  const histories = useQueries({
    queries: topics.map((item) => ({
      queryKey: [
        ...roadmapKeys.history(user.id, profileId, item.id),
        "session",
        sessionId,
        roadmap.dataUpdatedAt,
      ],
      queryFn: ({ signal }: { signal: AbortSignal }) =>
        getSessionTopicEvidence(profileId, item.id, sessionId, signal),
      retry: false,
      staleTime: 0,
    })),
  });
  const loading = histories.some((query) => query.isPending);
  const failed = histories.some((query) => query.isError);
  const touched = topics.filter(
    (_, index) => (histories[index].data?.length ?? 0) > 0,
  );
  return (
    <section
      className="space-y-4 rounded-xl border bg-card p-5 sm:p-6"
      aria-label={t("resultTitle")}
    >
      <h2 className="text-lg font-semibold">{t("resultTitle")}</h2>
      {roadmap.isPending ? (
        <LoadingMorph variant="compact" label={t("loadingResult")} />
      ) : roadmap.isError ? (
        <ErrorState
          variant="compact"
          description={errorMessage(roadmap.error, t("loadFailed"))}
          onRetry={() => void roadmap.refetch()}
          isRetrying={roadmap.isFetching}
          retryLabel={t("retry")}
        />
      ) : (
        data && (
          <>
            <RoadmapUpdateNotice roadmap={data} />
            {data.generation_status !== "ready" ? (
              <p className="text-sm text-muted-foreground">
                {t("resultUnavailable")}
              </p>
            ) : loading ? (
              <LoadingMorph variant="compact" label={t("loadingResult")} />
            ) : failed ? (
              <ErrorState
                variant="compact"
                description={t("resultFailed")}
                retryLabel={t("retry")}
                isRetrying={histories.some((query) => query.isFetching)}
                onRetry={() => {
                  histories
                    .filter((query) => query.isError)
                    .forEach((query) => {
                      void query.refetch();
                    });
                }}
              />
            ) : (
              <>
                <p className="text-sm text-muted-foreground">
                  {t(
                    touched.length ? "resultDescription" : "noSessionEvidence",
                  )}
                </p>
                <ul className="divide-y">
                  {touched.map((item) => (
                    <li
                      key={item.id}
                      className="grid gap-3 py-4 sm:grid-cols-[1fr_auto]"
                    >
                      <div className="space-y-2">
                        <h3 className="font-medium [overflow-wrap:anywhere]">
                          {item.title}
                        </h3>
                        <TopicStatus item={item} />
                        <p className="text-xs text-muted-foreground">
                          {t("evidence", {
                            count: item.evidence_question_count,
                            total: data.window_size,
                          })}
                        </p>
                      </div>
                      <TopicScore
                        item={item}
                        windowSize={data.window_size}
                        threshold={data.achieved_threshold}
                      />
                    </li>
                  ))}
                </ul>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {t("rules", {
                    count: data.window_size,
                    threshold: data.achieved_threshold,
                  })}
                </p>
              </>
            )}
          </>
        )
      )}
      <Button
        asChild
        variant="outline"
        className="h-auto min-h-11 whitespace-normal py-2"
      >
        <Link href={routes.roleProfiles.roadmap(profileId)}>
          {t("openRoadmap")}
        </Link>
      </Button>
    </section>
  );
}
