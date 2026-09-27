# Frontend agent instructions

- Work only inside frontend/ unless explicitly authorized otherwise. Preserve existing work; never initialize frontend/frontend.
- Read SKILL.md for the detailed frontend engineering playbook.
- Keep app pages focused on composition; Server Components are the default.
- Own business code inside features; shared UI cannot import features.
- Server state: TanStack Query. Forms: React Hook Form. Navigation: URL. Local UI: React. Cross-route client state: narrowly scoped Zustand only when needed.
- Use the canonical lib/api transport and domain key factories. No arbitrary token reads or competing HTTP clients.
- Keep the interview machine pure and browser runtimes behind client boundaries.
- Never invent auth, backend, AI, billing, socket or lip-sync contracts.
- Run lint, typecheck, bun test and build; discover Playwright tests and run smoke if a browser is installed.
- Do not add DESIGN.md, heavy media, empty directories or broad lint/type suppressions.
- Confirm git status/diff contains no changes outside frontend/ before handing off.

## Design source of truth

- The canonical RoleCue visual/design source is the read-only sibling repository `../../ai-interview-practice-design/` (relative to `frontend/`); begin every material UI change by reading `../../ai-interview-practice-design/DESIGN-CONTRACT.md`.
- Before implementing or materially changing a UI that already has a design artifact, inspect the relevant sibling-repository files under `../../ai-interview-practice-design/brand/`, `../../ai-interview-practice-design/exploration/`, and `../../ai-interview-practice-design/product/`.
- RoleCue brand files under `../../ai-interview-practice-design/brand/` are the canonical branding source. The approved OpenDesign landing/reference artifacts under `../../ai-interview-practice-design/exploration/` are the landing's canonical visual reference.
- Realtime 3D / Blender exploration is currently DEFERRED; do not introduce or invent Three.js / R3F / GLB during landing implementation.
- Approved OpenDesign output is a visual/interaction contract, not production source code.
- Agents should reproduce its visual grammar, layout, spacing, typography, responsive behavior, motion, and interaction intent in the real Next.js architecture. Generated OpenDesign HTML/CSS/JS is never production source code.
- Production implementation must still follow the frontend architecture, component boundaries, accessibility, and existing engineering conventions.
- When implementation and an approved design artifact visibly disagree, do not silently invent a new direction. Treat the approved design artifact as the visual reference unless newer explicit product/design direction supersedes it.
- Runtime assets must be copied into production-owned `frontend/` paths; production code must not use hardcoded sibling-repository paths or otherwise depend on that repository. The frontend must build when the sibling design repository is absent.
- Downloaded, vendor, or cache assets are not design source-of-truth and should not be committed merely because an agent used them during exploration.
