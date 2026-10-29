"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Smartphone } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "./session-gate";
import {
  SecurityPanel,
  SecurityRow,
  SecuritySection,
} from "./security-section";
import { SecuritySummary } from "./security-summary";
import { TotpEnrollmentDialog } from "./totp-enrollment-dialog";
import { TotpManagement } from "./totp-management";
import { SecuritySessions } from "./security-sessions";
import { SignInMethods } from "./sign-in-methods";
import { SecurityRequestError } from "./security-request-error";
import {
  credentialsKeys,
  getSecurityAccounts,
  getSecuritySession,
  getSecuritySessions,
} from "../api/credentials";
import {
  securityAccessDenied,
  securityErrorMessage,
  securityOutcomeUncertain,
} from "../utils/security-errors";
import type { SocialProvider } from "../utils/social-providers";
import { useTranslations } from "next-intl";

export function AccountCredentials({
  enabledProviders,
  linkingFailed = false,
  password,
}: {
  enabledProviders: readonly SocialProvider[];
  linkingFailed?: boolean;
  // The password section, owned by account-security, sits after the summary.
  password?: ReactNode;
}) {
  const te = useTranslations("errors");
  const user = useSessionUser();
  const cache = useQueryClient();
  const router = useRouter();
  const [accessError, setAccessError] = useState<unknown>(null);
  const [needsRefresh, setNeedsRefresh] = useState(false);
  const current = useQuery({
    queryKey: credentialsKeys.current(user.id),
    queryFn: ({ signal }) => getSecuritySession(user.id, signal),
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: true,
  });
  const accounts = useQuery({
    queryKey: credentialsKeys.accounts(user.id),
    queryFn: ({ signal }) => getSecurityAccounts(user.id, signal),
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: true,
  });
  const sessions = useQuery({
    queryKey: credentialsKeys.sessions(user.id),
    queryFn: ({ signal }) => getSecuritySessions(user.id, signal),
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: true,
  });
  const deniedError = [accessError, current.error, accounts.error].find(
    securityAccessDenied,
  );
  const onAccessError = useCallback((error: unknown) => {
    if (securityAccessDenied(error)) setAccessError(error);
    else if (securityOutcomeUncertain(error)) setNeedsRefresh(true);
  }, []);
  useEffect(() => {
    if (!deniedError) return;
    void cache.cancelQueries({ queryKey: credentialsKeys.root(user.id) });
    void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    router.replace(`${routes.login}?session=ended`);
  }, [cache, deniedError, router, user.id]);
  useEffect(
    () => () => {
      void cache.cancelQueries({ queryKey: credentialsKeys.root(user.id) });
      cache.removeQueries({ queryKey: credentialsKeys.root(user.id) });
    },
    [cache, user.id],
  );
  const refresh = useCallback(async () => {
    await cache.invalidateQueries({ queryKey: credentialsKeys.root(user.id) });
    void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, user.id]);
  const hasPassword =
    accounts.isSuccess &&
    accounts.data.some((account) => account.providerId === "credential");

  if (needsRefresh)
    return (
      <div
        role="alert"
        className="space-y-4 rounded-xl border border-destructive/20 bg-destructive/5 p-4"
      >
        <p className="text-sm">
          We could not confirm the outcome of your security action. Reload to
          check your account before trying again.
        </p>
        <Button className="min-h-11" onClick={() => window.location.reload()}>
          Reload account security
        </Button>
      </div>
    );
  if (deniedError)
    return (
      <p
        role="alert"
        className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm"
      >
        {securityErrorMessage(deniedError, te)}
      </p>
    );
  return (
    <>
      {current.isPending ? (
        <Skeleton
          className="h-[12.25rem] rounded-2xl lg:h-24"
          aria-label="Loading security summary"
        />
      ) : current.error ? (
        <SecurityRequestError
          error={current.error}
          onRetry={() => void current.refetch()}
        />
      ) : (
        <SecuritySummary
          emailVerified={current.data.user.emailVerified}
          hasPassword={accounts.isSuccess ? hasPassword : undefined}
          twoFactorEnabled={!!current.data.user.twoFactorEnabled}
          sessionCount={sessions.data?.length}
        />
      )}
      <div className="mt-8">
        {password}
        <SecuritySection
          id="two-factor-settings"
          title="Two-factor authentication"
          description="A code from your phone, asked for after your password."
        >
          {current.isPending ? (
            <SecurityPanel>
              <p role="status" className="py-4 text-sm text-muted-foreground">
                Loading authenticator status…
              </p>
            </SecurityPanel>
          ) : current.error ? (
            <SecurityRequestError
              error={current.error}
              onRetry={() => void current.refetch()}
            />
          ) : (
            <SecurityPanel>
              <SecurityRow
                icon={<Smartphone />}
                title="Authenticator app"
                status={
                  <Badge
                    variant={
                      current.data.user.twoFactorEnabled
                        ? "positive"
                        : "outline"
                    }
                  >
                    {current.data.user.twoFactorEnabled ? "On" : "Off"}
                  </Badge>
                }
                description={
                  hasPassword
                    ? "Asks for a 6-digit code after your password. Google, GitHub and Facebook sign-in follow that provider’s own security."
                    : "Add a password to your account first. Authenticator codes protect password sign-in."
                }
                action={
                  hasPassword && (
                    <TotpEnrollmentDialog
                      enabled={!!current.data.user.twoFactorEnabled}
                      onChanged={refresh}
                      onAccessError={onAccessError}
                    />
                  )
                }
              />
              {hasPassword && (
                <TotpManagement
                  enabled={!!current.data.user.twoFactorEnabled}
                  onChanged={refresh}
                  onAccessError={onAccessError}
                />
              )}
            </SecurityPanel>
          )}
        </SecuritySection>
        <SecuritySection
          id="sign-in-methods"
          title="Sign-in methods"
          description="Ways to sign in to your RoleCue account."
        >
          {accounts.isPending ? (
            <SecurityPanel>
              <p role="status" className="py-4 text-sm text-muted-foreground">
                Loading sign-in methods…
              </p>
            </SecurityPanel>
          ) : accounts.error ? (
            <SecurityRequestError
              error={accounts.error}
              onRetry={() => void accounts.refetch()}
            />
          ) : (
            <SignInMethods
              accounts={accounts.data}
              enabledProviders={enabledProviders}
              linkingFailed={linkingFailed}
              onChanged={refresh}
              onAccessError={onAccessError}
            />
          )}
        </SecuritySection>
        <SecuritySection
          id="active-sessions"
          title="Active sessions"
          description="Browsers and devices signed in to your account. A browser remembered for two-factor is not a session."
        >
          <SecuritySessions
            sessions={sessions}
            currentId={current.isSuccess ? current.data.session.id : undefined}
            onChanged={refresh}
            onAccessError={onAccessError}
          />
        </SecuritySection>
      </div>
    </>
  );
}
