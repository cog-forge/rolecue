import { authClient } from "@/lib/auth/client";
import { securityData } from "../utils/security-errors";

export const credentialsKeys = {
  all: ["auth", "credentials"] as const,
  root: (userId: string) => [...credentialsKeys.all, userId] as const,
  current: (userId: string) =>
    [...credentialsKeys.root(userId), "current"] as const,
  accounts: (userId: string) =>
    [...credentialsKeys.root(userId), "accounts"] as const,
  sessions: (userId: string) =>
    [...credentialsKeys.root(userId), "sessions"] as const,
};

function assertOwner(actualId: string, expectedId: string) {
  if (actualId !== expectedId)
    throw Object.assign(new Error("Account access changed"), {
      code: "UNAUTHORIZED",
    });
}

export async function getSecuritySession(userId: string, signal?: AbortSignal) {
  const current = securityData(
    await authClient.getSession({
      query: { disableCookieCache: true },
      fetchOptions: { signal },
    }),
  );
  assertOwner(current.user.id, userId);
  return current;
}

export async function getSecurityAccounts(
  userId: string,
  signal?: AbortSignal,
) {
  const accounts = securityData(
    await authClient.listAccounts({ fetchOptions: { signal } }),
  );
  for (const account of accounts) assertOwner(account.userId, userId);
  return accounts;
}

export async function getSecuritySessions(
  userId: string,
  signal?: AbortSignal,
) {
  const sessions = securityData(
    await authClient.listSessions({ fetchOptions: { signal } }),
  );
  for (const session of sessions) assertOwner(session.userId, userId);
  return sessions;
}

export type SecurityAccount = Awaited<
  ReturnType<typeof getSecurityAccounts>
>[number];
export type SecuritySession = Awaited<
  ReturnType<typeof getSecuritySessions>
>[number];
