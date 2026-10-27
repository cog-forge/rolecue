"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { roleProfileKeys } from "@/features/role-profile/api/keys";
import {
  getRoleProfile,
  roleError,
} from "@/features/role-profile/api/requests";

export function RoleProfileConfirmedGate({ id }: { id?: string }) {
  const user = useSessionUser();
  const query = useQuery({
    queryKey: roleProfileKeys.detail(user.id, id ?? "missing"),
    queryFn: ({ signal }) => getRoleProfile(id ?? "", signal),
    enabled: Boolean(id),
    refetchOnMount: "always",
    staleTime: 0,
    retry: false,
  });
  if (!id)
    return (
      <section className="space-y-4">
        <h1 className="text-3xl font-semibold">
          Choose your Role Profile first.
        </h1>
        <p className="text-sm text-muted-foreground">
          Review and confirm a role before setting up your interview.
        </p>
        <Button asChild className="h-11">
          <Link href={routes.roleProfiles.root}>Open Role Profiles</Link>
        </Button>
      </section>
    );
  if (query.isPending || query.isFetching)
    return <p role="status">Checking your saved Role Profile…</p>;
  if (query.error || !query.data)
    return (
      <div className="space-y-4">
        <p role="alert">{roleError(query.error)}</p>
        <Button variant="outline" onClick={() => void query.refetch()}>
          Try again
        </Button>
      </div>
    );
  if (query.data.status !== "confirmed")
    return (
      <section className="space-y-4">
        <h1 className="text-3xl font-semibold">Confirm this role first.</h1>
        <p className="text-sm text-muted-foreground">
          Review and confirm your requirements and seniority before setup.
        </p>
        <Button asChild className="h-11">
          <Link href={routes.roleProfiles.detail(id)}>
            Return to Role Profile
          </Link>
        </Button>
      </section>
    );
  return (
    <RoutePlaceholder
      title="Interview setup"
      description={`Your Role Profile “${query.data.title}” is confirmed. Session configuration will be available when the interview setup service is connected.`}
    />
  );
}
