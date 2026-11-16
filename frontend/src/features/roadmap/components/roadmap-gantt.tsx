"use client";
import { useState } from "react";
import Link from "next/link";
import {
  ArrowDownRight,
  ArrowRight,
  Check,
  Circle,
  History,
  Target,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { setupHref } from "@/features/interview-setup/lib/setup-params";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { cn } from "@/lib/utils";
import type { Roadmap } from "../api/roadmap";
import { TopicScore, TopicStatus } from "./topic-status";
import { TopicHistory } from "./topic-history";

export function RoadmapGantt({ roadmap }: { roadmap: Roadmap }) {
  const t = useTranslations("candidate.roadmap");
  const fmt = useFormatters();
  const items = [...roadmap.items].sort(
    (a, b) => a.position - b.position || a.id.localeCompare(b.id),
  );
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected =
    items.find((item) => item.id === selectedId) ??
    items.find((item) => item.learning_status !== "achieved") ??
    items[0];
  const [historyId, setHistoryId] = useState<string | null>(null);
  if (!selected) return null;
  return (
    <div className="overflow-hidden rounded-2xl border bg-card">
      <div className="flex flex-wrap items-start justify-between gap-5 border-b p-5 sm:p-6">
        <div>
          <div className="mb-2 flex items-center gap-2 text-xs font-medium text-muted-foreground">
            <Target className="size-4 text-rolecue-brand" aria-hidden="true" />
            {t("mapEyebrow")}
          </div>
          <h2 className="text-xl font-semibold tracking-tight">
            {t("mapTitle")}
          </h2>
          <p className="mt-2 max-w-xl text-sm text-muted-foreground">
            {t("mapDescription")}
          </p>
        </div>
        <span className="flex items-center gap-2 rounded-md border px-3 py-2 text-xs font-medium">
          <span
            className="h-4 border-l-2 border-dashed border-rolecue-brand"
            aria-hidden="true"
          />
          {t("target", { threshold: roadmap.achieved_threshold })}
        </span>
      </div>
      <div className="grid lg:grid-cols-[minmax(0,1fr)_18rem]">
        <div className="min-w-0">
          <div
            className="hidden grid-cols-[minmax(10rem,0.8fr)_minmax(0,1.3fr)] gap-6 border-b bg-muted/25 px-6 py-3 text-xs text-muted-foreground sm:grid"
            aria-hidden="true"
          >
            <span>{t("topic")}</span>
            <div className="flex justify-between tabular-nums">
              {[0, 25, 50, 75, 100].map((n) => (
                <span key={n}>{fmt.number(n)}</span>
              ))}
            </div>
          </div>
          <ol className="divide-y" aria-label={t("mapTitle")}>
            {items.map((item, index) => {
              const active = selected.id === item.id;
              const achieved = item.learning_status === "achieved";
              return (
                <li key={item.id}>
                  <button
                    type="button"
                    aria-pressed={active}
                    aria-label={t("selectTopic", { title: item.title })}
                    onClick={() => setSelectedId(item.id)}
                    className={cn(
                      "group grid w-full min-w-0 gap-4 px-5 py-5 text-left text-foreground outline-none transition-colors duration-(--duration-quick) hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring motion-reduce:transition-none sm:grid-cols-[minmax(10rem,0.8fr)_minmax(0,1.3fr)] sm:gap-6 sm:px-6",
                      active && "bg-rolecue-hero-surface/25",
                    )}
                  >
                    <div className="flex min-w-0 items-start gap-3">
                      <span
                        className={cn(
                          "mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-md border text-[11px] tabular-nums",
                          achieved
                            ? "border-emerald-700/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
                            : active
                              ? "border-rolecue-brand/30 text-rolecue-brand"
                              : "text-muted-foreground",
                        )}
                      >
                        {achieved ? (
                          <Check className="size-3.5" aria-hidden="true" />
                        ) : (
                          fmt.number(index + 1)
                        )}
                      </span>
                      <div className="min-w-0 space-y-2">
                        <h3 className="text-sm leading-snug font-medium [overflow-wrap:anywhere]">
                          {item.title}
                        </h3>
                        <TopicStatus item={item} />
                      </div>
                    </div>
                    <div className="min-w-0 space-y-2">
                      <div
                        className="relative h-8 rounded-md bg-muted/50"
                        aria-hidden="true"
                      >
                        {[25, 50, 75].map((n) => (
                          <span
                            key={n}
                            className="absolute inset-y-0 border-l border-background"
                            style={{ left: `${n}%` }}
                          />
                        ))}
                        {item.score !== null && (
                          <div
                            className={cn(
                              "absolute inset-y-1 left-0 min-w-1 rounded-sm transition-[width] duration-(--duration-slow) ease-(--ease-smooth-out) motion-reduce:transition-none",
                              achieved
                                ? "bg-emerald-600/80 dark:bg-emerald-500/70"
                                : "bg-rolecue-brand/70",
                              item.evidence_question_count <
                                roadmap.window_size && "opacity-60",
                            )}
                            style={{ width: `${item.score}%` }}
                          />
                        )}
                        <span
                          className="absolute -inset-y-1 z-10 border-l-2 border-dashed border-rolecue-brand/70"
                          style={{ left: `${roadmap.achieved_threshold}%` }}
                        />
                      </div>
                      <div className="flex flex-wrap items-center justify-between gap-2 text-xs tabular-nums">
                        <span className="text-muted-foreground">
                          {t("evidence", {
                            count: item.evidence_question_count,
                            total: roadmap.window_size,
                          })}
                        </span>
                        <span className="font-medium">
                          {item.score === null
                            ? t("notEvaluated")
                            : t("score", { score: fmt.number(item.score) })}
                        </span>
                      </div>
                    </div>
                  </button>
                </li>
              );
            })}
          </ol>
          <div className="flex items-start gap-2 border-t px-5 py-4 text-xs leading-relaxed text-muted-foreground sm:px-6">
            <ArrowDownRight className="size-4 shrink-0" aria-hidden="true" />
            {t("unlocked")}
          </div>
        </div>
        <aside
          aria-label={t("topicDetails")}
          className="min-w-0 space-y-5 border-t bg-muted/20 p-5 sm:p-6 lg:border-t-0 lg:border-l"
        >
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Circle className="size-3 text-rolecue-brand" aria-hidden="true" />
            {t("selectedTopic")}
          </div>
          <div
            className="t-panel-slide space-y-3"
            data-open="true"
            key={selected.id}
          >
            <h3 className="text-lg leading-snug font-semibold [overflow-wrap:anywhere]">
              {selected.title}
            </h3>
            <TopicStatus item={selected} />
            {selected.description && (
              <p className="text-sm leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">
                {selected.description}
              </p>
            )}
            <div className="border-y py-4">
              <TopicScore
                item={selected}
                windowSize={roadmap.window_size}
                threshold={roadmap.achieved_threshold}
              />
              <div className="mt-3 flex gap-1" aria-hidden="true">
                {Array.from({ length: roadmap.window_size }, (_, i) => (
                  <span
                    key={i}
                    className={cn(
                      "h-1.5 flex-1 rounded-sm",
                      i < selected.evidence_question_count
                        ? "bg-rolecue-brand/70"
                        : "bg-border",
                    )}
                  />
                ))}
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                {t("evidence", {
                  count: selected.evidence_question_count,
                  total: roadmap.window_size,
                })}
              </p>
            </div>
            <p className="text-xs leading-relaxed text-muted-foreground">
              {selected.last_evaluated_at
                ? t("lastEvaluated", {
                    when: fmt.date(selected.last_evaluated_at, "moment"),
                  })
                : t("noEvaluation")}
            </p>
            <Button
              asChild
              className="h-auto min-h-11 w-full whitespace-normal py-2"
            >
              <Link
                href={setupHref(routes.interviews.new.interviewer, {
                  roleProfileId: roadmap.role_profile_id,
                  focusItemId: selected.id,
                })}
              >
                {t("practiceTopic")}
                <ArrowRight aria-hidden="true" />
              </Link>
            </Button>
            <Button
              variant="ghost"
              className="h-auto min-h-11 w-full whitespace-normal py-2"
              onClick={() => setHistoryId(selected.id)}
            >
              <History aria-hidden="true" />
              {t("viewHistory")}
            </Button>
          </div>
        </aside>
      </div>
      <TopicHistory
        profileId={roadmap.role_profile_id}
        item={items.find((item) => item.id === historyId) ?? null}
        onClose={() => setHistoryId(null)}
      />
    </div>
  );
}
