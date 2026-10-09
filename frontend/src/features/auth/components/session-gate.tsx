"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { useRouter } from "next/navigation";
import { meSchema, type SessionUser } from "../schemas/session-schema";
export type { SessionUser } from "../schemas/session-schema";
import { apiClient } from "@/lib/api/client";
import { routes } from "@/config/routes";
import { Button } from "@/components/ui/button";
import { authClient } from "@/lib/auth/client";
import { VerificationNotice } from "./verification-notice";
import { gooeyToast } from "goey-toast";

const SessionContext = createContext<SessionUser | null>(null);

export function useSessionUser() {
  const user = useContext(SessionContext);
  if (!user) throw new Error("Session user requires a validated SessionGate");
  return user;
}

export function SessionGate({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const cache = useQueryClient();
  const session = useQuery({
    queryKey: ["auth", "me"],
    queryFn: async ({ signal }) =>
      meSchema.parse((await apiClient.get("/auth/me", { signal })).data).data,
    staleTime: 0,
    refetchOnMount: "always",
    refetchOnWindowFocus: true,
    retry: (count, error) =>
      !(
        isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
      ) && count < 1,
  });
  // Revalidate cached data on entry without destroying mounted page state on
  // every background focus check. Any failed recheck still hides the content.
  const [initialSessionUpdates] = useState(
    () => cache.getQueryState(["auth", "me"])?.dataUpdateCount ?? 0,
  );
  const checkingEntry =
    session.isFetching &&
    (cache.getQueryState(["auth", "me"])?.dataUpdateCount ?? 0) <=
      initialSessionUpdates;
  const unauthorized =
    isAxiosError(session.error) && session.error.response?.status === 401;
  const forbidden =
    isAxiosError(session.error) && session.error.response?.status === 403;
  // SDK session data explains a denied request; Go still owns access to content.
  const account = useQuery({
    queryKey: ["auth", "verification"],
    enabled: forbidden,
    queryFn: async () => {
      const result = await authClient.getSession({
        query: { disableCookieCache: true },
      });
      if (result.error) throw new Error("Unable to check email verification");
      return result.data;
    },
    staleTime: 0,
    retry: false,
  });
  const retry = () => {
    void session.refetch();
    if (forbidden) void account.refetch();
  };
  useEffect(() => {
    if (unauthorized) router.replace(routes.login);
  }, [router, unauthorized]);
  useEffect(() => {
    const validated = session.data && !session.error && !session.isFetching;
    const needsVerification =
      forbidden &&
      !account.error &&
      !account.isFetching &&
      account.data &&
      !account.data.user.emailVerified &&
      !account.data.user.banned;
    const url = new URL(window.location.href);
    const event = url.searchParams.get("auth");
    if (event === "signed-in" && (validated || needsVerification)) {
      url.searchParams.delete("auth");
      window.history.replaceState(
        window.history.state,
        "",
        url.pathname + url.search + url.hash,
      );
      gooeyToast.success("Signed in", {
        description: needsVerification
          ? "Verify your email to open your dashboard."
          : "Welcome back to RoleCue.",
        id: "auth-sign-in",
      });
    } else if (event === "email-verified" && validated) {
      url.searchParams.delete("auth");
      window.history.replaceState(
        window.history.state,
        "",
        url.pathname + url.search + url.hash,
      );
      gooeyToast.success("Email verified", {
        description: "Your account is ready. Welcome to RoleCue.",
        id: "auth-email-verified",
      });
    }
  }, [
    session.data,
    session.error,
    session.isFetching,
    forbidden,
    account.data,
    account.error,
    account.isFetching,
  ]);
  if (session.isPending || unauthorized || (!session.error && checkingEntry))
    return <p role="status">Checking your session…</p>;
  if (forbidden && account.isPending)
    return <p role="status">Checking your account…</p>;
  if (
    forbidden &&
    account.data &&
    !account.data.user.emailVerified &&
    !account.data.user.banned
  )
    return (
      <VerificationNotice
        email={account.data.user.email}
        onRetry={retry}
        checking={session.isFetching || account.isFetching}
      />
    );
  if (forbidden && !account.error)
    return <p role="alert">You do not have access to this page.</p>;
  // An old successful result must never render protected content after a failed recheck.
  if (session.error)
    return (
      <div role="alert" className="space-y-4">
        <p>Unable to check your session. Please try again.</p>
        <Button onClick={retry} disabled={session.isFetching}>
          Retry
        </Button>
      </div>
    );
  if (!session.data) return null;
  return (
    <SessionContext.Provider value={session.data}>
      {children}
    </SessionContext.Provider>
  );
}
