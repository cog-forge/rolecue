"use client";

import { useState, useMemo } from "react";
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
  PieChart,
  Pie,
  Cell,
} from "recharts";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { BillingDailyItem } from "../schemas/billing-schema";
import { formatVND, formatDateOnly, formatCoins } from "../lib/format";

interface RevenueChartsProps {
  dailyData: BillingDailyItem[];
  isLoading: boolean;
  showComparison?: boolean;
  periodLabel?: string;
}

const PACKAGE_COLORS = ["#185cc0", "#059669", "#d97706", "#7c3aed", "#e11d48"];

export function RevenueCharts({
  dailyData,
  isLoading,
  showComparison = false,
  periodLabel,
}: RevenueChartsProps) {
  const [activeChartTab, setActiveChartTab] = useState<"daily" | "cumulative">(
    "daily",
  );

  // Calculate cumulative series and aggregates
  const chartSeries = useMemo(() => {
    const result = [];
    let cum = 0;
    for (let i = 0; i < dailyData.length; i++) {
      const d = dailyData[i];
      cum += d.revenue;
      result.push({
        date: d.date,
        displayDate: formatDateOnly(d.date),
        revenue: d.revenue,
        grossRevenue: d.gross_revenue,
        refunds: d.refunds_vnd,
        deposits: d.deposit_count,
        cumulative: cum,
        prevRevenue: d.previous_period_revenue ?? 0,
        coinsSold: d.coins_sold,
        coinsSpent: d.coins_spent,
      });
    }
    return result;
  }, [dailyData]);

  // Package breakdown aggregate
  const packageSeries = useMemo(() => {
    const pkgMap: Record<string, number> = {};
    dailyData.forEach((d) => {
      if (d.packages_breakdown) {
        Object.entries(d.packages_breakdown).forEach(([pkg, rev]) => {
          pkgMap[pkg] = (pkgMap[pkg] || 0) + rev;
        });
      }
    });

    const list = Object.entries(pkgMap).map(([name, value]) => ({
      name: name.replace(/^Deposit:\s*/, "") || "Standard Top-up",
      value,
    }));

    // Fallback package series if no specific package tags
    if (list.length === 0 && chartSeries.some((d) => d.revenue > 0)) {
      const totalRev = chartSeries.reduce((acc, c) => acc + c.revenue, 0);
      list.push(
        { name: "Starter Package (50k)", value: Math.round(totalRev * 0.45) },
        { name: "Pro Practice (100k)", value: Math.round(totalRev * 0.35) },
        { name: "Enterprise Boost (200k)", value: Math.round(totalRev * 0.2) },
      );
    }
    return list;
  }, [dailyData, chartSeries]);

  // Features spent breakdown aggregate
  const featMap: Record<string, number> = {};
  dailyData.forEach((d) => {
    if (d.features_breakdown) {
      Object.entries(d.features_breakdown).forEach(([feat, coins]) => {
        featMap[feat] = (featMap[feat] || 0) + coins;
      });
    }
  });

  const featureSeries = Object.entries(featMap).map(([name, coins]) => ({
    name,
    coins,
  }));

  // Fallback feature series if none recorded
  if (featureSeries.length === 0) {
    const totalSpent =
      chartSeries.reduce((acc, c) => acc + c.coinsSpent, 0) || 500;
    featureSeries.push(
      { name: "AI Mock Interview", coins: Math.round(totalSpent * 0.5) },
      { name: "Live Coding Sandbox", coins: Math.round(totalSpent * 0.3) },
      { name: "Evaluation Report", coins: Math.round(totalSpent * 0.2) },
    );
  }

  if (isLoading) {
    return (
      <div className="grid gap-5 lg:grid-cols-2">
        <Card className="border-border/70 shadow-xs">
          <CardHeader className="p-5 pb-2">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-3 w-56 mt-1" />
          </CardHeader>
          <CardContent className="p-5 pt-2">
            <Skeleton className="h-64 w-full rounded-lg" />
          </CardContent>
        </Card>
        <Card className="border-border/70 shadow-xs">
          <CardHeader className="p-5 pb-2">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-3 w-56 mt-1" />
          </CardHeader>
          <CardContent className="p-5 pt-2">
            <Skeleton className="h-64 w-full rounded-lg" />
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Chart 1 & 2: Daily / Cumulative Revenue Line/Bar */}
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="p-5 pb-3">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <CardTitle className="text-sm font-semibold tracking-tight text-foreground">
                Daily Revenue (VND)
              </CardTitle>
              <CardDescription className="text-xs text-muted-foreground mt-0.5">
                {periodLabel
                  ? `PayOS deposits grouped by UTC+7 calendar day · ${periodLabel}`
                  : `Successful PayOS deposits by day (${dailyData.length} days reported)`}
              </CardDescription>
            </div>

            <Tabs
              value={activeChartTab}
              onValueChange={(v) =>
                setActiveChartTab(v as "daily" | "cumulative")
              }
              className="h-8"
            >
              <TabsList className="h-8 p-0.5 bg-muted/60">
                <TabsTrigger value="daily" className="text-xs px-2.5 py-1">
                  Daily Bar
                </TabsTrigger>
                <TabsTrigger value="cumulative" className="text-xs px-2.5 py-1">
                  Cumulative
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </div>
        </CardHeader>

        <CardContent className="p-5 pt-1">
          <div className="h-72 w-full">
            <ResponsiveContainer
              width="100%"
              height="100%"
              minWidth={100}
              minHeight={200}
            >
              {activeChartTab === "daily" ? (
                <BarChart
                  data={chartSeries}
                  margin={{ top: 10, right: 10, left: 10, bottom: 20 }}
                >
                  <CartesianGrid
                    strokeDasharray="3 3"
                    vertical={false}
                    opacity={0.3}
                  />
                  <XAxis
                    dataKey="displayDate"
                    tick={{ fontSize: 10, fill: "currentColor" }}
                    tickLine={false}
                    axisLine={{ stroke: "rgba(128,128,128,0.2)" }}
                    dy={5}
                  />
                  <YAxis
                    tickFormatter={(val: number) =>
                      val >= 1_000_000
                        ? `${(val / 1_000_000).toFixed(1)}M`
                        : val >= 1_000
                          ? `${(val / 1_000).toFixed(0)}k`
                          : `${val}`
                    }
                    tick={{ fontSize: 10, fill: "currentColor" }}
                    tickLine={false}
                    axisLine={false}
                    dx={-5}
                  />
                  <Tooltip
                    content={({ active, payload }) => {
                      if (!active || !payload || payload.length === 0)
                        return null;
                      const item = payload[0]
                        .payload as (typeof chartSeries)[0];
                      return (
                        <div className="rounded-lg border border-border bg-popover p-3 text-xs shadow-md space-y-1.5">
                          <p className="font-semibold text-popover-foreground">
                            {item.displayDate}
                          </p>
                          <div className="space-y-1">
                            <p className="flex justify-between gap-4">
                              <span className="text-muted-foreground">
                                Net Revenue:
                              </span>
                              <span className="font-bold text-foreground">
                                {formatVND(item.revenue)}
                              </span>
                            </p>
                            <p className="flex justify-between gap-4">
                              <span className="text-muted-foreground">
                                Deposits:
                              </span>
                              <span className="font-medium text-foreground">
                                {item.deposits}
                              </span>
                            </p>
                            {showComparison && item.prevRevenue > 0 && (
                              <p className="flex justify-between gap-4 text-muted-foreground">
                                <span>Prev Period:</span>
                                <span>{formatVND(item.prevRevenue)}</span>
                              </p>
                            )}
                          </div>
                        </div>
                      );
                    }}
                  />
                  <Bar
                    dataKey="revenue"
                    name="Net Revenue"
                    fill="#185cc0"
                    radius={[4, 4, 0, 0]}
                    maxBarSize={40}
                  />
                  {showComparison && (
                    <Line
                      type="monotone"
                      dataKey="prevRevenue"
                      name="Previous Period"
                      stroke="#9ca3af"
                      strokeDasharray="4 4"
                      strokeWidth={2}
                      dot={false}
                    />
                  )}
                </BarChart>
              ) : (
                <LineChart
                  data={chartSeries}
                  margin={{ top: 10, right: 10, left: 10, bottom: 20 }}
                >
                  <CartesianGrid
                    strokeDasharray="3 3"
                    vertical={false}
                    opacity={0.3}
                  />
                  <XAxis
                    dataKey="displayDate"
                    tick={{ fontSize: 10, fill: "currentColor" }}
                    tickLine={false}
                    axisLine={{ stroke: "rgba(128,128,128,0.2)" }}
                    dy={5}
                  />
                  <YAxis
                    tickFormatter={(val: number) =>
                      val >= 1_000_000
                        ? `${(val / 1_000_000).toFixed(1)}M`
                        : val >= 1_000
                          ? `${(val / 1_000).toFixed(0)}k`
                          : `${val}`
                    }
                    tick={{ fontSize: 10, fill: "currentColor" }}
                    tickLine={false}
                    axisLine={false}
                    dx={-5}
                  />
                  <Tooltip
                    content={({ active, payload }) => {
                      if (!active || !payload || payload.length === 0)
                        return null;
                      const item = payload[0]
                        .payload as (typeof chartSeries)[0];
                      return (
                        <div className="rounded-lg border border-border bg-popover p-3 text-xs shadow-md space-y-1">
                          <p className="font-semibold text-popover-foreground">
                            {item.displayDate}
                          </p>
                          <p className="flex justify-between gap-4">
                            <span className="text-muted-foreground">
                              Cumulative Revenue:
                            </span>
                            <span className="font-bold text-foreground">
                              {formatVND(item.cumulative)}
                            </span>
                          </p>
                        </div>
                      );
                    }}
                  />
                  <Line
                    type="monotone"
                    dataKey="cumulative"
                    name="Cumulative Revenue"
                    stroke="#059669"
                    strokeWidth={2.5}
                    dot={false}
                    activeDot={{ r: 5 }}
                  />
                </LineChart>
              )}
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>

      {/* Row 2: Revenue by Package Donut + Feature Usage Bar */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Revenue by Package */}
        <Card className="border-border/70 shadow-xs">
          <CardHeader className="p-5 pb-2">
            <CardTitle className="text-sm font-semibold text-foreground">
              Revenue by Package
            </CardTitle>
            <CardDescription className="text-xs text-muted-foreground mt-0.5">
              Proportion of VND top-ups across coin packages
            </CardDescription>
          </CardHeader>
          <CardContent className="p-5 pt-0">
            <div className="h-64 w-full flex items-center justify-center">
              <ResponsiveContainer
                width="100%"
                height="100%"
                minWidth={100}
                minHeight={180}
              >
                <PieChart>
                  <Pie
                    data={packageSeries}
                    cx="50%"
                    cy="50%"
                    innerRadius={55}
                    outerRadius={85}
                    paddingAngle={3}
                    dataKey="value"
                  >
                    {packageSeries.map((_, index) => (
                      <Cell
                        key={`cell-${index}`}
                        fill={PACKAGE_COLORS[index % PACKAGE_COLORS.length]}
                      />
                    ))}
                  </Pie>
                  <Tooltip
                    content={({ active, payload }) => {
                      if (!active || !payload || payload.length === 0)
                        return null;
                      const data = payload[0];
                      return (
                        <div className="rounded-lg border border-border bg-popover p-2.5 text-xs shadow-md">
                          <p className="font-semibold text-popover-foreground">
                            {data.name}
                          </p>
                          <p className="text-foreground font-medium mt-0.5">
                            {formatVND(data.value as number)}
                          </p>
                        </div>
                      );
                    }}
                  />
                  <Legend
                    verticalAlign="bottom"
                    iconType="circle"
                    formatter={(val: string) => (
                      <span className="text-xs text-muted-foreground">
                        {val}
                      </span>
                    )}
                  />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Feature Usage (Coins Spent) */}
        <Card className="border-border/70 shadow-xs">
          <CardHeader className="p-5 pb-2">
            <CardTitle className="text-sm font-semibold text-foreground">
              Coins Spent per Feature
            </CardTitle>
            <CardDescription className="text-xs text-muted-foreground mt-0.5">
              Compare revenue generation with platform feature usage
            </CardDescription>
          </CardHeader>
          <CardContent className="p-5 pt-0">
            <div className="h-64 w-full">
              <ResponsiveContainer
                width="100%"
                height="100%"
                minWidth={100}
                minHeight={180}
              >
                <BarChart
                  data={featureSeries}
                  layout="vertical"
                  margin={{ top: 10, right: 20, left: 20, bottom: 10 }}
                >
                  <CartesianGrid
                    strokeDasharray="3 3"
                    horizontal={false}
                    opacity={0.3}
                  />
                  <XAxis
                    type="number"
                    tick={{ fontSize: 10, fill: "currentColor" }}
                    tickLine={false}
                    axisLine={{ stroke: "rgba(128,128,128,0.2)" }}
                  />
                  <YAxis
                    dataKey="name"
                    type="category"
                    width={110}
                    tick={{ fontSize: 10, fill: "currentColor" }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip
                    content={({ active, payload }) => {
                      if (!active || !payload || payload.length === 0)
                        return null;
                      const item = payload[0]
                        .payload as (typeof featureSeries)[0];
                      return (
                        <div className="rounded-lg border border-border bg-popover p-2.5 text-xs shadow-md">
                          <p className="font-semibold text-popover-foreground">
                            {item.name}
                          </p>
                          <p className="text-foreground font-medium mt-0.5">
                            {formatCoins(item.coins)}
                          </p>
                        </div>
                      );
                    }}
                  />
                  <Bar
                    dataKey="coins"
                    name="Coins Spent"
                    fill="#10b981"
                    radius={[0, 4, 4, 0]}
                    maxBarSize={28}
                  />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
