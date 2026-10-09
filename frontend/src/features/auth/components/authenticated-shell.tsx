"use client";

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
      onSignOut={signOut}
      signingOut={pending}
      signOutError={error}
    >
      {children}
    </ApplicationShell>
  );
}
