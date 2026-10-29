"use client";

import { useQuery } from "@tanstack/react-query";
import { routes } from "@/config/routes";
import {
  getPendingCount,
  moderationKeys,
} from "@/features/admin/moderation/api/moderation";
import { useNewTodayCount } from "@/features/job-board/hooks/use-new-today-count";
import { WalletPurchase } from "@/features/billing/components/wallet-purchase";
import { ApplicationShell } from "@/components/layout/application-shell";
import { useSessionUser } from "./session-gate";
import { useSignOut } from "../hooks/use-sign-out";

export function AuthenticatedShell({
  children,
}: {
  children: React.ReactNode;
}) {
  const user = useSessionUser();
  const pendingReviews = useQuery({
    queryKey: moderationKeys.pendingCount(user.id),
    queryFn: ({ signal }) => getPendingCount(signal),
    enabled: user.role === "admin",
    retry: false,
    refetchOnWindowFocus: true,
    // Check for new submissions while admin is active; background polling stays disabled.
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
  });
  const newPostings = useNewTodayCount(user.role === "candidate");
  const { signOut, pending, error } = useSignOut();
  return (
    <ApplicationShell
      user={user}
      navigationBadges={
        user.role === "admin" && pendingReviews.isSuccess
          ? {
              [routes.admin.jobPostings]: {
                count: pendingReviews.data,
                label: "job postings waiting for review",
              },
            }
          : user.role === "candidate" && newPostings.isSuccess
            ? {
                [routes.jobs]: {
                  count: newPostings.data,
                  label: "job postings new today",
                },
              }
            : undefined
      }
      sidebarFooter={
        user.role === "admin"
          ? undefined
          : (collapsed) => (
              <WalletPurchase
                key={user.id}
                userId={user.id}
                role={user.role}
                collapsed={collapsed}
              />
            )
      }
      onSignOut={signOut}
      signingOut={pending}
      signOutError={error}
    >
      {children}
    </ApplicationShell>
  );
}
