"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "@/components/ui/empty-state";
import { ChartNoAxesCombined } from "lucide-react";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { formatScore } from "@/features/report/utils/scoring";
import type { PracticeQuery } from "../api/dashboard";
import { recentPractice, hasCompleteScore } from "../utils/practice-summary";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";
import { PracticeScoreChart } from "./practice-score-chart";

export function PracticeProgressCard({
  query,
  className,
}: {
  query: PracticeQuery;
  className?: string;
}) {
  const t = useTranslations("candidate.dashboard.progress");
  const fmt = useFormatters();
  const [window, setWindow] = useState("10");
  const recent = recentPractice(query.data?.items ?? []).slice(
    0,
    Number(window),
  );
  const scored = recent.filter(hasCompleteScore);
  const latest = scored[0];
  const best = scored.length
    ? Math.max(...scored.map((item) => item.score))
    : null;
  const data = [...recent].reverse().map((item, index) => ({
    session: String(index + 1),
    date: fmt.date(item.started_at, "calendarDay"),
    title: item.role_profile_title,
    score: hasCompleteScore(item) ? item.score : null,
  }));
  return (
    <DashboardCard
      isLoading={query.isPending}
      title={t("title")}
      className={className}
      action={
        <Select value={window} onValueChange={setWindow}>
          <SelectTrigger
            aria-label={t("windowLabel")}
            className="h-11 min-w-36 bg-background"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="10">{t("window", { count: 10 })}</SelectItem>
            <SelectItem value="20">{t("window", { count: 20 })}</SelectItem>
          </SelectContent>
        </Select>
      }
    >
      {query.isPending ? (
        <CardLoading label={t("loading")} />
      ) : query.isError ? (
        <CardError
          message={t("error")}
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <>
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <p className="text-sm text-muted-foreground">{t("latest")}</p>
              <p className="mt-2 text-5xl font-semibold tracking-tight tabular-nums sm:text-6xl">
                {latest ? formatScore(latest.score, fmt.number) : "—"}
                <span className="ml-2 text-lg font-normal tracking-normal text-muted-foreground">
                  / 100
                </span>
              </p>
              <p className="mt-2 text-sm text-muted-foreground">
                {latest ? latest.role_profile_title : t("noScore")}
              </p>
            </div>
            <dl className="flex gap-6 text-sm">
              <div>
                <dt className="text-muted-foreground">{t("sessions")}</dt>
                <dd className="mt-1 text-lg font-semibold tabular-nums">
                  {fmt.number(query.data.total)}
                </dd>
              </div>
              <div>
                <dt className="text-muted-foreground">{t("best")}</dt>
                <dd className="mt-1 text-lg font-semibold tabular-nums">
                  {best !== null ? formatScore(best, fmt.number) : "—"}
                </dd>
              </div>
            </dl>
          </div>
          {scored.length ? (
            <>
              <PracticeScoreChart key={window} data={data} />
              <details className="mt-4 border-t pt-3 text-xs text-muted-foreground">
                <summary className="min-h-11 cursor-pointer rounded-md py-3 outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50">
                  {t("scope", { count: recent.length })} · {t("viewValues")}
                </summary>
                <ul className="space-y-2 pb-2">
                  {scored.map((item) => (
                    <li
                      key={item.id}
                      className="flex flex-wrap justify-between gap-2"
                    >
                      <span>
                        {fmt.date(item.started_at, "calendarDay")} ·{" "}
                        {item.role_profile_title}
                      </span>
                      <span className="font-medium tabular-nums text-foreground">
                        {formatScore(item.score, fmt.number)}/100
                      </span>
                    </li>
                  ))}
                </ul>
              </details>
            </>
          ) : (
            <EmptyState
              variant="plain"
              icons={[ChartNoAxesCombined]}
              title={t("emptyTitle")}
              description={t("empty")}
              className="min-h-52 px-0! py-6!"
            />
          )}
          <p className="text-xs leading-relaxed text-muted-foreground">
            {t("explanation")}
          </p>
        </>
      )}
    </DashboardCard>
  );
}
