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
import { useTranslations } from "next-intl";
import { ChangePasswordDialog } from "./change-password-dialog";
import type { Formatters } from "@/lib/i18n/formatters";
import { useFormatters } from "@/lib/i18n/use-formatters";

function listSignInMethods(
  providerIds: string[],
  fmt: Formatters,
  fallback: string,
) {
  const names = providerIds
    .filter((id) => id !== "credential")
    .map(providerName);
  return names.length > 0 ? fmt.list(names, "disjunction") : fallback;
}

export function AccountSecurity({ children }: { children?: ReactNode }) {
  const t = useTranslations("auth.security");
  const user = useSessionUser();
  return (
    <section className="mx-auto w-full max-w-5xl">
      <header className="flex flex-wrap items-end justify-between gap-4 pb-8">
        <div className="min-w-0 space-y-1">
          <h1 className="text-[1.75rem] leading-tight font-semibold tracking-tight">
            {t("page.title")}
          </h1>
          <p className="wrap-anywhere text-sm text-muted-foreground">
            {t.rich("page.signedInAs", {
              email: user.email,
              emailTag: (chunks) => (
                <span className="font-medium text-foreground">{chunks}</span>
              ),
            })}
          </p>
        </div>
        <Button variant="outline" asChild className="min-h-11">
          <Link href={routes.profile}>
            <UserRound aria-hidden />
            {t("page.editProfile")}
          </Link>
        </Button>
      </header>
      {children}
    </section>
  );
}

export function PasswordSettings() {
  const t = useTranslations("auth.security");
  const fmt = useFormatters();
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
      title={t("password.title")}
      description={t("password.description")}
    >
      <SecurityPanel>
        {accounts.isPending ? (
          <div role="status" className="py-4">
            <span className="sr-only">{t("password.loading")}</span>
            <Skeleton className="h-11 w-full max-w-sm" />
          </div>
        ) : accounts.isError ? (
          <SecurityRow
            title={t("password.loadFailed")}
            action={
              <Button
                variant="outline"
                className="min-h-11"
                disabled={accounts.isFetching}
                onClick={() => void accounts.refetch()}
              >
                {t("tryAgain")}
              </Button>
            }
          />
        ) : accounts.data.includes("credential") && !noPassword ? (
          <SecurityRow
            icon={<KeyRound />}
            title={t("password.title")}
            status={<Badge variant="positive">{t("status.set")}</Badge>}
            description={t("password.rowDescription")}
            action={
              <>
                <Button
                  variant="outline"
                  className="min-h-11"
                  onClick={() => setDialogOpen(true)}
                >
                  {t("password.change")}
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
            title={t("password.noPassword")}
            description={
              <p role="status">
                {t.rich("password.noPasswordText", {
                  methods: listSignInMethods(
                    accounts.data,
                    fmt,
                    t("password.socialAccount"),
                  ),
                  link: (chunks) => (
                    <Link
                      href={routes.forgotPassword}
                      className="font-medium text-foreground underline-offset-4 hover:underline"
                    >
                      {chunks}
                    </Link>
                  ),
                })}
              </p>
            }
          />
        )}
      </SecurityPanel>
    </SecuritySection>
  );
}
