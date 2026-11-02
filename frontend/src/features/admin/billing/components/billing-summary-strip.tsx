import { useTranslations } from "next-intl";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { formatCoinAmount, formatVND } from "../lib/format";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { BillingSummary } from "../schemas/billing-schema";

// The rows behind the current filters in one divided strip: what came in, and what needs a look.
export function BillingSummaryStrip({
  summary,
  isLoading,
}: {
  summary?: BillingSummary;
  isLoading: boolean;
}) {
  const t = useTranslations("admin.billing.transactions.summary");
  const { locale } = useFormatters();
  const amount = (value: number) => formatCoinAmount(value, locale);
  const figures = summary
    ? [
        {
          label: t("revenue"),
          value: formatVND(summary.total_revenue),
          note: t("revenueNote", {
            count: amount(summary.successful_deposits),
          }),
        },
        {
          label: t("waiting"),
          value: amount(summary.pending_count),
          note: t("waitingNote"),
        },
        {
          label: t("failedOrCancelled"),
          value: amount(summary.failed_count + summary.cancelled_count),
          note: t("failedOrCancelledNote", {
            failed: amount(summary.failed_count),
            cancelled: amount(summary.cancelled_count),
          }),
        },
        {
          label: t("coinsSpent"),
          value: amount(summary.coins_spent),
          note: t("coinsSpentNote", {
            refunded: amount(summary.coins_refunded),
          }),
        },
      ]
    : [];

  return (
    <section aria-label={t("label")}>
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
