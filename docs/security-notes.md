# Security notes

Accepted risks and exceptions from the security-hardening review. Revisit when the reason changes.

## KAN-306 informational findings

- **Reset-password token in the URL query.** Accepted. The token arrives in the emailed link and
  Better Auth reads it from the query string; moving it would mean a custom flow. The token is
  single-use and short-lived, and the Caddy error log excludes request URLs. The app sends
  `Referrer-Policy: strict-origin-when-cross-origin`, so the token does not leak in referrers to
  other origins.
- **No Next middleware guarding workspace routes.** Accepted. Workspace pages are client-gated
  (`SessionGate`, `RoleGate`) for presentation only; every sensitive operation is authorised by the
  Go API (ownership and role checks) and Better Auth. A proxy-level redirect would only add a
  second, weaker check on the cookie's presence. `src/proxy.ts` exists for the CSP nonce only.

## Dependency pins (KAN-306d)

`@lottiefiles/dotlottie-react`, `axios` and `sonner` were pinned to the versions already resolved in
`frontend/bun.lock` (0.20.0, 1.20.0, 2.0.8). Only the workspace specifiers in `package.json` and
`bun.lock` changed; no resolution changed.

## Audit exception GHSA-vfj7-8cjw-p6xm (KAN-306e)

The `Makefile` `frontend-audit` target ignores this `braces` advisory (tooling-only, via
`micromatch`). `frontend/bun.lock` still resolves `braces@3.0.3` (`micromatch@4.0.8` depends on
`^3.0.3`), so the exception cannot be removed yet. The expiry was extended by 60 days, from
2026-11-17 to 2027-01-16 UTC. Re-check `bun audit` before that date and drop the exception once a
patched `braces` is resolvable. Not verified with `bun audit` locally (bun is not installed here).

## CSP rollout (KAN-258)

The frontend sends `Content-Security-Policy-Report-Only` until `CSP_ENFORCE=true` is set on the
frontend container. Watch browser console violations in the interview room, the Avaturn avatar
creator and the PayOS redirect, then enforce.
