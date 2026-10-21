"use client";

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
  const { signOut, pending, error } = useSignOut();
  return (
    <ApplicationShell
      user={user}
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
