import { createHmac } from "node:crypto";
import { createAuthMiddleware } from "better-auth/api";
import type { BetterAuthOptions, BetterAuthPlugin } from "better-auth";

// Structured auth audit events (KAN-290). Better Auth's own logger stays disabled because it can print
// token-bearing URLs. Every event is built from an allow-list: an error code, never an error message, and
// a user id or salted email hash, never a raw email, password, token or cookie.
export type AuditEvent = {
  event:
    | "auth.sign_in"
    | "auth.two_factor"
    | "auth.lockout"
    | "auth.password_reset_request"
    | "auth.password_reset"
    | "auth.password_change"
    | "auth.account_lock";
  outcome: "success" | "failure" | "challenge" | "requested" | "blocked";
  reason?: string;
  userId?: string;
  emailHash?: string;
  ip?: string;
  at: string;
};
export type AuditSink = (event: AuditEvent) => void;

// One JSON line on stdout, which the platform log collector reads.
export const consoleSink: AuditSink = (event) => {
  console.info(JSON.stringify({ type: "auth_audit", ...event }));
};

// Codes that mean the account (or its second factor) is locked, as opposed to a plain wrong credential.
const lockoutCodes = new Set([
  "BANNED_USER",
  "ACCOUNT_TEMPORARILY_LOCKED",
  "TOO_MANY_ATTEMPTS_REQUEST_NEW_CODE",
]);

const signInPaths = new Set(["/sign-in/email"]);
const twoFactorPaths = new Set([
  "/two-factor/verify-totp",
  "/two-factor/verify-backup-code",
]);

type Loose = Record<string, unknown>;
const asRecord = (value: unknown): Loose | undefined =>
  value && typeof value === "object" ? (value as Loose) : undefined;

// Salted, keyed hash so the same address correlates across events without being recoverable from the log.
export function createEmailHasher(secret: string) {
  const key = createHmac("sha256", secret)
    .update("rolecue:audit-log:v1")
    .digest();
  return (email: unknown) =>
    typeof email === "string" && email.trim()
      ? createHmac("sha256", key)
          .update(email.trim().toLowerCase())
          .digest("hex")
          .slice(0, 16)
      : undefined;
}

function errorCode(returned: unknown): string | undefined {
  const record = asRecord(returned);
  if (!(returned instanceof Error) && !record?.statusCode && !record?.body)
    return undefined;
  const code = asRecord(record?.body)?.code;
  if (typeof code === "string" && /^[A-Z0-9_]{1,64}$/.test(code)) return code;
  return "UNKNOWN_ERROR";
}

function cleanIp(value: unknown) {
  const first = typeof value === "string" ? value.split(",")[0].trim() : "";
  return /^[0-9a-fA-F:.]{2,45}$/.test(first) ? first : undefined;
}

function requestIp(headers: Headers | undefined) {
  return cleanIp(
    headers?.get("x-forwarded-for") ?? headers?.get("x-real-ip") ?? undefined,
  );
}

export function createAuditLog(options: {
  secret: string;
  sink?: AuditSink;
  now?: () => Date;
}) {
  const sink = options.sink ?? consoleSink;
  const hashEmail = createEmailHasher(options.secret);
  const now = options.now ?? (() => new Date());
  const emit = (event: Omit<AuditEvent, "at">) => {
    try {
      sink({ ...event, at: now().toISOString() });
    } catch {
      // Auditing must never break the sign-in flow.
    }
  };

  const afterHook = createAuthMiddleware(async (ctx) => {
    const path = ctx.path;
    const returned: unknown = ctx.context.returned;
    const code = errorCode(returned);
    const failed = code !== undefined;
    const ip = requestIp(ctx.headers);
    const body = asRecord(ctx.body);
    const emailHash = hashEmail(body?.email);
    const returnedUser = asRecord(asRecord(returned)?.user);
    const userId =
      (typeof returnedUser?.id === "string" ? returnedUser.id : undefined) ??
      ctx.context.session?.user.id;

    if (signInPaths.has(path)) {
      if (failed) {
        emit({
          event: "auth.sign_in",
          outcome: "failure",
          reason: code,
          emailHash,
          ip,
        });
        if (lockoutCodes.has(code))
          emit({
            event: "auth.lockout",
            outcome: "blocked",
            reason: code,
            emailHash,
            ip,
          });
      } else if (asRecord(returned)?.twoFactorRedirect === true) {
        emit({
          event: "auth.sign_in",
          outcome: "challenge",
          reason: "TWO_FACTOR_REQUIRED",
          userId,
          emailHash,
          ip,
        });
      }
      // A completed sign-in is reported by the session hook, which also covers OAuth.
    } else if (twoFactorPaths.has(path)) {
      emit({
        event: "auth.two_factor",
        outcome: failed ? "failure" : "success",
        reason: code,
        userId,
        ip,
      });
      if (failed && lockoutCodes.has(code))
        emit({
          event: "auth.lockout",
          outcome: "blocked",
          reason: code,
          userId,
          ip,
        });
    } else if (path === "/request-password-reset") {
      // The endpoint answers the same for unknown addresses, so only the hashed address is logged.
      emit({
        event: "auth.password_reset_request",
        outcome: failed ? "failure" : "requested",
        reason: code,
        emailHash,
        ip,
      });
    } else if (path === "/reset-password") {
      emit({
        event: "auth.password_reset",
        outcome: failed ? "failure" : "success",
        reason: code,
        ip,
      });
    } else if (path === "/change-password") {
      emit({
        event: "auth.password_change",
        outcome: failed ? "failure" : "success",
        reason: code,
        userId,
        ip,
      });
    } else if (path === "/admin/ban-user" || path === "/admin/unban-user") {
      const target = typeof body?.userId === "string" ? body.userId : undefined;
      emit({
        event: "auth.account_lock",
        outcome: failed ? "failure" : "success",
        reason: failed
          ? code
          : path === "/admin/ban-user"
            ? "LOCKED"
            : "UNLOCKED",
        userId: target,
        ip,
      });
    }
  });

  const plugin = {
    id: "rolecue-audit-log",
    hooks: {
      after: [
        {
          matcher: (ctx) =>
            signInPaths.has(ctx.path ?? "") ||
            twoFactorPaths.has(ctx.path ?? "") ||
            [
              "/request-password-reset",
              "/reset-password",
              "/change-password",
              "/admin/ban-user",
              "/admin/unban-user",
            ].includes(ctx.path ?? ""),
          handler: afterHook,
        },
      ],
    },
  } satisfies BetterAuthPlugin;

  const databaseHooks = {
    session: {
      create: {
        after: async (session) => {
          emit({
            event: "auth.sign_in",
            outcome: "success",
            userId: session.userId,
            ip: cleanIp(session.ipAddress),
          });
        },
      },
    },
  } satisfies BetterAuthOptions["databaseHooks"];

  return { plugin, databaseHooks };
}
