import "server-only";
import { betterAuth } from "better-auth";
import { Pool } from "pg";
import { authOptions } from "./options";
import {
  assertAuthEmailConfigured,
  sendPasswordResetEmail,
  sendVerificationEmail,
} from "@/lib/email/server";

export function createAuth(database?: Pool) {
  const baseURL = process.env.BETTER_AUTH_URL;
  const secret = process.env.BETTER_AUTH_SECRET;
  const connectionString = process.env.BETTER_AUTH_DATABASE_URL;
  if (!baseURL || !secret || secret.length < 32 || !connectionString)
    throw new Error(
      "Configure BETTER_AUTH_URL, BETTER_AUTH_SECRET (32+ characters), and BETTER_AUTH_DATABASE_URL",
    );
  const origin = new URL(baseURL);
  if (
    origin.origin !== baseURL ||
    !["http:", "https:"].includes(origin.protocol)
  )
    throw new Error("BETTER_AUTH_URL must be an exact HTTP(S) origin");
  const domain = process.env.BETTER_AUTH_COOKIE_DOMAIN;
  if (
    domain &&
    (origin.protocol !== "https:" ||
      domain !== "dorriss.com" ||
      !origin.hostname.endsWith(".dorriss.com"))
  )
    throw new Error("Shared cookies require an HTTPS dorriss.com origin");
  assertAuthEmailConfigured("verification");
  assertAuthEmailConfigured("password-reset");
  return betterAuth({
    ...authOptions,
    baseURL,
    secret,
    database:
      database ??
      new Pool({ connectionString, max: 10, connectionTimeoutMillis: 5000 }),
    trustedOrigins: [origin.origin],
    advanced: {
      ...authOptions.advanced,
      cookiePrefix:
        process.env.BETTER_AUTH_COOKIE_PREFIX || "rolecue",
      useSecureCookies: origin.protocol === "https:",
      defaultCookieAttributes: {
        httpOnly: true,
        secure: origin.protocol === "https:",
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
        await sendPasswordResetEmail(user, url);
      },
    },
    emailVerification: {
      sendOnSignUp: true,
      sendOnSignIn: false,
      autoSignInAfterVerification: true,
      sendVerificationEmail: async ({ user, url }) => {
        await sendVerificationEmail(user, url);
      },
    },
    socialProviders: {
      ...(process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET
        ? {
            google: {
              clientId: process.env.GOOGLE_CLIENT_ID,
              clientSecret: process.env.GOOGLE_CLIENT_SECRET,
            },
          }
        : {}),
      ...(process.env.GITHUB_CLIENT_ID && process.env.GITHUB_CLIENT_SECRET
        ? {
            github: {
              clientId: process.env.GITHUB_CLIENT_ID,
              clientSecret: process.env.GITHUB_CLIENT_SECRET,
            },
          }
        : {}),
      ...(process.env.FACEBOOK_CLIENT_ID && process.env.FACEBOOK_CLIENT_SECRET
        ? {
            facebook: {
              clientId: process.env.FACEBOOK_CLIENT_ID,
              clientSecret: process.env.FACEBOOK_CLIENT_SECRET,
            },
          }
        : {}),
    },
  });
}
let auth: ReturnType<typeof createAuth> | undefined;
export function getAuth() {
  return (auth ??= createAuth());
}
