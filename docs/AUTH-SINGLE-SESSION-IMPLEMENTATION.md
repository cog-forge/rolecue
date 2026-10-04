# Better Auth single-session implementation

Implemented on 2026-10-03 in `Better-Auth-Authentication`, preserving the previously ported UI. Better Auth 1.7.7 runs in Next; Go validates its session through a fixed upstream get-session URL and reads the latest application user. There is no JWT/Bearer fallback, exchange, JWKS or Go-owned session store.

## Setup

1. The root Compose file stays unchanged; `api/compose.local.override.yaml` sets the isolated container name and healthcheck when invoked through Make. Root `.env` configures Go and Compose; frontend `.env.local` configures Next (and overrides a supplied frontend `.env`). Use database `rolecue_authentication` on localhost port 5434 for both applications. The separate container is `rolecue-authentication-postgres`, with Compose project/volume `rolecue-authentication`.
2. `make -C api migrate` starts the DB, waits for readiness and applies migrations 000001–000003. Do not apply `frontend/better-auth-schema.sql` separately. This is generated review evidence; migration 000003 is authoritative.
3. After the user disabled WARP, the default Docker bridge network was retested successfully with `make -C api run`; no host-network flag is required on this machine. The temporary host-network fallback was removed. The sibling spike container on 5433 is untouched.
4. Supply Better Auth secret (32+ characters), Resend sender/key and the two approved template IDs; provider buttons require both provider ID and secret. Start `make -C api run` and `cd frontend && bun run dev`, consistently using `localhost:3000` and `localhost:8080`.
5. Production: Next `https://rolecue.dorriss.com`, Go `https://rolecue-api.dorriss.com`; `BETTER_AUTH_COOKIE_DOMAIN=dorriss.com`, `AUTH_SECURE_COOKIES=true` on Go, exact frontend origin for credentialed CORS and `NEXT_PUBLIC_API_BASE_URL=https://rolecue-api.dorriss.com` before building. Configure the three provider callback URLs under `/api/auth/callback/{google,github,facebook}`. HTTPS termination must preserve Set-Cookie individually.

Cookies have prefix `rolecue-authentication`; HTTPS adds `__Secure-`. Development cookies are host-only; production cookies use Domain `dorriss.com`, Path `/`, HttpOnly, Secure and SameSite Lax. Session lifetime is rolling 14 days, with a one-day updateAge; cookie cache and Go auth cache are disabled. The optional "remember me" behavior is owned by the SDK.

Go forwards only SDK auth cookies to `/api/auth/get-session?disableCookieCache=true`, uses a finite timeout and no redirects, bounds response bodies, validates session/user UUID linkage and expiry, and relays validated Set-Cookie headers individually. Missing/expired/revoked session is 401; locked/unverified/unauthorized application users and untrusted write Origins are 403; upstream timeout/5xx/malformed response or DB outage is 503. Client gates show retry on outage rather than pretending logout succeeded.

The shared users table owns application identity and FK targets; accounts now stores provider credentials. Migration preserves existing owner UUIDs/business rows but intentionally invalidates old passwords/verification. Down migration is a dev schema rollback, not credential recovery. Restore a snapshot for actual data/credential rollback. No CASCADE drop of business tables is used.

Admin role is bootstrapped by an operator in the isolated database after verifying a user. The UI exposes paginated user listing and lock/unlock through Admin plugin only. Candidate/recruiter have no admin privileges; admin permissions are restricted to list/get/ban. Impersonation and unrelated admin endpoints are disabled. Signup/profile edits cannot set roles, bans or 2FA status. Ban revokes sessions; unban does not revive them.

The two-factor table/columns come from the pinned plugin schema; no runtime 2FA routes or user enablement are exposed. Profile/onboarding, change-password UI, username and 2FA remain separate follow-ups.

## Automated evidence

| Check                                                  | Result                                                                                      |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| Frozen Bun install                                     | Pass; no lockfile changes                                                                   |
| Frontend lint/typecheck/production build               | Pass                                                                                        |
| Unit/component suite                                   | 39 tests pass                                                                               |
| SDK integration with real PostgreSQL/HTTP and Go relay | 5 tests pass; unique DB created and removed per run                                         |
| Public/auth UI Chromium smoke                          | 4 tests pass                                                                                |
| Actual Next + Go browser login/renewal/reload/logout   | Pass on localhost                                                                           |
| Equivalent two-subdomain TLS browser topology          | Pass; Secure/domain cookies, Go-only renewal and old-cookie rejection                       |
| Live Next → Resend verification/resend/reset           | Pass against official delivery simulation addresses, using configured credentials/templates |
| Go vet/unit tests/build                                | Pass                                                                                        |
| Go migration/repository integration                    | Pass against disposable test databases                                                      |
| Go golangci-lint                                       | 0 issues                                                                                    |
| SQLC and Swagger generation                            | Pass; generated JD sources unchanged; Swagger updated                                       |
| Diff whitespace/private secret scan                    | Pass; no configured private credentials in source changes                                   |

The aggregate `make check` stops at an existing gofmt discrepancy in untouched `api/internal/pkg/logger/logger.go`. Vet/test/lint/build ran separately and passed; all modified Go files are formatted. This pre-existing formatting issue is recorded rather than folded into the authentication change.

