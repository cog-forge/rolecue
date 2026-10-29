import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { compareValues, type ComparisonKind } from "../lib/compare";
import { formatCoinAmount, formatVND } from "../lib/format";
import type { RevenueReport } from "../schemas/billing-schema";
import { ComparisonDelta } from "./comparison-delta";
import { SwapValue } from "./swap-value";
import { useFormatters } from "@/lib/i18n/use-formatters";

type Figure = {
  label: string;
  value: string;
  current: number;
  previous: number;
  kind: ComparisonKind;
  note?: string;
};

export function RevenueKpis({
  report,
  comparisonRange,
}: {
  report: RevenueReport;
  comparisonRange: string;
}) {
  const fmt = useFormatters();
  const { current: now, comparison: before } = report;
  const returning = Math.max(0, now.paying_users - now.new_payers);
  const figures: Figure[] = [
    {
      label: "Revenue",
      value: formatVND(now.revenue_vnd),
      current: now.revenue_vnd,
      previous: before.revenue_vnd,
      kind: "money",
      note: `${formatCoinAmount(now.successful_topups)} paid top-ups`,
    },
    {
      label: "Paying users",
      value: formatCoinAmount(now.paying_users),
      current: now.paying_users,
      previous: before.paying_users,
      kind: "count",
      note: `${formatCoinAmount(now.new_payers)} new · ${formatCoinAmount(returning)} returning`,
    },
    {
      label: "Average order",
      value: formatVND(now.average_order_vnd),
      current: now.average_order_vnd,
      previous: before.average_order_vnd,
      kind: "money",
    },
    {
      label: "Payment success",
      value: fmt.percent(now.payment_success_rate),
      current: now.payment_success_rate,
      previous: before.payment_success_rate,
      kind: "rate",
      note: `${formatCoinAmount(now.successful_topups)} of ${formatCoinAmount(now.topup_attempts)} settled PayOS payments`,
    },
  ];

  return (
    <section aria-label="Headline figures">
      <Card className="gap-0 p-0">
        <dl className="grid grid-cols-2 lg:grid-cols-4">
          {figures.map((figure, index) => (
            <div
              key={figure.label}
              role="group"
              aria-label={figure.label}
              className={cn(
                "flex flex-col gap-1 border-border/70 p-5",
                index % 2 === 1 && "border-l",
                index >= 2 && "border-t lg:border-t-0",
                index === 2 && "lg:border-l",
              )}
            >
              <dt className="text-sm text-muted-foreground">{figure.label}</dt>
              <dd
                className={cn(
                  "font-semibold tracking-tight whitespace-nowrap text-foreground tabular-nums",
                  index === 0
                    ? "text-2xl sm:text-[1.875rem] sm:leading-9"
                    : "text-xl sm:text-2xl",
                )}
              >
                <SwapValue value={figure.value} />
              </dd>
              <dd className="text-sm">
                <ComparisonDelta
                  comparison={compareValues(
                    figure.current,
                    figure.previous,
                    figure.kind,
                    fmt.locale,
                  )}
                />
              </dd>
              <dd className="text-xs text-muted-foreground">
                vs {comparisonRange}
              </dd>
              {figure.note && (
                <dd className="mt-1 text-xs text-muted-foreground">
                  {figure.note}
                </dd>
              )}
            </div>
          ))}
        </dl>
      </Card>
    </section>
  );
}
