"use client";

import { useTranslations } from "next-intl";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { dashboardKeys, listDashboardRoleProfiles } from "../api/dashboard";
import type { RoleProfileSummary } from "../schemas/dashboard-schema";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";

const PREVIEW_COUNT = 3;

export function RoleProfilesCard() {
  const t = useTranslations("candidate.dashboard.profiles");
  const user = useSessionUser();
  const query = useQuery({
    queryKey: dashboardKeys.roleProfiles(user.id),
    queryFn: ({ signal }) => listDashboardRoleProfiles(PREVIEW_COUNT, signal),
    retry: false,
  });

  return (
    <DashboardCard title={t("title")}>
      {query.isPending ? (
        <CardLoading label={t("loading")} />
      ) : query.isError ? (
        <CardError
          message={t("error")}
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <RoleProfilesBody
          total={query.data.pagination.total}
          items={query.data.items}
        />
      )}
    </DashboardCard>
  );
}

function RoleProfilesBody({
  total,
  items,
}: {
  total: number;
  items: readonly RoleProfileSummary[];
}) {
  const t = useTranslations("candidate.dashboard.profiles");
  if (total === 0)
    return (
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
        <Button asChild variant="outline" className="h-11 bg-background">
          <Link href={routes.targetJds}>{t("add")}</Link>
        </Button>
      </div>
    );

  const pending = items.find((profile) => profile.status !== "confirmed");
  return (
    <div className="space-y-3">
      <p className="text-2xl font-semibold tabular-nums">
        {t("count", { count: total, countText: String(total) })}
      </p>
      {pending ? (
        <p className="text-sm text-muted-foreground">
          {t.rich("next", {
            status: t(`status.${pending.status}`),
            title: pending.title,
            name: (chunks) => (
              <span className="font-medium text-foreground">{chunks}</span>
            ),
          })}
        </p>
      ) : (
        <p className="text-sm text-muted-foreground">{t("ready")}</p>
      )}
      <ul className="divide-y rounded-lg border">
        {items.map((profile) => (
          <li
            key={profile.id}
            className="flex items-center justify-between gap-3 px-3 py-2 text-sm"
          >
            <span className="min-w-0 truncate font-medium">
              {profile.title}
            </span>
            <span className="shrink-0 text-xs text-muted-foreground">
              {t(`status.${profile.status}`)}
            </span>
          </li>
        ))}
      </ul>
      <Button asChild variant="link" className="h-11 px-0">
        <Link href={routes.targetJds}>{t("manage")}</Link>
      </Button>
    </div>
  );
}
