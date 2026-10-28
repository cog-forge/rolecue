import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { formatCoinAmount, formatVND } from "../lib/format";
import type { BillingSummary } from "../schemas/billing-schema";

// The rows behind the current filters in one divided strip: what came in, and what needs a look.
export function BillingSummaryStrip({
  summary,
  isLoading,
}: {
  summary?: BillingSummary;
  isLoading: boolean;
}) {
  const figures = summary
    ? [
        {
          label: "Revenue",
          value: formatVND(summary.total_revenue),
          note: `${formatCoinAmount(summary.successful_deposits)} paid top-ups`,
        },
        {
          label: "Waiting for payment",
          value: formatCoinAmount(summary.pending_count),
          note: "PayOS has not confirmed yet",
        },
        {
          label: "Failed or cancelled",
          value: formatCoinAmount(
            summary.failed_count + summary.cancelled_count,
          ),
          note: `${formatCoinAmount(summary.failed_count)} failed · ${formatCoinAmount(summary.cancelled_count)} cancelled`,
        },
        {
          label: "Coins spent",
          value: formatCoinAmount(summary.coins_spent),
          note: `${formatCoinAmount(summary.coins_refunded)} refunded`,
        },
      ]
    : [];

  return (
    <section aria-label="Matching transactions">
      <Card className="gap-0 p-0">
        <dl className="grid grid-cols-2 lg:grid-cols-4">
          {(isLoading || !summary ? Array.from({ length: 4 }) : figures).map(
            (figure, index) => {
              const item = figure as (typeof figures)[number] | undefined;
              return (
                <div
                  key={item?.label ?? index}
                  role="group"
                  aria-label={item?.label}
                  className={cn(
                    "flex flex-col gap-1 border-border/70 p-4",
                    index % 2 === 1 && "border-l",
                    index >= 2 && "border-t lg:border-t-0",
                    index === 2 && "lg:border-l",
                  )}
                >
                  {item ? (
                    <>
                      <dt className="text-sm text-muted-foreground">
                        {item.label}
                      </dt>
                      <dd className="text-xl font-semibold whitespace-nowrap text-foreground tabular-nums">
                        {item.value}
                      </dd>
                      <dd className="text-xs text-muted-foreground">
                        {item.note}
                      </dd>
                    </>
                  ) : (
                    <>
                      <Skeleton className="h-4 w-24" />
                      <Skeleton className="h-7 w-28" />
                      <Skeleton className="h-3 w-32" />
                    </>
                  )}
                </div>
              );
            },
          )}
        </dl>
      </Card>
    </section>
  );
}
