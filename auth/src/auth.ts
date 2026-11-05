import { betterAuth } from "better-auth";
import { createAuthMiddleware } from "better-auth/api";
import type { Pool } from "pg";
import type { AuthEnv } from "./env";
import { authOptions } from "./options";
import { createAccountNotifications } from "./account-notifications";
import { createAuditLog } from "./audit-log";
import { createMailer } from "./email";
import { enforceInternalAdmin } from "./internal-admin";
import { forceRevokeOtherSessions } from "./password-change-sessions";
import { rateLimitIpAddress, rateLimitOptions } from "./rate-limit";

// Better Auth accepts one hooks.before handler, so every before-check runs here in order.
export const createBeforeHooks = (adminSecret: string) =>
  createAuthMiddleware(async (ctx) => {
    enforceInternalAdmin(ctx, adminSecret);
    return forceRevokeOtherSessions(ctx);
  });

// The schema mapping lives in authOptions; the lifetime is configurable (AUTH_SESSION_MAX_AGE_DAYS).
// The session is refreshed at most once a day, and never less often than it lives.
export const sessionOptions = (maxAgeDays: number) => ({
  ...authOptions.session,
  expiresIn: maxAgeDays * 24 * 60 * 60,
  updateAge: Math.min(authOptions.session.updateAge, maxAgeDays * 24 * 60 * 60),
});

export function createAuth(config: AuthEnv, database: Pool) {
  const { baseURL, appURL, secret, domain, secure, cookiePrefix } = config;
  const mailer = createMailer(config.email);
  const audit = createAuditLog({ secret });
  return betterAuth({
    ...authOptions,
    plugins: [...authOptions.plugins, audit.plugin],
    databaseHooks: audit.databaseHooks,
    hooks: {
      before: createBeforeHooks(config.adminSecret),
      after: createAccountNotifications(mailer.sendAccountStatusEmail),
    },
    session: sessionOptions(config.sessionMaxAgeDays),
    rateLimit: rateLimitOptions,
    // The library logger can print token-bearing URLs; audit events come from audit-log.ts instead.
    logger: { disabled: true },
    baseURL,
    secret,
    database,
    trustedOrigins: [...new Set([appURL, baseURL])],
    advanced: {
      ...authOptions.advanced,
      ipAddress: rateLimitIpAddress,
      cookiePrefix,
      useSecureCookies: secure,
      defaultCookieAttributes: {
        httpOnly: true,
        secure,
        sameSite: "lax",
        path: "/",
      },
      crossSubDomainCookies: domain
        ? { enabled: true, domain }
        : { enabled: false },
    },
    emailAndPassword: {
      enabled: true,
      requireEmailVerification: true,
      minPasswordLength: 8,
      revokeSessionsOnPasswordReset: true,
      sendResetPassword: async ({ user, url }) => {
        await mailer.sendPasswordResetEmail(user, url);
      },
    },
    emailVerification: {
      sendOnSignUp: true,
      sendOnSignIn: false,
      autoSignInAfterVerification: true,
      sendVerificationEmail: async ({ user, url }) => {
        await mailer.sendVerificationEmail(user, url);
      },
    },
    socialProviders: config.socialProviders,
  });
}
