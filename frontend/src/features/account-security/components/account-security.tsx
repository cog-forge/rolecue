"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import {
  credentialsKeys,
  getSecurityAccounts,
} from "@/features/auth/api/credentials";
import {
  SecurityPanel,
  SecurityRow,
  SecuritySection,
} from "@/features/auth/components/security-section";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { providerName } from "@/features/auth/utils/social-providers";
import { useQuery } from "@tanstack/react-query";
import { KeyRound, UserRound } from "lucide-react";
import Link from "next/link";
import { useState, type ReactNode } from "react";
import { ChangePasswordDialog } from "./change-password-dialog";

function listSignInMethods(providerIds: string[]) {
  const names = providerIds
    .filter((id) => id !== "credential")
    .map(providerName);
  return names.length > 0
    ? new Intl.ListFormat("en", { type: "disjunction" }).format(names)
    : "a social account";
}

export function AccountSecurity({ children }: { children?: ReactNode }) {
  const user = useSessionUser();
  return (
    <section className="mx-auto w-full max-w-5xl">
      <header className="flex flex-wrap items-end justify-between gap-4 pb-8">
        <div className="min-w-0 space-y-1">
          <h1 className="text-[1.75rem] leading-tight font-semibold tracking-tight">
            Account security
          </h1>
          <p className="wrap-anywhere text-sm text-muted-foreground">
            How you sign in as{" "}
            <span className="font-medium text-foreground">{user.email}</span>
          </p>
        </div>
        <Button variant="outline" asChild className="min-h-11">
          <Link href={routes.profile}>
            <UserRound aria-hidden />
            Edit profile
          </Link>
        </Button>
      </header>
      {children}
    </section>
  );
}

export function PasswordSettings() {
  const user = useSessionUser();
  const [dialogOpen, setDialogOpen] = useState(false);
  // Set when change-password reports no password even though the list showed one.
  const [noPassword, setNoPassword] = useState(false);
  // Shares the cache entry with the sign-in methods list on the same screen.
  const accounts = useQuery({
    queryKey: credentialsKeys.accounts(user.id),
    queryFn: ({ signal }) => getSecurityAccounts(user.id, signal),
    select: (data) => data.map((account) => account.providerId),
    retry: false,
  });

  return (
    <SecuritySection
      id="password-settings"
      title="Password"
      description="Change the password you use to sign in with your email."
    >
      <SecurityPanel>
        {accounts.isPending ? (
          <div role="status" className="py-4">
            <span className="sr-only">Loading sign-in methods…</span>
            <Skeleton className="h-11 w-full max-w-sm" />
          </div>
        ) : accounts.isError ? (
          <SecurityRow
            title="We couldn't load your sign-in methods."
            action={
              <Button
                variant="outline"
                className="min-h-11"
                disabled={accounts.isFetching}
                onClick={() => void accounts.refetch()}
              >
                Try again
              </Button>
            }
          />
        ) : accounts.data.includes("credential") && !noPassword ? (
          <SecurityRow
            icon={<KeyRound />}
            title="Password"
            status={<Badge variant="positive">Set</Badge>}
            description="At least 8 characters. Changing it can also sign out your other devices."
            action={
              <>
                <Button
                  variant="outline"
                  className="min-h-11"
                  onClick={() => setDialogOpen(true)}
                >
                  Change password
                </Button>
                <ChangePasswordDialog
                  open={dialogOpen}
                  onOpenChange={setDialogOpen}
                  onNoPassword={() => setNoPassword(true)}
                />
              </>
            }
          />
        ) : (
          // Better Auth's reset flow adds a password to an account that only has a social sign-in.
          <SecurityRow
            icon={<KeyRound />}
            title="No password"
            description={
              <p role="status">
                You sign in with {listSignInMethods(accounts.data)}, so this
                account has no password to change. To add one, use{" "}
                <Link
                  href={routes.forgotPassword}
                  className="font-medium text-foreground underline-offset-4 hover:underline"
                >
                  Forgot password
                </Link>
                .
              </p>
            }
          />
        )}
      </SecurityPanel>
    </SecuritySection>
  );
}
