"use client";

import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { formatVND } from "../lib/format";
import { buildTrendRows, type TrendRow } from "../lib/revenue-chart";
import type { RevenueReport } from "../schemas/billing-schema";
import type { Formatters } from "@/lib/i18n/formatters";
import { useFormatters } from "@/lib/i18n/use-formatters";

const CURRENT = "var(--rolecue-brand-blue)";
const COMPARISON =
  "color-mix(in oklch, var(--rolecue-ink-subtle) 45%, transparent)";

function compactVND(value: number, fmt: Formatters) {
  if (value >= 1_000_000)
    return `${fmt.number(value / 1_000_000, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}M`;
  if (value >= 1_000) return `${Math.round(value / 1_000)}k`;
  return String(value);
}

// This period as solid bars beside the comparison as grey bars on one axis, the reporting
// convention for actuals against the prior year.
export function RevenueTrendChart({
  report,
  range,
  comparisonRange,
}: {
  report: RevenueReport;
  range: string;
  comparisonRange: string;
}) {
  const fmt = useFormatters();
  const rows = buildTrendRows(report);
  const config = {
    comparison: { label: comparisonRange, color: COMPARISON },
    current: { label: range, color: CURRENT },
  } satisfies ChartConfig;
  const step = Math.max(1, Math.ceil(rows.length / 8));
  const isEmpty = rows.every(
    (row) => row.current === 0 && row.comparison === 0,
  );

  return (
    <section aria-label="Revenue over time" className="lg:col-span-2">
      <Card className="h-full">
        <CardHeader className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <CardTitle className="text-base font-semibold">
            Revenue by {report.granularity}
          </CardTitle>
          <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <li className="flex items-center gap-1.5">
              <span
                aria-hidden
                className="size-2 rounded-[2px]"
                style={{ background: CURRENT }}
              />
              {range}
            </li>
            <li className="flex items-center gap-1.5">
              <span
                aria-hidden
                className="size-2 rounded-[2px]"
                style={{ background: COMPARISON }}
              />
              {comparisonRange}
            </li>
          </ul>
        </CardHeader>
        <CardContent>
          {isEmpty && (
            <p className="mb-2 text-sm text-muted-foreground">
              No paid top-ups in either period.
            </p>
          )}
          <ChartContainer
            config={config}
            className="aspect-auto h-60 w-full"
            aria-label={`Revenue by ${report.granularity}, ${range} against ${comparisonRange}`}
          >
            <BarChart data={rows} barGap={2} barCategoryGap="20%">
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="tick"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                interval={step - 1}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                width={40}
                tickFormatter={(value: number) => compactVND(value, fmt)}
                allowDecimals={false}
              />
              <ChartTooltip
                cursor={{ fill: "var(--muted)", opacity: 0.6 }}
                content={
                  <ChartTooltipContent
                    hideLabel
                    formatter={(value, name, item) => {
                      const row = item.payload as TrendRow;
                      return (
                        <div className="flex w-full items-center justify-between gap-4">
                          <span className="flex items-center gap-1.5 text-muted-foreground">
                            <span
                              aria-hidden
                              className="size-2 rounded-[2px]"
                              style={{
                                background:
                                  name === "current" ? CURRENT : COMPARISON,
                              }}
                            />
                            {name === "current"
                              ? row.label
                              : row.comparisonLabel}
                          </span>
                          <span className="font-medium text-foreground tabular-nums">
                            {formatVND(Number(value))}
                          </span>
                        </div>
                      );
                    }}
                  />
                }
              />
              <Bar
                dataKey="comparison"
                fill="var(--color-comparison)"
                radius={[2, 2, 0, 0]}
              />
              <Bar
                dataKey="current"
                fill="var(--color-current)"
                radius={[2, 2, 0, 0]}
              />
            </BarChart>
          </ChartContainer>
        </CardContent>
      </Card>
    </section>
  );
}
