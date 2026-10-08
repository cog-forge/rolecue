# Frontend agent instructions

- Work only inside `frontend/` unless the task explicitly authorizes another repository or a temporary external reference checkout.
- Preserve existing work; never initialize `frontend/frontend`.
- Read `SKILL.md` for the detailed frontend engineering playbook.
- Keep App Router pages focused on composition; Server Components are the default.
- Own business/product code inside `features/`; shared UI must not import feature internals.
- TanStack Query owns remote async state; React Hook Form + Zod own forms; Zustand is only for genuine cross-route client state.
- Axios is the single shared browser HTTP client. The browser calls the Go API directly; add a Next.js proxy/BFF only for a concrete requirement.
- Prefer existing shadcn/ui primitives for interactive controls. Recreate Button, Input, Field, Label, Alert, Dialog, Sheet, Tabs, Accordion, or similar primitives only with a documented reason.
- Tailwind CSS v4 is the default styling mechanism. Do not add feature-local CSS for ordinary styling or simple motion.
- Use Motion only when declarative motion materially improves the interaction; prefer normal Tailwind transitions for simple hover/focus/press states.
- Do not add one-line wrapper hooks, adapters, repositories, or services. Extract only for real reuse, behavior, or complexity; keep each feature screen cohesive.
- Unit/component tests live under `tests/unit/`, mirroring source paths. Browser tests live under `tests/e2e/`; never colocate tests with production source.
- Keep the interview state machine pure and browser runtimes behind explicit client boundaries.
- Never invent auth, backend, AI, billing, socket, or lip-sync contracts.
- Better Auth is the selected authentication engine in the product contract, but do not introduce or modify Better Auth integration unless the current task explicitly scopes it.
- Run lint, typecheck, `bun test`, and build; discover Playwright coverage and run relevant smoke/E2E when a browser is available.
- Do not add local DESIGN.md copies, heavy reference media, empty directories, or broad lint/type suppressions.
- Confirm `git status` / diff contains no unrelated changes before handing off.

## Design source of truth

- For UI work, follow the current `DESIGN.md` in the [RoleCue design repository](https://github.com/cog-forge/role-cue-design).

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
