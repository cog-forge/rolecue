# Interview Practice frontend

Frontend architecture for the AI Virtual Technical Interview Platform. This directory is the Next.js application root inside the Go monorepo.

## Quick start

Use Bun 1.4.0 and Node 22.12+ (Node 26.7.0 was used for verification). From this directory:

```sh
bun install --frozen-lockfile
bun run dev
```

The UI and build do not require a running backend. Copy `.env.example` to `.env.local` and set `NEXT_PUBLIC_API_BASE_URL` to the Go backend origin to use Login. Public variables are build-time values, never secrets.

## Stack

Next.js App Router, React 19, strict TypeScript, Tailwind v4 and shadcn/ui (Radix Nova, neutral, Lucide). Axios is the shared browser HTTP client. TanStack Query owns remote async state, RHF + Zod own forms, and Zustand holds the cross-route access token. Three.js, React Three Fiber and Drei are ready for the future avatar integration.

See package.json for exact pinned versions and bun.lock for resolved dependencies.

## Commands

| Command                               | Purpose                                               |
| ------------------------------------- | ----------------------------------------------------- |
| bun run dev                           | Development server                                    |
| bun run build                         | Production compilation and prerendering               |
| bun run start                         | Serve the production build                            |
| bun run lint                          | Next.js core web vitals and TypeScript ESLint rules   |
| bun run typecheck                     | Generate route types, then strict TypeScript checking |
| bun test                              | Native Bun bridge that runs the Vitest suite          |
| bun run test                          | Run Vitest directly                                   |
| bun run test:watch                    | Vitest watch mode                                     |
| bun run test:e2e -- --list            | Discover Playwright tests                             |
| bun run test:e2e                      | Smoke test against an existing production build       |
| bun run format / bun run format:check | Format / verify frontend files                        |

Playwright uses port 3100 and an already installed Chromium browser. Run build first. Set PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH when reusing an existing compatible Chromium executable instead of Playwright's expected revision. Browser binaries and operating-system dependencies are not installed automatically. Unit/component tests live under `tests/unit/`, mirroring source paths; browser tests live under `tests/e2e/`.

## Source structure

- src/app: Server Component pages, route groups, layouts, redirects and error/loading boundaries.
- src/features: auth, dashboard, job-description, interview-setup, interview, report, history, billing, profile and admin.
- src/components: shared layout, feedback and domain-neutral shadcn primitives.
- src/lib: shared Axios client, API configuration and styling utility.
- src/config: routes, navigation, environment, site metadata and role vocabulary.
- src/providers: per-browser-lifecycle QueryClient and application provider composition.
- tests: Vitest setup, native Bun bridge and Playwright smoke test.

No global types folder or empty public asset directories are needed yet. Feature-specific contracts stay beside their owner.

## Architecture and workflow

Read AGENTS.md and SKILL.md before changing this frontend. Prefer Server Components; introduce client boundaries for interaction or browser runtime ownership only. URL routes determine wizard steps and allow refresh/back/deep links. Step guards and draft persistence are intentionally pending actual draft semantics.

The browser calls the Go backend directly through the shared Axios client; no Next.js proxy is used. Its request interceptor reads the minimal Zustand access-token state and adds a bearer header. Login validates the known request and `{ success: true, data: string }` response with Zod. Axios preserves backend error payloads for the Login toast.

Feature integration follows request function → Query wrapper → orchestration → UI. interviewKeys demonstrates hierarchical key factories without a fabricated request. Do not put navigation or notifications inside transport/query functions.

The interview machine is deterministic TypeScript independent of React and Zustand. Invalid transitions are no-ops, terminal states require RESET, and resume enters synchronization through RECONNECTING. The 2D state is an acknowledgement gate; persistent renderer mode and authoritative session resumption must be designed with the runtime contract.

The avatar stage is a small unmounted client boundary; future callers must dynamically import it with SSR disabled. There are no models or fake lip-sync. Browser capability checks are user-triggered and only test availability; they do not verify permissions, actual devices or connection quality.

## Current status and integration limits

All planned public/auth/candidate/admin route placeholders render without backend services. /interviews/new redirects to its first explicit step; /admin redirects to /admin/dashboard. Dynamic room/report routes display references without inventing session records.

**Candidate and admin layouts are not authorization barriers.** Login uses the current custom Go `/auth/login` contract and stores the returned access token in tab-scoped sessionStorage; that client state does not make layouts authorization barriers. Implement server/backend authorization before exposing sensitive data.

Social provider routes are informational previews; real OAuth is not connected. Registration and password recovery are placeholders. AI analysis, interview wire protocol, voice capture/playback, resume reconciliation, evaluation/report data, avatar assets and payment gateway integration remain unimplemented. Billing belongs behind backend/provider boundaries, including credits, packages, checkout, transactions and optional invoices/subscriptions.

The Login form uses RHF/Zod and Sonner for backend error feedback. Semantic theme variables remain ready for a later visual system. DESIGN.md is intentionally absent.

## References

- [Next.js installation](https://nextjs.org/docs/app/getting-started/installation)
- [shadcn Tailwind v4](https://ui.shadcn.com/docs/tailwind-v4)
