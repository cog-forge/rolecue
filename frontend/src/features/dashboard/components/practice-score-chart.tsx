"use client";

import { useId } from "react";
import { useTranslations } from "next-intl";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ReferenceDot,
  XAxis,
  YAxis,
} from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { useFormatters } from "@/lib/i18n/use-formatters";

type ScorePoint = {
  session: string;
  date: string;
  title: string;
  score: number | null;
};

export function PracticeScoreChart({ data }: { data: ScorePoint[] }) {
  const t = useTranslations("candidate.dashboard.progress");
  const fmt = useFormatters();
  const gradient = `practice-${useId().replace(/:/g, "")}`;
  const lastPoint = [...data].reverse().find((item) => item.score !== null);
  return (
    <ChartContainer
      config={{
        score: {
          label: t("score"),
          color: "var(--rolecue-brand-blue)",
        },
      }}
      className="mt-6 h-48 w-full aspect-auto sm:h-52 [&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground"
      aria-label={t("chartLabel")}
    >
      <AreaChart
        accessibilityLayer
        data={data}
        margin={{ left: 0, right: 12, top: 12, bottom: 0 }}
      >
        <defs>
          <linearGradient id={gradient} x1="0" y1="0" x2="0" y2="1">
            <stop
              offset="0%"
              stopColor="var(--color-score)"
              stopOpacity={0.18}
            />
            <stop
              offset="100%"
              stopColor="var(--color-score)"
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
          tick={{ fill: "var(--muted-foreground)" }}
          dataKey="session"
          tickLine={false}
          axisLine={false}
          tickMargin={12}
          minTickGap={32}
          tickFormatter={(value) => data[Number(value) - 1]?.date ?? value}
        />
        <YAxis
          tick={{ fill: "var(--muted-foreground)" }}
          domain={[0, 100]}
          ticks={[0, 25, 50, 75, 100]}
          tickLine={false}
          axisLine={false}
          width={30}
          tickFormatter={(value) => fmt.number(Number(value))}
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(_, payload) => {
                const point = data.find(
                  (item) => item.session === payload[0]?.payload?.session,
                );
                return point ? `${point.date} · ${point.title}` : "";
              }}
            />
          }
        />
        <Area
          type="monotone"
          dataKey="score"
          stroke="var(--color-score)"
          strokeWidth={2.5}
          fill={`url(#${gradient})`}
          connectNulls={false}
          isAnimationActive="auto"
          animationBegin={200}
          animationDuration={1200}
          animationEasing="ease-in-out"
          dot={{ r: 3, strokeWidth: 2, fill: "var(--background)" }}
          activeDot={{ r: 5 }}
        />
        {lastPoint && (
          <ReferenceDot
            x={lastPoint.session}
            y={lastPoint.score ?? 0}
            r={5}
            fill="var(--color-score)"
            stroke="var(--background)"
            strokeWidth={3}
          />
        )}
      </AreaChart>
    </ChartContainer>
  );
}
