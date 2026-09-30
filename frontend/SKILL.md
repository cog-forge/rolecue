# Frontend engineering playbook

## Stack

Use Next.js 16 App Router, React 19, strict TypeScript, Bun, Tailwind v4, shadcn/ui and Lucide. Axios is the shared browser HTTP client. Server state uses TanStack Query. Forms use React Hook Form + Zod + @hookform/resolvers. Zustand is reserved for genuine cross-route client state. The interview renderer uses Three.js/R3F/Drei; browser WebSocket, Web Audio and MediaRecorder remain native. Tests use Vitest, RTL, jest-dom, jsdom and Playwright.

## Commands

Run from frontend/: bun install, bun run dev, bun run lint, bun run typecheck, bun test, bun run test:watch, bun run build, bun run start, bun run test:e2e -- --list, bun run test:e2e. Use bun run format and format:check for formatting. Keep only bun.lock. The Bun bridge delegates to Vitest; do not mix Bun test APIs into src tests. Build before E2E.

## Architecture

Compose routes → features → shared presentation/infrastructure. Browser UI calls Go directly through one shared Axios client; do not add fetch-wrapper stacks or Next.js proxy/API routes without a concrete server/BFF requirement. TanStack Query owns remote async state, RHF + Zod own forms, and Zustand owns only genuine cross-route client state. Avoid one-line wrappers. Add folders only with meaningful content.

## Folder Ownership

app owns route composition and framework boundaries. features owns business components, validation and APIs. src/stores owns genuine cross-feature client state. components/ui contains domain-neutral shadcn primitives; components/layout and feedback contain reusable presentation. lib owns platform infrastructure, config owns route/environment/navigation rules, providers owns application React context. Shared types are introduced only for truly cross-domain contracts. public belongs at the frontend root when assets exist.

## Next.js / RSC Rules

Prefer Server Components, including pages and root layout. Mark only interactive or browser-dependent entry points with use client. Async dynamic route params are promises. Route groups must not add candidate URL prefixes; admin is a real segment. Wizard steps live at explicit URLs; never replace them with a Zustand currentStep. Implement server/backend session checks once auth is real; existing layouts do not protect anything. Keep per-user credentials and query caches isolated between server requests.

## Feature Rules

Keep components, schemas and contracts near their feature. Avoid giant global hooks/services/types buckets, unused boilerplate and broad barrels. Do not reach into unrelated feature internals. interview-setup may consume the interview subsystem's browser-capability function as an explicit preflight interface; other interview internals remain private to that subsystem.

## API Rules

Use one shared Axios instance configured from `NEXT_PUBLIC_API_BASE_URL`, with browser credentials and the current access-token bearer interceptor. Preserve Axios response payloads for feature validation and messages; do not recreate them as a generic ApiError hierarchy. Do not add refresh/retry logic or a Next.js proxy without a concrete requirement.

Feature API functions validate known request/response contracts. Query mutations may call those functions directly. Navigation follows a successful real operation. Do not create one-line hooks, repositories, adapters or wrappers for endpoints that do not exist.

## TanStack Query Rules

One QueryClient per browser lifecycle; new instances on the server. Current defaults: 60-second stale time, one query retry, no focus refetch and no mutation retry. Override per domain where appropriate; do not add uncontrolled polling. Devtools load only in development.

Use domain key factories, as in interview/api/keys.ts. Lists and details form explicit key hierarchies; include filters in keys. Invalidate the narrowest relevant factory key after mutations. Never mirror query data into Zustand or fetch server data in ad hoc effects.

## Zustand Rules

No global useAppStore. Install does not imply a store is needed. Put genuine cross-feature client state in `src/stores/`; feature-specific drafts may stay with their feature. Expose reset, consume selectors and keep the current wizard step in the URL. Do not mirror backend records into Zustand. Keep the current access token tab-scoped in sessionStorage; document the lifetime and cleanup of any new persistence.

## React Hook Form / Zod Rules

RHF owns form values/errors; schemas live beside their feature API contract. Use the schema input/output types when transforms differ. Prefer existing shadcn primitives for interactive controls and native semantic HTML for headings, descriptions and simple layout. Keep labels, error descriptions and pending/disabled state accessible. Decode external data from unknown with Zod at real API boundaries.

