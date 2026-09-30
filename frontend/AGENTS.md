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

- The canonical RoleCue design repository is:
  `https://github.com/cog-forge/role-cue-design`.

- Before every material UI implementation or visual change, read the current:
  `DESIGN.md`
  from the canonical design repository.

- A local sibling or cached checkout may be used when available for speed, but it is only a convenience. Production implementation must never depend on a local sibling path existing.

- Relevant design artifacts should be inspected from the canonical design repository, especially:
  - `brand/`
  - `references/open-design/`
  - `references/open-design/source-system/`
  - `screens/`
  - `flows/`
  - the relevant current screen notes/inventory when implementing product screens.

## Visual authority model

- OpenDesign is RoleCue's structural / interaction / editorial reference.
- RoleCue's current brand contract is the branding authority.

OpenDesign defines how the interface is constructed:
- composition
- layout geometry
- typography hierarchy
- spacing rhythm
- material/surface treatment
- border and elevation discipline
- responsive behavior
- interaction patterns
- motion restraint
- editorial presentation grammar

RoleCue defines whose interface it is:
- canonical logo / mark
- canonical blue identity
- product-specific brand expression
- copy and product semantics
- action hierarchy

Use this rule:

**Reuse the grammar, not the branding.**

Do not copy OpenDesign's green/lime identity into RoleCue.

## Current RoleCue visual thesis

- Canonical RoleCue identity is blue.
- Blue is restrained identity/accent, not a full-interface fill color.
- Near-black remains the primary text and primary-action authority.
- Neutral near-white and white surfaces dominate the interface.
- Media/artwork may carry substantially more chroma than interface chrome.

Use this principle:

**Blue identifies RoleCue. Black drives action. Media carries emotion.**

Explicitly avoid:
- blue-purple AI gradients
- neon/glow
- glassmorphism
- cyberpunk styling
- generic AI blobs
- excessive pill UI
- dense card soup
- arbitrary blue decoration everywhere
- recoloring third-party provider brands into RoleCue blue

## OpenDesign source-code references

- It is allowed to inspect or temporarily clone OpenDesign source code when a task explicitly asks to study an existing OpenDesign component or interaction.
- Treat OpenDesign source as reference material, not production source.
- Study geometry, responsive logic, typography, interaction physics, motion, and composition.
- Re-author the implementation using RoleCue's actual frontend architecture:
  Next.js App Router, React, Tailwind CSS v4, shadcn/ui, and Motion where justified.
- Do not copy OpenDesign branding, copywriting, product identity, or source-specific architectural assumptions.
- Do not add OpenDesign as a runtime dependency, submodule, workspace dependency, or production import.
- Temporary/reference clones must live outside the RoleCue repository and must not be committed.

## Brand asset rule

- Use the newest explicitly approved RoleCue brand direction.
- The current product direction is the team-approved blue RoleCue identity.
- Some historical artifacts in the design repository may still contain the older Split Halo/green direction; historical assets do not override the current `DESIGN.md`.
- Do not fabricate, recolor, auto-vectorize, or substitute brand assets.
- Runtime assets must be copied into production-owned `frontend/` paths when authorized by the task.

## Implementation contract

- Approved design/reference output is a visual and interaction contract, not production HTML/CSS/JS.
- Preserve visual grammar, layout, spacing, typography hierarchy, responsive behavior, motion intent, and interaction intent while implementing with the project's real architecture.
- When implementation and an older artifact disagree, the newest explicit RoleCue design contract wins.
- When product behavior is unclear, do not invent it from a reference website.