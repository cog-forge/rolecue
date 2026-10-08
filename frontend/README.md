# RoleCue frontend

Next.js 16 / React 19 application in the RoleCue Go monorepo. The canonical repository is https://github.com/cog-forge/rolecue.

## Local setup

Use Bun 1.4.x and Node 22.12+. From `frontend/`:

```sh
bun install --frozen-lockfile
cp .env.example .env
bun run dev
```

Fill in the auth database, a random `BETTER_AUTH_SECRET` of at least 32 characters, and all four Resend settings before using authentication. Public pages can render without backend services; login, registration, verification, recovery, and protected API calls need their configured services. Enable each OAuth provider by supplying both its client ID and secret.

Next.js reads environment files in `frontend/`; it does not automatically read the root `.env`. Go's `make dev` / `make run` loads the root `.env`. `.env.local` takes priority over `.env` in Next.js, so use one local file consistently. Keep secrets out of Git. `NEXT_PUBLIC_*` values are public and embedded at build time; rebuild after changing them.

From `api/`, `make dev` starts the local database, applies migrations, and starts Go. Configure the root `.env` first. The `DB_*` settings configure local PostgreSQL; `DATABASE_URL` is the Go/migration connection string and takes precedence over Go's individual DB settings.

## Better Auth and environment contract

Better Auth runs in Next.js at `/api/auth/*` and owns users, credential accounts, verification records, and login sessions. Browser authentication calls use the Better Auth SDK. Business requests use the shared Axios client with `withCredentials: true` and go directly to Go. Go forwards the allowed session cookies to Next's `/api/auth/get-session`, then reads the current user role, verification status, and lock status from PostgreSQL. There is one session system; several devices can each have their own session.

| Setting                     | Owner         | Meaning                                                                                                                 |
| --------------------------- | ------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `NEXT_PUBLIC_API_BASE_URL`  | Frontend      | Go origin; localhost default example is `http://localhost:8080`.                                                        |
| `BETTER_AUTH_URL`           | Both          | Next/auth origin, `http://localhost:3000` locally. Go must be able to reach it. No trailing slash or path.              |
| `BETTER_AUTH_SECRET`        | Frontend only | Signs auth data; use a stable random secret of at least 32 characters.                                                  |
| `BETTER_AUTH_DATABASE_URL`  | Frontend      | Must target the same migrated database as root `DATABASE_URL`.                                                          |
| `BETTER_AUTH_COOKIE_PREFIX` | Both          | Cookie namespace. Values must match; changing it makes existing cookies unusable.                                       |
| `BETTER_AUTH_COOKIE_DOMAIN` | Both          | Empty on localhost. The approved production topology uses `dorriss.com` to share cookies across Next and Go subdomains. |
| `AUTH_SECURE_COOKIES`       | Go/root       | `false` for local HTTP, `true` for production HTTPS; Next derives this from `BETTER_AUTH_URL`.                          |
| `AUTH_SESSION_TIMEOUT`      | Go/root       | Maximum wait for the Next session verification request, currently `5s`. This is not the session lifetime.               |
| `CORS_ALLOW_ORIGINS`        | Go/root       | Exact browser frontend origin; never `*` with credentialed requests.                                                    |
| `CORS_ALLOW_CREDENTIALS`    | Go/root       | `true` to allow browser cookie credentials.                                                                             |
| OAuth / `RESEND_*`          | Frontend only | Provider credentials and verification/reset email templates.                                                            |

The session lifetime is configured in `src/lib/auth/options.ts`: `expiresIn = 14 days`, `updateAge = 1 day`. Using an eligible session after the refresh threshold extends its expiry; this is a rolling lifetime. Cookie caching is disabled so verification checks the database. Logout revokes the current session; successful password reset revokes all sessions. `HttpOnly` prevents JavaScript from reading the session token; `Secure` limits transport to HTTPS; `SameSite=Lax` restricts cross-site sending. Subdomains of the same site still require CORS when their origins differ.

Local cookie scope is the hostname, not the port. Use `localhost` for both apps rather than mixing `localhost` and `127.0.0.1`. Production is currently explicitly constrained in code to Next `https://rolecue.dorriss.com`, Go `https://rolecue-api.dorriss.com`, shared cookie domain `dorriss.com`, secure cookies, and exact frontend CORS origin. Deploying to another domain requires changing the validation contract in both apps.

Different `.env.example` and `.env` values are expected: examples contain placeholders, while `.env` contains local credentials. The two examples intentionally expose different variables because the apps have different responsibilities. Copying only the frontend env into the root does not configure Go.

## Commands

| Command                      | Purpose                                            |
| ---------------------------- | -------------------------------------------------- |
| `bun run dev`                | Development server                                 |
| `bun run lint`               | ESLint                                             |
| `bun run typecheck`          | Route types and strict TypeScript                  |
| `bun test`                   | Bun bridge to Vitest                               |
| `bun run build`              | Production build                                   |
| `bun run start`              | Production server                                  |
| `bun run test:e2e -- --list` | Discover browser tests                             |
| `bun run test:e2e`           | Playwright against a production build on port 3100 |

Playwright requires an installed Chromium. Set `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` to reuse a compatible executable. Unit/component tests belong in `tests/unit/`; browser tests belong in `tests/e2e/`.

## Architecture and current limits

Routes compose feature components; shared UI is under `src/components`, infrastructure under `src/lib`, and providers under `src/providers`. Server Components are the default. TanStack Query owns remote state and RHF/Zod owns forms. No bearer token or Zustand access-token store is used by current auth.

Candidate/admin layouts use a client `SessionGate` backed by Go `/auth/me`. It controls displayed UI; it is not a server authorization check for Server Components or future Server Actions. All sensitive operations must enforce authorization at their server/API boundary. Go business endpoints enforce authentication and ownership; Better Auth's admin plugin enforces administrative permissions.

Email/password registration, login, verification, recovery, configured OAuth entry points, sign-out, and admin lock/unlock are implemented. AI interview runtime, avatar assets, voice capture, evaluation/report data, payments, and much of the candidate UI remain placeholders. The interview state machine is a pure local lifecycle foundation, not a wire protocol. The avatar component is unmounted; future usage must load it dynamically from a client boundary with SSR disabled.

Read `AGENTS.md` and `SKILL.md` before changing this app. See `../docs/AUTH-SINGLE-SESSION-SPEC.md` for the approved auth architecture.

## References

- [Better Auth sessions](https://better-auth.com/docs/concepts/session-management)
- [Better Auth cookies](https://better-auth.com/docs/concepts/cookies)
- [Next.js environment variables](https://nextjs.org/docs/app/guides/environment-variables)
