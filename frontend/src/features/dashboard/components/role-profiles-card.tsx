"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { dashboardKeys, listDashboardRoleProfiles } from "../api/dashboard";
import type { RoleProfileSummary } from "../schemas/dashboard-schema";
import { roleProfileStatusLabel } from "../utils/dashboard-text";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";

const PREVIEW_COUNT = 3;

export function RoleProfilesCard() {
  const user = useSessionUser();
  const query = useQuery({
    queryKey: dashboardKeys.roleProfiles(user.id),
    queryFn: ({ signal }) => listDashboardRoleProfiles(PREVIEW_COUNT, signal),
    retry: false,
  });

  return (
    <DashboardCard title="Target JDs">
      {query.isPending ? (
        <CardLoading label="Loading your Target JDs…" />
      ) : query.isError ? (
        <CardError
          message="We couldn't load your Target JDs."
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
  if (total === 0)
    return (
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">
          You haven&apos;t added a Target JD yet. Add one to shape your practice
          interviews.
        </p>
        <Button asChild variant="outline" className="h-11 bg-background">
          <Link href={routes.targetJds}>Add a Target JD</Link>
        </Button>
      </div>
    );

  const pending = items.find((profile) => profile.status !== "confirmed");
  return (
    <div className="space-y-3">
      <p className="text-2xl font-semibold tabular-nums">
        {total} {total === 1 ? "Target JD" : "Target JDs"}
      </p>
      {pending ? (
        <p className="text-sm text-muted-foreground">
          Next step: {roleProfileStatusLabel[pending.status]} for{" "}
          <span className="font-medium text-foreground">{pending.title}</span>.
        </p>
      ) : (
        <p className="text-sm text-muted-foreground">
          Your Target JDs are ready for practice.
        </p>
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
              {roleProfileStatusLabel[profile.status]}
            </span>
          </li>
        ))}
      </ul>
      <Button asChild variant="link" className="h-11 px-0">
        <Link href={routes.targetJds}>Manage Target JDs</Link>
      </Button>
    </div>
  );
}
