import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatVND } from "../lib/format";
import type { RevenueMixItem } from "../schemas/billing-schema";
import { ComparisonDelta } from "./comparison-delta";

const roleKeys = ["recruiter", "candidate", "unknown"] as const;
type RoleKey = (typeof roleKeys)[number];

function isRoleKey(role: string): role is RoleKey {
  return roleKeys.some((key) => key === role);
}

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
  const t = useTranslations("admin.billing.revenue");
  return (
    <div className="space-y-2">
      <h3 className="text-xs text-muted-foreground">{title}</h3>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("noTopups")}</p>
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
                    current={item.revenue_vnd}
                    previous={item.comparison_revenue_vnd}
                    kind="money"
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
  const t = useTranslations("admin.billing.revenue.mix");
  return (
    <section aria-label={t("title")}>
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-base font-semibold">
            {t("title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <MixList
            title={t("byRole")}
            items={byRole}
            total={total}
            nameOf={(key) => (isRoleKey(key) ? t(`roles.${key}`) : key)}
          />
          <MixList
            title={t("byPackage")}
            items={byPackage}
            total={total}
            nameOf={(key) => key}
          />
        </CardContent>
      </Card>
    </section>
  );
}
