import type { BetterAuthOptions } from "better-auth";

// Explicit limits, not NODE_ENV-dependent: Better Auth enables rate limiting only in production by
// default and keeps counters in process memory. Counters live in PostgreSQL so every replica and
// restart shares them. Windows are seconds; max is requests per window, per client IP and path.
export const rateLimitOptions = {
  enabled: true,
  storage: "database",
  modelName: "rate_limits",
  fields: { key: "key", count: "count", lastRequest: "last_request" },
  customRules: {
    // Password guessing: a real user needs at most a few tries per minute.
    "/sign-in/email": { window: 60, max: 5 },
    // Each call sends mail, so keep it well below what a mailbox flood needs.
    "/request-password-reset": { window: 60, max: 3 },
    "/send-verification-email": { window: 60, max: 3 },
    // Six-digit TOTP codes and backup codes are brute-forceable without a per-IP cap.
    "/two-factor/verify-totp": { window: 60, max: 5 },
    "/two-factor/verify-backup-code": { window: 60, max: 5 },
  },
} satisfies NonNullable<BetterAuthOptions["rateLimit"]>;

// The client IP decides the rate-limit key. Caddy (.caddy/Caddyfile.deploy) does not set X-Real-IP;
// its reverse proxy appends the peer address to X-Forwarded-For, so a client-supplied value
// arrives as an extra entry. Better Auth accepts only a single-valued X-Forwarded-For when no
// trustedProxies are configured, so a spoofed header yields no IP and falls into one shared
// bucket instead of a bypass. Pinning the header name keeps the key independent of library
// defaults. Set trustedProxies if Caddy ever moves behind another hop.
export const rateLimitIpAddress = {
  ipAddressHeaders: ["x-forwarded-for"],
} satisfies NonNullable<BetterAuthOptions["advanced"]>["ipAddress"];
