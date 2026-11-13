# RoleCue frontend

Next.js 16 / React 19 application in the RoleCue Go monorepo. The canonical repository is https://github.com/cog-forge/rolecue.

## Local setup

Use Bun 1.4.2 and Node 24.20.0 to match CI. From `frontend/`:

```sh
bun install --frozen-lockfile
cp .env.example .env
bun run dev
```

Configure `NEXT_PUBLIC_API_BASE_URL` and the non-secret `AUTH_SOCIAL_PROVIDERS` display allowlist. Run `make dev` from the repository root to start PostgreSQL, Go, auth, Next on port 3002 and Caddy in Docker; open `http://localhost:3000` through Caddy. `bun run dev` alone starts Next on port 3002 without auth or Caddy. Provider, signing, mail and database secrets belong in `auth/.env`, never Next.

Next reads `frontend/.env`; Go reads the root `.env`; Bun auth reads `auth/.env`. Existing `.env.local` takes precedence in Next. Keep one local source consistently and never commit secrets. Public `NEXT_PUBLIC_*` values are embedded at build time.

## Better Auth contract

The browser SDK keeps the same-origin `/api/auth/*` API. Caddy routes this prefix directly to the standalone Better Auth 1.7.7 runtime and web paths to Next. Next contains no auth server route or proxy. Production ingress must provide the same path routing. Business Axios calls still go directly to Go with credentials. Go validates sessions through its private `AUTH_UPSTREAM_URL`, then checks current user policy in PostgreSQL.

One session authority supports multiple concurrent devices. The runtime preserves the 14-day rolling lifetime, one-day renewal, cookie prefix/domain/security attributes, verification and revocation behavior. The complete environment matrix, development commands, migration and rollback procedure are in [auth/README.md](../auth/README.md).

Use the same hostname for browser frontend and Go in local development (`localhost` for both). Production uses `rolecue.dorriss.com` and `rolecue-api.dorriss.com`, shared `dorriss.com` secure cookies and exact credentialed CORS origins. `AUTH_BASE_URL` is public HTTPS even when internal `AUTH_UPSTREAM_URL` uses HTTP.

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

The application includes authentication, candidate and recruiter workspaces, live interview sessions, avatar selection/conversion, rubric-based evaluation and reports, job applications, moderation and billing. These workflows depend on the configured Go API, auth runtime and external providers. Unit and browser tests usually mock those boundaries; they do not establish that a live provider is configured.

Avatar and interview rendering stay inside client boundaries; Three.js/VRM scenes require WebGL and have fallback presentation. Read the current feature source and tests for behavior rather than relying on earlier placeholder descriptions. `/state-preview` is available only during development for reviewing shared loading, error, empty and AI states.

Read `AGENTS.md` and `SKILL.md` before changing this app. See `../docs/AUTH-SINGLE-SESSION-SPEC.md` for the approved auth architecture.

## References

- [Better Auth sessions](https://better-auth.com/docs/concepts/session-management)
- [Better Auth cookies](https://better-auth.com/docs/concepts/cookies)
- [Next.js environment variables](https://nextjs.org/docs/app/guides/environment-variables)
