"use client";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { authClient } from "@/lib/auth/client";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { ChangePasswordDialog } from "./change-password-dialog";

const providerNames: Record<string, string> = {
  google: "Google",
  github: "GitHub",
  facebook: "Facebook",
};

function listSignInMethods(providerIds: string[]) {
  const names = providerIds
    .filter((id) => id !== "credential")
    .map((id) => providerNames[id] ?? id);
  return names.length > 0
    ? new Intl.ListFormat("en", { type: "disjunction" }).format(names)
    : "a social account";
}

export function AccountSecurity() {
  return (
    <section className="mx-auto w-full max-w-5xl space-y-8">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Your account
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Account security
        </h1>
        <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
          Manage how you sign in to RoleCue.
        </p>
      </header>
      {/* Two-factor authentication (KAN-108) goes in this grid next to the password card. */}
      <div className="grid gap-6 lg:grid-cols-2">
        <PasswordCard />
      </div>
    </section>
  );
}

function PasswordCard() {
  const user = useSessionUser();
  const [dialogOpen, setDialogOpen] = useState(false);
  // Set when change-password reports no password even though the list showed one.
  const [noPassword, setNoPassword] = useState(false);
  const accounts = useQuery({
    queryKey: ["auth", "accounts", user.id],
    queryFn: async () => {
      const result = await authClient.listAccounts();
      if (result.error) throw new Error("Unable to load sign-in methods");
      return result.data.map((account) => account.providerId);
    },
    retry: false,
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2 className="text-lg font-semibold">Password</h2>
        </CardTitle>
        <CardDescription>
          Change the password you use to sign in with your email.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {accounts.isPending ? (
          <div role="status">
            <span className="sr-only">Loading sign-in methods…</span>
            <Skeleton className="h-9 w-40" />
          </div>
        ) : accounts.isError ? (
          <div className="space-y-3">
            <p role="alert" className="text-sm text-destructive">
              We couldn&apos;t load your sign-in methods.
            </p>
            <Button
              variant="outline"
              disabled={accounts.isFetching}
              onClick={() => void accounts.refetch()}
            >
              Try again
            </Button>
          </div>
        ) : accounts.data.includes("credential") && !noPassword ? (
          <>
            <Button onClick={() => setDialogOpen(true)}>Change password</Button>
            <ChangePasswordDialog
              open={dialogOpen}
              onOpenChange={setDialogOpen}
              onNoPassword={() => setNoPassword(true)}
            />
          </>
        ) : (
          // Better Auth's reset flow adds a password to an account that only has a social sign-in.
          <p role="status" className="text-sm leading-relaxed">
            You sign in with {listSignInMethods(accounts.data)}, so this account
            has no password to change. To add one, use{" "}
            <Link
              href={routes.forgotPassword}
              className="font-semibold underline-offset-4 hover:underline"
            >
              Forgot password
            </Link>
            .
          </p>
        )}
      </CardContent>
    </Card>
  );
}
