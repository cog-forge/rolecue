"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { useTranslations } from "next-intl";
import { KeyRound, LoaderCircle, Mail } from "lucide-react";
import {
  FacebookIcon,
  GitHubIcon,
  GoogleIcon,
} from "@/components/icons/social-icons";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { routes } from "@/config/routes";
import { authClient } from "@/lib/auth/client";
import type { SecurityAccount } from "../api/credentials";
import { securityDay } from "../utils/format";
import { securityData } from "../utils/security-errors";
import { providerName, type SocialProvider } from "../utils/social-providers";
import { SecurityPanel, SecurityRow } from "./security-section";
import { SecurityRequestError } from "./security-request-error";
import { useFormatters } from "@/lib/i18n/use-formatters";

// Provider marks keep their own brand colours; email sign-in uses the neutral key.
function ProviderIcon({ provider }: { provider: string }) {
  switch (provider) {
    case "google":
      return <GoogleIcon />;
    case "github":
      return <GitHubIcon className="text-foreground" />;
    case "facebook":
      return <FacebookIcon className="text-[#1877f2]" />;
    case "credential":
      return <Mail />;
    default:
      return <KeyRound />;
  }
}

export function SignInMethods({
  accounts,
  enabledProviders,
  linkingFailed = false,
  onChanged,
  onAccessError,
}: {
  accounts: readonly SecurityAccount[];
  enabledProviders: readonly SocialProvider[];
  linkingFailed?: boolean;
  onChanged: () => Promise<void>;
  onAccessError: (error: unknown) => void;
}) {
  const t = useTranslations("auth.security");
  const fmt = useFormatters();
  // Provider brands stay as they are; the email sign-in name is the only translated one.
  const nameOf = (provider: string) =>
    provider === "credential"
      ? t("methods.emailAndPassword")
      : providerName(provider);
  const [disconnectId, setDisconnectId] = useState<string | null>(null);
  const disconnect = useMutation({
    gcTime: 0,
    retry: false,
    mutationFn: async () => {
      const account = accounts.find((item) => item.id === disconnectId);
      if (
        !account ||
        account.providerId === "credential" ||
        accounts.length < 2
      )
        throw new Error("Keep a sign-in method connected");
      securityData(await authClient.unlinkAccount({ accountId: account.id }));
      gooeyToast.success(
        t("methods.disconnected", { provider: nameOf(account.providerId) }),
      );
      await onChanged();
    },
    onSuccess: (): void => {
      setDisconnectId(null);
      disconnect.reset();
    },
    onError: onAccessError,
  });
  const connect = useMutation({
    gcTime: 0,
    retry: false,
    mutationFn: async (provider: SocialProvider) => {
      if (!enabledProviders.includes(provider))
        throw new Error("Provider is unavailable");
      securityData(
        await authClient.linkSocial({
          provider,
          callbackURL: `${routes.settings}?linked=success`,
          errorCallbackURL: `${routes.settings}?linked=failed`,
        }),
      );
      // The SDK owns the provider redirect. Connected state comes only from a
      // fresh account-list read after the OAuth callback.
    },
    onError: onAccessError,
  });
  const busy = connect.isPending || disconnect.isPending;
  const disconnected = accounts.find((account) => account.id === disconnectId);
  const available = enabledProviders.filter(
    (provider) => !accounts.some((account) => account.providerId === provider),
  );
  return (
    <>
      {linkingFailed && (
        <p
          role="alert"
          className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm"
        >
          {t("methods.linkingFailed")}
        </p>
      )}
      {Boolean(connect.error) && <SecurityRequestError error={connect.error} />}
      <SecurityPanel>
        {!accounts.length && (
          <p className="py-4 text-sm text-muted-foreground">
            {t("methods.none")}
          </p>
        )}
        {accounts.map((account) => (
          <SecurityRow
            key={account.id}
            icon={<ProviderIcon provider={account.providerId} />}
            title={nameOf(account.providerId)}
            status={<Badge variant="positive">{t("status.connected")}</Badge>}
            description={t("methods.since", {
              date: securityDay(account.createdAt, fmt, t),
            })}
            action={
              account.providerId !== "credential" && (
                <Button
                  variant="outline"
                  className="min-h-11"
                  disabled={busy || accounts.length < 2}
                  aria-label={t("methods.disconnectProvider", {
                    provider: nameOf(account.providerId),
                  })}
                  onClick={() => {
                    disconnect.reset();
                    setDisconnectId(account.id);
                  }}
                >
                  {t("methods.disconnect")}
                </Button>
              )
            }
          />
        ))}
        {available.map((provider) => (
          <SecurityRow
            key={provider}
            icon={<ProviderIcon provider={provider} />}
            title={nameOf(provider)}
            description={t("methods.notConnected")}
            action={
              <Button
                variant="outline"
                className="min-h-11"
                disabled={busy}
                aria-busy={connect.isPending && connect.variables === provider}
                onClick={() => {
                  connect.reset();
                  connect.mutate(provider);
                }}
              >
                {connect.isPending && connect.variables === provider && (
                  <LoaderCircle className="animate-spin" aria-hidden />
                )}
                {t("methods.connect", { provider: nameOf(provider) })}
              </Button>
            }
          />
        ))}
      </SecurityPanel>
      <p className="text-xs leading-5 text-muted-foreground">
        {accounts.length === 1
          ? t("methods.lastMethod")
          : t("methods.connectHint")}
      </p>
      <Dialog
        open={disconnectId !== null}
        onOpenChange={(open) => {
          if (!open && !busy) {
            setDisconnectId(null);
            disconnect.reset();
          }
        }}
      >
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>
              {disconnected
                ? t("methods.dialogTitle", {
                    provider: nameOf(disconnected.providerId),
                  })
                : t("methods.dialogTitleFallback")}
            </DialogTitle>
            <DialogDescription>
              {t("methods.dialogDescription")}
            </DialogDescription>
          </DialogHeader>
          {Boolean(disconnect.error) && (
            <SecurityRequestError error={disconnect.error} />
          )}
          <DialogFooter>
            <Button
              variant="outline"
              className="min-h-11"
              disabled={busy}
              onClick={() => {
                setDisconnectId(null);
                disconnect.reset();
              }}
            >
              {t("cancel")}
            </Button>
            <Button
              variant="destructive"
              className="min-h-11"
              disabled={busy}
              aria-busy={disconnect.isPending}
              onClick={() => disconnect.mutate()}
            >
              {disconnect.isPending && (
                <LoaderCircle className="animate-spin" aria-hidden />
              )}
              {busy ? t("methods.disconnecting") : t("methods.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
