"use client";
import { LoadingMorph } from "@/components/ui/loading-morph";
import { ErrorState } from "@/components/ui/error-state";
import { useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { CircleCheck, ArrowUpRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { routes } from "@/config/routes";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { getTopicHistory, roadmapKeys, type RoadmapItem } from "../api/roadmap";
import { readApiError } from "@/lib/i18n/api-errors";

export function TopicHistory({
  profileId,
  item,
  onClose,
}: {
  profileId: string;
  item: RoadmapItem | null;
  onClose: () => void;
}) {
  const t = useTranslations("candidate.roadmap");
  return (
    <Dialog
      open={Boolean(item)}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {t("historyTitle", { title: item?.title ?? "" })}
          </DialogTitle>
          <DialogDescription>{t("historyDescription")}</DialogDescription>
        </DialogHeader>
        {item && (
          <HistoryPage
            key={item.id}
            profileId={profileId}
            itemId={item.id}
            onStale={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
function HistoryPage({
  profileId,
  itemId,
  onStale,
}: {
  profileId: string;
  itemId: string;
  onStale: () => void;
}) {
  const t = useTranslations("candidate.roadmap");
  const user = useSessionUser();
  const cache = useQueryClient();
  const fmt = useFormatters();
  const errorMessage = useApiErrorMessage();
  const [offset, setOffset] = useState(0);
  const query = useQuery({
    queryKey: [...roadmapKeys.history(user.id, profileId, itemId), offset],
    queryFn: ({ signal }) => getTopicHistory(profileId, itemId, offset, signal),
    retry: false,
    staleTime: 0,
    refetchOnMount: "always",
  });
  const stale = readApiError(query.error).code === "ROADMAP_ITEM_NOT_FOUND";
  return (
    <div className="space-y-4">
      {query.isPending ? (
        <LoadingMorph variant="compact" label={t("loading")} />
      ) : query.isError ? (
        <ErrorState
          variant="compact"
          description={errorMessage(query.error, t("loadFailed"))}
          isRetrying={query.isFetching}
          retryLabel={stale ? t("refresh") : t("retry")}
          onRetry={() => {
            if (stale) {
              void cache.invalidateQueries({
                queryKey: roadmapKeys.all(user.id, profileId),
              });
              onStale();
            } else void query.refetch();
          }}
        />
      ) : (
        <>
          {query.data.items.length === 0 ? (
            <p className="py-8 text-center text-muted-foreground">
              {t("historyEmpty")}
            </p>
          ) : (
            <ol className="divide-y">
              {query.data.items.map((row) => (
                <li
                  key={`${row.interview_id}-${row.question_number}`}
                  className="space-y-3 py-4"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                    <time
                      className="text-muted-foreground"
                      dateTime={row.evaluated_at}
                    >
                      {fmt.date(row.evaluated_at, "moment")}
                    </time>
                    <span className="font-semibold tabular-nums">
                      {t("score", { score: fmt.number(row.score) })}
                    </span>
                  </div>
                  <p className="text-sm leading-relaxed [overflow-wrap:anywhere]">
                    {row.question_text ?? t("skipped")}
                  </p>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span
                      className={
                        row.in_current_window
                          ? "flex items-center gap-1.5 text-xs font-medium text-rolecue-brand"
                          : "text-xs text-muted-foreground"
                      }
                    >
                      {row.in_current_window && (
                        <CircleCheck className="size-3.5" aria-hidden="true" />
                      )}
                      {t(row.in_current_window ? "inWindow" : "outsideWindow")}
                    </span>
                    <Link
                      className="inline-flex min-h-11 items-center gap-1 text-xs font-medium underline underline-offset-4"
                      href={routes.reports.detail(row.interview_id)}
                    >
                      {t("openReport")}
                      <ArrowUpRight className="size-3.5" aria-hidden="true" />
                    </Link>
                  </div>
                </li>
              ))}
            </ol>
          )}
          <div className="flex items-center justify-between gap-3 border-t pt-4">
            <Button
              variant="outline"
              className="min-h-11"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - 20))}
            >
              {t("previous")}
            </Button>
            <span className="text-xs tabular-nums">
              {t("historyCount", { count: query.data.pagination.total })}
            </span>
            <Button
              variant="outline"
              className="min-h-11"
              disabled={offset + 20 >= query.data.pagination.total}
              onClick={() => setOffset(offset + 20)}
            >
              {t("next")}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
