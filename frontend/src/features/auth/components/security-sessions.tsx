"use client";

import { useEffect, useState } from "react";
import { useMutation, type UseQueryResult } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { LoaderCircle, Monitor, Smartphone } from "lucide-react";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { authClient } from "@/lib/auth/client";
import type { SecuritySession } from "../api/credentials";
import { useEndLocalSession } from "../hooks/use-sign-out";
import { deviceLabel, isMobileAgent, securityDate } from "../utils/format";
import { securityAccessDenied, securityData } from "../utils/security-errors";
import { SecurityPanel, SecurityRow } from "./security-section";
import { SecurityRequestError } from "./security-request-error";
import { useFormatters } from "@/lib/i18n/use-formatters";

type Selection =
  { kind: "one"; id: string } | { kind: "others" } | { kind: "all" };
export function SecuritySessions({
  sessions,
  currentId,
  onChanged,
  onAccessError,
}: {
  // Owned by AccountCredentials, which also counts sessions for the summary.
  sessions: UseQueryResult<SecuritySession[]>;
  currentId?: string;
  onChanged: () => Promise<void>;
  onAccessError: (error: unknown) => void;
}) {
  const fmt = useFormatters();
  const endLocalSession = useEndLocalSession();
  const [selection, setSelection] = useState<Selection | null>(null);
  useEffect(() => {
    if (securityAccessDenied(sessions.error)) onAccessError(sessions.error);
  }, [sessions.error, onAccessError]);
  const revocation = useMutation({
    gcTime: 0,
    retry: false,
    mutationFn: async () => {
      if (!selection || !currentId)
        throw new Error("Refresh your current session first");
      let logout = false;
      if (selection.kind === "all") {
        securityData(await authClient.revokeSessions());
        logout = true;
        // Revocation already succeeded. A failed cookie cleanup cannot make
        // those sessions active again; always discard local authenticated state.
        try {
          await authClient.signOut();
        } catch {
          /* Local logout still follows. */
        }
      } else if (selection.kind === "others") {
        securityData(await authClient.revokeOtherSessions());
      } else if (selection.id === currentId) {
        securityData(await authClient.signOut());
        logout = true;
      } else {
        const targetId = selection.id;
        const target = sessions.data?.find(
          (session) => session.id === targetId,
        );
        if (!target) throw new Error("Session is no longer available");
        securityData(await authClient.revokeSession({ token: target.token }));
      }
      if (logout) {
        await endLocalSession();
      } else {
        gooeyToast.success(
          selection.kind === "others"
            ? "Your other sessions have been signed out."
            : "Session signed out.",
        );
        await onChanged();
      }
    },
    onSuccess: (): void => {
      setSelection(null);
      revocation.reset();
    },
    onError: onAccessError,
  });
  if (sessions.isPending)
    return (
      <SecurityPanel>
        <p role="status" className="sr-only">
          Loading active sessions…
        </p>
        {[0, 1].map((row) => (
          <div key={row} className="flex items-center gap-3 py-4">
            <Skeleton className="size-5 rounded-full" />
            <div className="flex-1 space-y-2">
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-3.5 w-64 max-w-full" />
            </div>
            <Skeleton className="h-11 w-24" />
          </div>
        ))}
      </SecurityPanel>
    );
  if (sessions.error)
    return (
      <SecurityRequestError
        error={sessions.error}
        onRetry={() => void sessions.refetch()}
      />
    );
  const others = sessions.data.filter((session) => session.id !== currentId);
  // This browser first, then the most recently active.
  const ordered = [...sessions.data].sort(
    (a, b) =>
      Number(b.id === currentId) - Number(a.id === currentId) ||
      new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
  );
  return (
    <>
      {!sessions.data.length ? (
        <SecurityPanel>
          <p className="py-4 text-sm text-muted-foreground">
            No active sessions were returned. Refresh account details before
            taking action.
          </p>
        </SecurityPanel>
      ) : (
        <SecurityPanel>
          <Accordion type="multiple">
            {ordered.map((session) => {
              const current = session.id === currentId;
              const label = deviceLabel(session.userAgent);
              return (
                <AccordionItem
                  key={session.id}
                  value={session.id}
                  className="border-border/70"
                >
                  <SecurityRow
                    icon={
                      isMobileAgent(session.userAgent) ? (
                        <Smartphone />
                      ) : (
                        <Monitor />
                      )
                    }
                    title={label}
                    status={
                      current && <Badge variant="outline">This session</Badge>
                    }
                    description={
                      <p className="wrap-anywhere">
                        {session.ipAddress || "Unknown IP"} · Active{" "}
                        {securityDate(session.updatedAt, fmt)}
                      </p>
                    }
                    action={
                      <Button
                        variant="outline"
                        className="min-h-11"
                        disabled={!currentId || revocation.isPending}
                        aria-label={
                          current
                            ? "Sign out this session"
                            : `Sign out ${label}`
                        }
                        onClick={() => {
                          revocation.reset();
                          setSelection({ kind: "one", id: session.id });
                        }}
                      >
                        Sign out
                      </Button>
                    }
                  >
                    <AccordionTrigger className="mt-1 min-h-11 flex-none items-center gap-1.5 py-0 pl-8 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline">
                      Session details
                    </AccordionTrigger>
                    <AccordionContent>
                      <dl className="grid gap-x-6 gap-y-3 pt-1 pb-1 pl-8 text-xs sm:grid-cols-2">
                        <div>
                          <dt className="text-muted-foreground">Started</dt>
                          <dd className="mt-0.5">
                            {securityDate(session.createdAt, fmt)}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground">Expires</dt>
                          <dd className="mt-0.5">
                            {securityDate(session.expiresAt, fmt)}
                          </dd>
                        </div>
                        <div className="sm:col-span-2">
                          <dt className="text-muted-foreground">
                            Browser identifier
                          </dt>
                          <dd className="mt-0.5 wrap-anywhere">
                            {session.userAgent || "Unavailable"}
                          </dd>
                        </div>
                      </dl>
                    </AccordionContent>
                  </SecurityRow>
                </AccordionItem>
              );
            })}
          </Accordion>
        </SecurityPanel>
      )}
      {sessions.data.length > 0 && (
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            className="min-h-11"
            disabled={!currentId || !others.length || revocation.isPending}
            onClick={() => {
              revocation.reset();
              setSelection({ kind: "others" });
            }}
          >
            Sign out other sessions
          </Button>
          <Button
            variant="ghost"
            className="min-h-11 text-destructive hover:bg-destructive/10 hover:text-destructive"
            disabled={!currentId || revocation.isPending}
            onClick={() => {
              revocation.reset();
              setSelection({ kind: "all" });
            }}
          >
            Sign out all sessions
          </Button>
        </div>
      )}
      <Dialog
        open={selection !== null}
        onOpenChange={(open) => {
          if (!open && !revocation.isPending) {
            setSelection(null);
            revocation.reset();
          }
        }}
      >
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>
              Sign out{" "}
              {selection?.kind === "others"
                ? "other sessions"
                : selection?.kind === "all"
                  ? "all sessions"
                  : "this session"}
              ?
            </DialogTitle>
            <DialogDescription>
              {selection?.kind === "all" ||
              (selection?.kind === "one" && selection.id === currentId)
                ? "You will also be signed out of this browser."
                : "The selected sessions will need to sign in again."}
            </DialogDescription>
          </DialogHeader>
          {Boolean(revocation.error) && (
            <SecurityRequestError error={revocation.error} />
          )}
          <DialogFooter>
            <Button
              variant="outline"
              className="min-h-11"
              disabled={revocation.isPending}
              onClick={() => {
                setSelection(null);
                revocation.reset();
              }}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              className="min-h-11"
              disabled={revocation.isPending}
              aria-busy={revocation.isPending}
              onClick={() => revocation.mutate()}
            >
              {revocation.isPending && (
                <LoaderCircle className="animate-spin" aria-hidden />
              )}
              {revocation.isPending ? "Signing out…" : "Confirm sign out"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
