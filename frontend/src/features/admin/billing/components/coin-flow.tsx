import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { formatCoins, formatVND } from "../lib/format";
import type { RevenueReport } from "../schemas/billing-schema";
import { ComparisonDelta } from "./comparison-delta";

const featureKeys = [
  "practice_interview",
  "interview_slots",
  "avatar_storage",
  "other",
] as const;
type FeatureKey = (typeof featureKeys)[number];

function isFeatureKey(feature: string): feature is FeatureKey {
  return featureKeys.some((key) => key === feature);
}

export function CoinFlow({ report }: { report: RevenueReport }) {
  const t = useTranslations("admin.billing.revenue.coinFlow");
  const fmt = useFormatters();
  const { current: now, comparison: before } = report;
  const flows = [
    { label: t("sold"), now: now.coins_sold, before: before.coins_sold },
    { label: t("spent"), now: now.coins_spent, before: before.coins_spent },
    {
      label: t("refunded"),
      now: now.coins_refunded,
      before: before.coins_refunded,
    },
  ];
  return (
    <section aria-label={t("title")} className="lg:col-span-2">
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-base font-semibold">
            {t("title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <dl className="grid grid-cols-3 gap-3">
            {flows.map((flow) => (
              <div key={flow.label} className="min-w-0 space-y-0.5">
                <dt className="text-xs text-muted-foreground">{flow.label}</dt>
                <dd className="text-lg font-semibold text-foreground tabular-nums">
                  {formatCoins(flow.now, fmt)}
                </dd>
                <dd className="text-xs">
                  <ComparisonDelta
                    showAbsolute={false}
                    current={flow.now}
                    previous={flow.before}
                    kind="count"
                  />
                </dd>
              </div>
            ))}
          </dl>
          <div className="space-y-2">
            <h3 className="text-xs text-muted-foreground">{t("spentOn")}</h3>
            {report.spend_by_feature.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("noneSpent")}</p>
            ) : (
              <ul className="divide-y divide-border/70">
                {report.spend_by_feature.map((item) => (
                  <li
                    key={item.feature}
                    className="flex items-baseline justify-between gap-3 py-2 text-sm"
                  >
                    <span className="min-w-0 wrap-break-word text-foreground">
                      {isFeatureKey(item.feature)
                        ? t(`features.${item.feature}`)
                        : item.feature}
                    </span>
                    <span className="flex items-baseline gap-3 tabular-nums">
                      <span className="text-xs text-muted-foreground">
                        {t("before", {
                          coins: formatCoins(item.comparison_coins, fmt),
                        })}
                      </span>
                      <span className="font-medium text-foreground">
                        {formatCoins(item.coins, fmt)}
                      </span>
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </CardContent>
      </Card>
    </section>
  );
}

export function OutstandingCoins({ report }: { report: RevenueReport }) {
  const t = useTranslations("admin.billing.revenue.outstanding");
  const fmt = useFormatters();
  return (
    <section aria-label={t("title")}>
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-base font-semibold">
            {t("title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <p className="text-2xl font-semibold tracking-tight text-foreground tabular-nums">
            {formatCoins(report.outstanding_coins, fmt)}
          </p>
          <p className="text-sm text-muted-foreground">
            {t("value", { amount: formatVND(report.outstanding_value_vnd) })}
          </p>
          <p className="text-xs text-muted-foreground">{t("note")}</p>
        </CardContent>
      </Card>
    </section>
  );
}
