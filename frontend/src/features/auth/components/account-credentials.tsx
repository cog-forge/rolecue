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
  const t = useTranslations("auth.security");
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
        <p className="text-sm">{t("credentials.reloadNotice")}</p>
        <Button className="min-h-11" onClick={() => window.location.reload()}>
          {t("credentials.reload")}
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
          aria-label={t("summary.loading")}
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
          title={t("credentials.twoFactorTitle")}
          description={t("credentials.twoFactorDescription")}
        >
          {current.isPending ? (
            <SecurityPanel>
              <p role="status" className="py-4 text-sm text-muted-foreground">
                {t("credentials.loadingAuthenticator")}
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
                title={t("credentials.authenticatorApp")}
                status={
                  <Badge
                    variant={
                      current.data.user.twoFactorEnabled
                        ? "positive"
                        : "outline"
                    }
                  >
                    {current.data.user.twoFactorEnabled
                      ? t("status.on")
                      : t("status.off")}
                  </Badge>
                }
                description={
                  hasPassword
                    ? t("credentials.authenticatorWithPassword")
                    : t("credentials.authenticatorNoPassword")
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
          title={t("credentials.methodsTitle")}
          description={t("credentials.methodsDescription")}
        >
          {accounts.isPending ? (
            <SecurityPanel>
              <p role="status" className="py-4 text-sm text-muted-foreground">
                {t("credentials.loadingMethods")}
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
          title={t("credentials.sessionsTitle")}
          description={t("credentials.sessionsDescription")}
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