## shadcn Rules

components.json configures TypeScript, RSC, Tailwind v4 variables and Lucide using Radix Nova. Keep primitives domain-neutral. Prefer existing Button, Input, Field, Label, Alert, Dialog, Sheet, Tabs, Accordion and Separator components; add only primitives that are used. Normalize generated cn imports to @/lib/utils and preserve the local neutral font setup. Avoid installing the whole catalog. Keep business cards/avatars/reports in features.

## TypeScript Rules

Use strict mode and @/* → src/*. Prefer literal unions, inferred schema types and unknown at external boundaries. No any, ts-ignore, ts-nocheck, unsafe assertions or broad lint disables. Distinguish application-facing contracts from unconfirmed wire data. Favor explicit arguments and narrow return contracts.

## Interview Engine Rules

machine/interview-machine.ts is pure deterministic TypeScript. No React, Zustand, network or timers inside transitions. Events in that file are local lifecycle vocabulary, not socket messages. Invalid transitions return the current state. Terminal states accept RESET only. Pause/resume and reconnect require future authority reconciliation; never infer the previous conversation phase from a socket reconnect.

DEGRADED_2D currently gates fallback acknowledgement. Introduce separate rendering-mode context when the actual runtime is built. Add representative state-transition tests when lifecycle semantics change. Do not claim full protocol coverage or introduce XState during bootstrap.

## Audio / WebSocket / Three.js Rules

No browser API execution at module import during SSR. Native WebSocket must sit behind InterviewTransport and a schema-validated parser once wire messages are known. Runtime code must own aborts, listeners, reconnect backoff and teardown. Do not connect to fake URLs.

Request microphone/camera permissions only after user action. Release tracks, recorder handlers, audio nodes/contexts and object URLs on teardown. Capability availability is not readiness or permission confirmation.

AvatarStage is a client component, unmounted until assets exist. Load it using a client dynamic import with ssr: false. Keep Canvas out of general route bundles. Provide a 2D alternative and error boundary when mounting; dispose resources. No generated GLB files, invented visemes or fake lip sync.

## Billing Boundary

Own credits, packages, checkout and transactions under features/billing. Future invoice/subscription features depend on product decisions. The backend is authoritative for prices, balances and payment status. Keep payment-provider SDKs/contracts outside components; do not pick a gateway until chosen. Never simulate paid status, checkout completion or grant credits client-side.

## Testing Rules

Put meaningful unit/component tests under `tests/unit/`, mirroring production source paths; do not colocate tests with source. Use Vitest with jsdom and RTL cleanup. Test behavior and accessible interaction. E2E belongs in `tests/e2e/`. Never install machine packages to force E2E execution without authorization.

## Naming Conventions

Use kebab-case files/directories, PascalCase components, useSomething hooks and somethingKeys query factories. Framework-reserved names remain unchanged. Avoid helpers/common/misc files and giant barrel exports.

## Accessibility Baseline

Provide one clear page heading, semantic landmarks, keyboard navigation, skip links, visible focus and descriptive link/button names. Associate form labels/errors and expose asynchronous status with appropriate live regions. Do not rely solely on color. Browser/media/3D functionality needs text alternatives and user control. Preserve reduced-motion preferences when adding motion.

## Definition of Done

Implement only authorized frontend scope. Preserve existing work. Run lint, strict typecheck, unit/component tests, production build and Playwright discovery; run smoke when supported. Review boundaries, dependencies, route behavior and meaningful test coverage. Inspect git status/diff and confirm no path outside frontend changed. Document real limitations instead of implying integrations exist.

## Things Agents Must Never Do

Never write outside frontend without explicit authorization, create frontend/frontend, edit Go/root/CI files, add alternate lockfiles, overwrite unrelated work, invent protocol/auth/payment/AI contracts, auto-start media, expose tokens, duplicate server state into Zustand, use forms as global state, mark all pages client-side, add empty scaffolding forests, install arbitrary dependencies/system packages, create DESIGN.md or claim unimplemented authentication is secure.
