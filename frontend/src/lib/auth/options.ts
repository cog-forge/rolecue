import type { BetterAuthOptions } from "better-auth";
import { admin, twoFactor } from "better-auth/plugins";
import { defaultAc, userAc } from "better-auth/plugins/admin/access";

const timestamps = { createdAt: "created_at", updatedAt: "updated_at" };

// Exported separately so generation and integration tests use the exact runtime mapping.
export const authOptions = {
  user: {
    modelName: "users",
    fields: { ...timestamps, emailVerified: "email_verified" },
  },
  session: {
    modelName: "sessions",
    fields: {
      ...timestamps,
      userId: "user_id",
      expiresAt: "expires_at",
      ipAddress: "ip_address",
      userAgent: "user_agent",
    },
    expiresIn: 14 * 24 * 60 * 60,
    updateAge: 24 * 60 * 60,
    cookieCache: { enabled: false },
  },
  account: {
    modelName: "accounts",
    fields: {
      ...timestamps,
      userId: "user_id",
      accountId: "account_id",
      providerId: "provider_id",
      accessToken: "access_token",
      refreshToken: "refresh_token",
      idToken: "id_token",
      accessTokenExpiresAt: "access_token_expires_at",
      refreshTokenExpiresAt: "refresh_token_expires_at",
    },
    accountLinking: { enabled: true, trustedProviders: [] },
  },
  verification: {
    modelName: "verifications",
    fields: { ...timestamps, expiresAt: "expires_at" },
  },
  advanced: { database: { generateId: "uuid" } },
  disabledPaths: [
    "/admin/impersonate-user",
    "/admin/stop-impersonating",
    "/admin/create-user",
    "/admin/remove-user",
    "/admin/set-role",
    "/admin/set-user-password",
    "/admin/update-user",
    "/admin/list-user-sessions",
    "/admin/revoke-user-session",
    "/admin/revoke-user-sessions",
  ],
  plugins: [
    admin({
      defaultRole: "candidate",
      adminRoles: ["admin"],
      roles: {
        candidate: userAc,
        recruiter: userAc,
        admin: defaultAc.newRole({ user: ["list", "get", "ban"], session: [] }),
      },
      schema: {
        user: {
          fields: {
            banned: "is_locked",
            banReason: "lock_reason",
            banExpires: "lock_expires_at",
          },
        },
        session: { fields: { impersonatedBy: "impersonated_by" } },
      },
    }),
    // Reserve the pinned plugin schema without exposing enable/verify/recovery endpoints.
    {
      id: "two-factor-schema",
      schema: twoFactor({
        schema: {
          user: { fields: { twoFactorEnabled: "two_factor_enabled" } },
          twoFactor: {
            modelName: "two_factors",
            fields: {
              backupCodes: "backup_codes",
              userId: "user_id",
              failedVerificationCount: "failed_verification_count",
              lockedUntil: "locked_until",
            },
          },
        },
      }).schema,
    },
  ],
} satisfies BetterAuthOptions;
