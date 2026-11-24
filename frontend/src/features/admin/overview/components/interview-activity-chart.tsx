import { useId, useState } from "react";
import { useTranslations } from "next-intl";
import { Activity } from "lucide-react";
import { useReducedMotion } from "motion/react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  XAxis,
  YAxis,
} from "recharts";
import { Card } from "@/components/ui/card";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { cn } from "@/lib/utils";
import type { Overview } from "../schemas/overview-schema";

const modes = ["all", "practice", "recruitment"] as const;
type Mode = (typeof modes)[number];
const dayMs = 86_400_000;

type ChartPoint = {
  start: number;
  end: number;
  previousStart: number;
  previousEnd: number;
  current: number;
  previous: number;
};

export function InterviewActivityChart({ overview }: { overview: Overview }) {
  const t = useTranslations("admin.overview.activity");
  const fmt = useFormatters();
  const reduceMotion = useReducedMotion();
  const [mode, setMode] = useState<Mode>("all");
  const gradientId = useId().replace(/:/g, "");
  const {
    window,
    previous_window: previous,
    interview_activity: activity,
  } = overview;
  const config = {
    current: { label: t("current"), color: "var(--rolecue-brand-blue)" },
    previous: { label: t("previous"), color: "var(--muted-foreground)" },
  } satisfies ChartConfig;

  const rows: ChartPoint[] =
    activity && previous
      ? activity.buckets.map((bucket, index) => {
          const step = activity.bucket_days * dayMs;
          const start = Date.parse(window.from) + index * step;
          const previousStart = Date.parse(previous.from) + index * step;
          return {
            start,
            end: Math.min(Date.parse(window.to), start + step),
            previousStart,
            previousEnd: Math.min(
              Date.parse(previous.to),
              previousStart + step,
            ),
            current:
              (mode === "recruitment" ? 0 : bucket.practice) +
              (mode === "practice" ? 0 : bucket.recruitment),
            previous:
              (mode === "recruitment" ? 0 : bucket.previous_practice) +
              (mode === "practice" ? 0 : bucket.previous_recruitment),
          };
        })
      : [];

  const hasStarts = rows.some(
    (point) => point.current > 0 || point.previous > 0,
  );
  const completed = fmt.number(overview.interviews.completed_in_window);

  return (
    <Card
      role="region"
      aria-labelledby="admin-interview-activity"
      className="min-w-0 gap-0 p-0 xl:col-start-1 xl:col-end-9 xl:row-start-2"
    >
      <div className="flex flex-wrap items-start justify-between gap-4 px-4 pt-5 sm:px-6">
        <div className="space-y-1">
          <h2
            id="admin-interview-activity"
            className="text-lg font-semibold tracking-tight"
          >
            {t("title")}
          </h2>
          <p className="text-sm text-muted-foreground">
            {activity?.bucket_days === 7 ? t("weekly") : t("daily")}
          </p>
        </div>
        {rows.length > 0 && (
          <div
            role="group"
            aria-label={t("mode")}
            className="flex max-w-full flex-wrap gap-0.5 rounded-lg bg-muted p-1"
          >
            {modes.map((value) => (
              <button
                key={value}
                type="button"
                aria-pressed={mode === value}
                onClick={() => setMode(value)}
                className={cn(
                  "min-h-11 rounded-md px-2.5 text-xs font-medium outline-none transition-colors motion-reduce:transition-none focus-visible:ring-2 focus-visible:ring-ring sm:px-3",
                  mode === value
                    ? "bg-background text-foreground shadow-xs"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {t(value)}
              </button>
            ))}
          </div>
        )}
      </div>

      {hasStarts ? (
        <ChartContainer
          config={config}
          className="mx-2 mt-5 h-60 min-w-0 aspect-auto sm:mx-4 sm:h-72"
          aria-label={t("chartLabel", { mode: t(mode) })}
        >
          <ComposedChart
            accessibilityLayer
            data={rows}
            margin={{ top: 10, right: 18, bottom: 4, left: 0 }}
          >
            <defs>
              <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
                <stop
                  offset="0%"
                  stopColor="var(--color-current)"
                  stopOpacity={0.18}
                />
                <stop
                  offset="100%"
                  stopColor="var(--color-current)"
                  stopOpacity={0.015}
                />
              </linearGradient>
            </defs>
            <CartesianGrid
              vertical={false}
              stroke="var(--border)"
              strokeDasharray="3 5"
            />
            <XAxis
              dataKey="start"
              tickFormatter={(value: number) =>
                fmt.date(value, "vietnamShortDate")
              }
              tickLine={false}
              axisLine={false}
              tickMargin={10}
              minTickGap={20}
            />
            <YAxis
              allowDecimals={false}
              domain={[0, "auto"]}
              tickFormatter={(value: number) => fmt.number(value)}
              tickLine={false}
              axisLine={false}
              tickMargin={6}
              width={46}
            />
            <ChartTooltip
              isAnimationActive={!reduceMotion}
              content={
                <ChartTooltipContent
                  hideLabel
                  formatter={(value, _, item) => {
                    const point = item.payload as ChartPoint;
                    const current = item.dataKey === "current";
                    const start = current ? point.start : point.previousStart;
                    const end = current ? point.end : point.previousEnd;
                    return (
                      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
                        <div>
                          <div className="font-medium">
                            {current ? t("current") : t("previous")}
                          </div>
                          <div className="text-muted-foreground">
                            {fmt.date(start, "vietnamDateTime")} –{" "}
                            {fmt.date(end, "vietnamDateTime")}
                          </div>
                        </div>
                        <span className="font-semibold tabular-nums">
                          {fmt.number(Number(value))}
                        </span>
                      </div>
                    );
                  }}
                />
              }
            />
            <Area
              dataKey="current"
              name={t("current")}
              stroke="var(--color-current)"
              strokeWidth={2.5}
              fill={`url(#${gradientId})`}
              isAnimationActive={!reduceMotion}
              animationDuration={420}
              activeDot={{ r: 4 }}
            />
            <Line
              dataKey="previous"
              name={t("previous")}
              stroke="var(--color-previous)"
              strokeWidth={2}
              strokeDasharray="5 5"
              dot={false}
              isAnimationActive={!reduceMotion}
              animationDuration={420}
              activeDot={{ r: 4 }}
            />
          </ComposedChart>
        </ChartContainer>
      ) : (
        <div className="grid h-60 place-content-center justify-items-center gap-3 px-5 text-center text-sm text-muted-foreground sm:h-72">
          <Activity className="size-5" aria-hidden />
          <p>{rows.length ? t("empty") : t("unavailable")}</p>
        </div>
      )}

      <div className="mt-2 flex flex-wrap items-center justify-between gap-3 border-t px-4 py-4 text-xs text-muted-foreground sm:px-6">
        {rows.length > 0 && (
          <ul className="flex flex-wrap gap-x-4 gap-y-1.5">
            <li className="flex items-center gap-1.5">
              <span
                aria-hidden
                className="w-4 border-t-2 border-rolecue-brand"
              />
              {t("current")}
            </li>
            <li className="flex items-center gap-1.5">
              <span
                aria-hidden
                className="w-4 border-t-2 border-dashed border-muted-foreground"
              />
              {t("previous")}
            </li>
          </ul>
        )}
        <p className="font-medium">{t("completed", { count: completed })}</p>
      </div>
    </Card>
  );
}
