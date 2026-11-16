"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowRight, Check, Circle, CircleDashed } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { setupHref } from "@/features/interview-setup/lib/setup-params";
import { cn } from "@/lib/utils";
import type { Roadmap } from "../api/roadmap";
import { nextPracticeTopic } from "../lib/next-practice";
import { TopicScore, TopicStatus } from "./topic-status";

export function RoadmapPreview({
  roadmap,
  open,
}: {
  roadmap: Roadmap;
  open: boolean;
}) {
  const t = useTranslations("candidate.roadmapOverview.preview");
  const topicT = useTranslations("candidate.roadmap");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const items = [...roadmap.items].sort(
    (a, b) => a.position - b.position || a.id.localeCompare(b.id),
  );
  const selected =
    items.find((item) => item.id === selectedId) ??
    nextPracticeTopic(roadmap) ??
    items[0];
  if (!selected) return null;
  return (
    <div className="mt-5 space-y-5 rounded-xl border bg-muted/20 p-4 sm:p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h4 className="text-sm font-medium">{t("heading")}</h4>
        <p className="text-xs text-muted-foreground">{t("hint")}</p>
      </div>
      <div
        className="overflow-x-auto overscroll-x-contain pb-2"
        tabIndex={0}
        role="group"
        aria-label={t("rail")}
      >
        <ol
          className="grid w-full gap-2"
          style={{
            gridTemplateColumns: `repeat(${items.length}, minmax(9rem, 1fr))`,
          }}
        >
          {items.map((item, index) => {
            const achieved = item.learning_status === "achieved";
            const connected =
              achieved && items[index + 1]?.learning_status === "achieved";
            const Icon = achieved
              ? Check
              : item.learning_status === "in_progress"
                ? CircleDashed
                : Circle;
            return (
              <li
                key={item.id}
                className={cn(
                  "relative min-w-0 transition-[opacity,transform] duration-(--duration-fast) ease-(--ease-smooth-out) motion-reduce:transform-none motion-reduce:transition-none",
                  open
                    ? "translate-x-0 opacity-100"
                    : "-translate-x-2 opacity-0",
                )}
                style={{
                  transitionDelay: open
                    ? `${Math.min(index, 7) * 40}ms`
                    : "0ms",
                }}
              >
                {index < items.length - 1 && (
                  <span
                    aria-hidden="true"
                    className={cn(
                      "absolute top-5 left-1/2 h-px w-[calc(100%+0.5rem)]",
                      connected
                        ? "bg-emerald-600/60 dark:bg-emerald-400/60"
                        : "bg-border",
                    )}
                  />
                )}
                <button
                  type="button"
                  onClick={() => setSelectedId(item.id)}
                  aria-label={topicT("selectTopic", { title: item.title })}
                  aria-pressed={selected.id === item.id}
                  className="group/topic relative flex min-h-11 w-full flex-col items-center gap-2 rounded-lg px-2 pb-2 text-center outline-none transition-colors duration-(--duration-quick) hover:bg-muted/70 focus-visible:ring-2 focus-visible:ring-ring motion-reduce:transition-none"
                >
                  <span
                    aria-hidden="true"
                    className="flex h-10 items-center bg-transparent"
                  >
                    <span
                      className={cn(
                        "grid size-6 place-items-center rounded-full border bg-card transition-[background-color,box-shadow] duration-(--duration-fast) motion-reduce:transition-none",
                        achieved
                          ? "border-emerald-600 bg-emerald-600 text-white ring-4 ring-emerald-600/10 dark:border-emerald-400 dark:bg-emerald-400 dark:text-emerald-950"
                          : "border-border text-muted-foreground",
                        selected.id === item.id &&
                          !achieved &&
                          "border-rolecue-brand text-rolecue-brand ring-4 ring-rolecue-brand/10",
                      )}
                    >
                      <Icon className="size-3.5" />
                    </span>
                  </span>
                  <span
                    className={cn(
                      "text-xs leading-relaxed [overflow-wrap:anywhere]",
                      achieved
                        ? "font-medium text-foreground"
                        : "text-muted-foreground",
                      selected.id === item.id && "font-medium text-foreground",
                    )}
                  >
                    {item.title}
                  </span>
                  <span
                    className={cn(
                      "text-[11px]",
                      achieved
                        ? "text-emerald-700 dark:text-emerald-400"
                        : "text-muted-foreground",
                    )}
                  >
                    {topicT(`status.${item.learning_status}`)}
                  </span>
                </button>
              </li>
            );
          })}
        </ol>
      </div>
      <div className="grid gap-4 border-t pt-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <h5 className="text-sm font-medium [overflow-wrap:anywhere]">
              {selected.title}
            </h5>
            <TopicStatus item={selected} />
          </div>
          <p className="line-clamp-2 max-w-3xl text-sm leading-relaxed text-muted-foreground">
            {selected.description || t("noDescription")}
          </p>
          <div className="flex flex-wrap items-start gap-x-5 gap-y-2">
            <TopicScore
              item={selected}
              windowSize={roadmap.window_size}
              threshold={roadmap.achieved_threshold}
            />
            <p className="text-xs leading-5 text-muted-foreground">
              {topicT("evidence", {
                count: selected.evidence_question_count,
                total: roadmap.window_size,
              })}
            </p>
          </div>
        </div>
        <Button
          asChild
          variant="outline"
          className="h-auto min-h-11 justify-self-start whitespace-normal py-2"
        >
          <Link
            href={setupHref(routes.interviews.new.interviewer, {
              roleProfileId: roadmap.role_profile_id,
              focusItemId: selected.id,
            })}
          >
            {topicT("practiceTopic")}
            <ArrowRight aria-hidden="true" />
          </Link>
        </Button>
      </div>
    </div>
  );
}
