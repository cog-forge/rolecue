import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { compareValues } from "../lib/compare";
import { formatVND } from "../lib/format";
import type { RevenueMixItem } from "../schemas/billing-schema";
import { ComparisonDelta } from "./comparison-delta";

const roleNames: Record<string, string> = {
  recruiter: "Recruiters",
  candidate: "Candidates",
  unknown: "Unknown payer",
};

function MixList({
  title,
  items,
  total,
  nameOf,
}: {
  title: string;
  items: RevenueMixItem[];
  total: number;
  nameOf: (key: string) => string;
}) {
  return (
    <div className="space-y-2">
      <h3 className="text-xs text-muted-foreground">{title}</h3>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No paid top-ups in either period.
        </p>
      ) : (
        <ul className="space-y-3">
          {items.map((item) => {
            const share = total > 0 ? (item.revenue_vnd / total) * 100 : 0;
            return (
              <li key={item.key} className="space-y-1.5">
                <div className="flex items-baseline justify-between gap-3 text-sm">
                  <span className="truncate text-foreground">
                    {nameOf(item.key)}
                  </span>
                  <span className="shrink-0 font-medium text-foreground tabular-nums">
                    {formatVND(item.revenue_vnd)}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <div className="h-1 flex-1 rounded-full bg-muted">
                    <div
                      className="h-1 rounded-full bg-[color:var(--rolecue-brand-blue)]"
                      style={{ width: `${share}%` }}
                    />
                  </div>
                  <ComparisonDelta
                    className="shrink-0 text-xs"
                    showAbsolute={false}
                    comparison={compareValues(
                      item.revenue_vnd,
                      item.comparison_revenue_vnd,
                      "money",
                    )}
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

export function RevenueMix({
  byRole,
  byPackage,
  total,
}: {
  byRole: RevenueMixItem[];
  byPackage: RevenueMixItem[];
  total: number;
}) {
  return (
    <section aria-label="Where revenue came from">
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-base font-semibold">
            Where revenue came from
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <MixList
            title="By who paid"
            items={byRole}
            total={total}
            nameOf={(key) => roleNames[key] ?? key}
          />
          <MixList
            title="By coin pack"
            items={byPackage}
            total={total}
            nameOf={(key) => key}
          />
        </CardContent>
      </Card>
    </section>
  );
}