Swagger output remains Swagger 2.0, which cannot model native cookie security. `SessionCookie` documents the Cookie request header and actual cookie names; browsers manage this header automatically. The old Bearer security scheme and `/auth/login` are absent.

## Reproduce tests

From `frontend/`, set `AUTH_TEST_DATABASE_URL` to the isolated server DSN, then run `bun run test:integration`. This suite creates unique `test_rolecue_*` databases and never migrates the supplied database. Go integration helpers and the TEST_POSTGRES_URL/testcontainers path were removed during review cleanup; use `go test ./...` for the remaining Go suite.

Build the default frontend, then run Chromium smoke:

```sh
bun run build
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome-stable bun run test:e2e
```

Set `AUTH_E2E_DATABASE_URL` to the migrated `rolecue_authentication` database, then run `bun run test:e2e --config playwright.auth.config.ts`. This seeds a generated verified user using the SDK password hasher and deletes it afterward; real Next/Go handle login/session checks. Explicitly set `AUTH_LIVE_EMAIL_CHECK=1` to send three messages through the actual Resend API to uniquely labelled `delivered+...@resend.dev` simulation addresses. Auth traces are disabled to avoid retaining session/password material.

For equivalent HTTPS topology (from frontend):

```sh
mkdir -p /tmp/rolecue-auth-tls
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout /tmp/rolecue-auth-tls/key.pem -out /tmp/rolecue-auth-tls/cert.pem \
  -days 1 -subj '/CN=rolecue.dorriss.com' \
  -addext 'subjectAltName=DNS:rolecue.dorriss.com,DNS:rolecue-api.dorriss.com'
chmod 600 /tmp/rolecue-auth-tls/key.pem
NEXT_PUBLIC_API_BASE_URL=https://rolecue-api.dorriss.com:8444 bun run build
# Set AUTH_E2E_DATABASE_URL privately before running this command.
AUTH_E2E_TLS=1 SSL_CERT_FILE=/tmp/rolecue-auth-tls/cert.pem \
  AUTH_TLS_KEY_FILE=/tmp/rolecue-auth-tls/key.pem \
  PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome-stable \
  bun run test:e2e --config playwright.auth.config.ts
bun run build
```

The harness uses local TLS proxies on 8443/8444, Chromium-only hostname mapping and a restricted CONNECT proxy for Go. Go verifies the temporary test CA; Chromium explicitly accepts the test certificate. No hosts file/system trust store changes or remote production deployment occur. The last build restores the localhost API destination. Stop existing Go/Next instances on 8080/3100 before these tests.

## Human/account checks still pending

- Actual inbox receipt and opening verification/reset links: recipient not provided yet. SDK link consumption/expiry/reuse and session revocation passed automatically; Resend simulation API acceptance does not prove inbox delivery.
- Full Google/GitHub/Facebook consent and callback: all configured providers generate the expected callback/state; browser Google/GitHub/Facebook entry reaches each provider login screen. The isolated browser is not signed into test provider accounts. Do not mark `account_not_linked` as successful linking.
- Real deployment at the two public production URLs: not deployed or tested; the equivalent local TLS topology passed.

No commit/push was requested. Tasks remain working notes outside the commit scope, as required by the plan.

## Sources

Pinned local Better Auth 1.7.7 code and [database mapping](https://better-auth.com/docs/concepts/database), [cookie options](https://better-auth.com/docs/concepts/cookies), [Admin plugin](https://better-auth.com/docs/plugins/admin). [Resend test email documentation](https://resend.com/docs/dashboard/emails/send-test-emails) specifies that its delivery addresses simulate events and consume sending quota; they are not an inbox validation.

## Review follow-up

The backend was committed in six API-only commits; the frontend remains uncommitted for review. Unused Go integration-test infrastructure and testcontainers dependencies were removed at the user's request. The earlier integration evidence above records verification before that cleanup.

Facebook login created a candidate with `email_verified=false`; Go correctly denied application access. The frontend now explains the missing email verification, lets the signed-in user request a link and recheck access, and keeps protected content gated by Go. Provider emails are not automatically marked verified. Registration has independent show/hide buttons for both password fields. Goey Toast 0.5.0 replaces the previous Sonner rendering and uses short titles with explanatory descriptions.

Latest validation: frontend lint/typecheck/build, 39 unit/component tests and three real-browser tests passed. The browser suite covers login/renewal/logout, the unverified social-session state and recovery after verification, registration eye toggles, and success/error toast rendering. The email-send response in the new unverified-session browser case is mocked; the earlier live Resend delivery evidence is separate.

Auth feedback now includes successful sign-in/sign-out, account creation, verification email sends, verification completion, password recovery/update and admin lock/unlock, with error toasts for failed actions. Sign-in and verification callbacks carry a one-time UI marker; the session gate consumes it only after validating the account and removes it from the URL so reload/refetch does not repeat the toast. Recovery keeps a generic informational response without disclosing account existence.

The two frontend scripts are development utilities: `generate-auth-schema.ts` exports pinned Better Auth SQL from an empty schema for review; `auth-tls-proxy.ts` is the temporary local HTTPS/proxy harness used by the TLS browser tests. Neither is needed for `bun run dev`.
