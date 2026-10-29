"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
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
  const fmt = useFormatters();
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
      gooeyToast.success(`${providerName(account.providerId)} disconnected`);
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
          Unable to connect that provider. Use a verified account with the same
          email address, or try again.
        </p>
      )}
      {Boolean(connect.error) && <SecurityRequestError error={connect.error} />}
      <SecurityPanel>
        {!accounts.length && (
          <p className="py-4 text-sm text-muted-foreground">
            No connected sign-in methods were returned.
          </p>
        )}
        {accounts.map((account) => (
          <SecurityRow
            key={account.id}
            icon={<ProviderIcon provider={account.providerId} />}
            title={providerName(account.providerId)}
            status={<Badge variant="positive">Connected</Badge>}
            description={`Since ${securityDay(account.createdAt, fmt)}`}
            action={
              account.providerId !== "credential" && (
                <Button
                  variant="outline"
                  className="min-h-11"
                  disabled={busy || accounts.length < 2}
                  aria-label={`Disconnect ${providerName(account.providerId)}`}
                  onClick={() => {
                    disconnect.reset();
                    setDisconnectId(account.id);
                  }}
                >
                  Disconnect
                </Button>
              )
            }
          />
        ))}
        {available.map((provider) => (
          <SecurityRow
            key={provider}
            icon={<ProviderIcon provider={provider} />}
            title={providerName(provider)}
            description="Not connected"
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
                Connect {providerName(provider)}
              </Button>
            }
          />
        ))}
      </SecurityPanel>
      <p className="text-xs leading-5 text-muted-foreground">
        {accounts.length === 1
          ? "Your last sign-in method cannot be disconnected."
          : "Connect a verified provider account that uses the same email address. Keep at least one sign-in method connected."}
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
              Disconnect{" "}
              {disconnected
                ? providerName(disconnected.providerId)
                : "provider"}
              ?
            </DialogTitle>
            <DialogDescription>
              You will no longer be able to sign in with this connected account.
              Your other sign-in methods will remain available.
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
              Cancel
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
              {busy ? "Disconnecting…" : "Confirm disconnect"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
