import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { compareValues } from "../lib/compare";
import { formatCoins, formatVND } from "../lib/format";
import type { RevenueReport } from "../schemas/billing-schema";
import { ComparisonDelta } from "./comparison-delta";

const featureNames: Record<string, string> = {
  practice_interview: "Practice interviews",
  interview_slots: "Interview slots",
  avatar_storage: "Avatar storage",
  other: "Other",
};

export function CoinFlow({ report }: { report: RevenueReport }) {
  const { current: now, comparison: before } = report;
  const flows = [
    { label: "Sold", now: now.coins_sold, before: before.coins_sold },
    { label: "Spent", now: now.coins_spent, before: before.coins_spent },
    {
      label: "Refunded",
      now: now.coins_refunded,
      before: before.coins_refunded,
    },
  ];
  return (
    <section aria-label="Coins" className="lg:col-span-2">
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-base font-semibold">Coins</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <dl className="grid grid-cols-3 gap-3">
            {flows.map((flow) => (
              <div key={flow.label} className="space-y-0.5">
                <dt className="text-xs text-muted-foreground">{flow.label}</dt>
                <dd className="text-lg font-semibold text-foreground tabular-nums">
                  {formatCoins(flow.now)}
                </dd>
                <dd className="text-xs">
                  <ComparisonDelta
                    showAbsolute={false}
                    comparison={compareValues(flow.now, flow.before, "count")}
                  />
                </dd>
              </div>
            ))}
          </dl>
          <div className="space-y-2">
            <h3 className="text-xs text-muted-foreground">Spent on</h3>
            {report.spend_by_feature.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No coins were spent in either period.
              </p>
            ) : (
              <ul className="divide-y divide-border/70">
                {report.spend_by_feature.map((item) => (
                  <li
                    key={item.feature}
                    className="flex items-baseline justify-between gap-3 py-2 text-sm"
                  >
                    <span className="text-foreground">
                      {featureNames[item.feature] ?? item.feature}
                    </span>
                    <span className="flex items-baseline gap-3 tabular-nums">
                      <span className="text-xs text-muted-foreground">
                        {formatCoins(item.comparison_coins)} before
                      </span>
                      <span className="font-medium text-foreground">
                        {formatCoins(item.coins)}
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
  return (
    <section aria-label="Coins users still hold">
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-base font-semibold">
            Coins users still hold
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <p className="text-2xl font-semibold tracking-tight text-foreground tabular-nums">
            {formatCoins(report.outstanding_coins)}
          </p>
          <p className="text-sm text-muted-foreground">
            ≈ {formatVND(report.outstanding_value_vnd)} at the average price
            paid per coin
          </p>
          <p className="text-xs text-muted-foreground">
            Prepaid and not yet used, so not revenue: RoleCue still owes this in
            interviews, slots and storage. Balance right now, not for the
            period.
          </p>
        </CardContent>
      </Card>
    </section>
  );
}
