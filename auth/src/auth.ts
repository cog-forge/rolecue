import { betterAuth } from "better-auth";
import type { Pool } from "pg";
import type { AuthEnv } from "./env";
import { authOptions } from "./options";
import { createAccountNotifications } from "./account-notifications";
import { createMailer } from "./email";
import { createInternalAdmin } from "./internal-admin";

export function createAuth(config: AuthEnv, database: Pool) {
  const { baseURL, appURL, secret, domain, secure, cookiePrefix } = config;
  const mailer = createMailer(config.email);
  return betterAuth({
    ...authOptions,
    hooks: {
      before: createInternalAdmin(config.adminSecret),
      after: createAccountNotifications(mailer.sendAccountStatusEmail),
    },
    logger: { disabled: true },
    baseURL,
    secret,
    database,
    trustedOrigins: [...new Set([appURL, baseURL])],
    advanced: {
      ...authOptions.advanced,
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
