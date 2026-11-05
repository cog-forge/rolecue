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

Notes attached to the security-hardening pull request. Each section is owned by
the ticket named in its heading.

## KAN-254

Scope: `.github/workflows/claude.yml` (the on-demand `@claude` assistant; the
team has since disabled the review bot, the workflow is kept valid and hardened).

Changes:

- `pull_request_review_comment` and `pull_request_review` runs now require
  `github.event.pull_request.head.repo.full_name == github.repository`. These
  events check out the PR merge ref, so for a fork PR the workspace would hold
  fork code while `CLAUDE_CODE_OAUTH_TOKEN` and `id-token: write` are available.
  Fork PRs are now skipped for review events. `issue_comment` events check out
  the default branch (not PR code), so they need no extra gate.
- `Bash(git diff:*)`, `Bash(git log:*)` and `Bash(git show:*)` were removed from
  the allowlist. They looked read-only, but `--output=<path>` makes them write
  files, and git accepts abbreviated long options (`--out=`), so a deny pattern
  on `--output` alone is not a complete guard. Diff data is available through
  `gh pr diff`, which stays allowed. As defence in depth the deny list also gets
  `Bash(git * --output*)`, `Bash(git * --ext-diff*)`, `Bash(git * --textconv*)`
  and `Bash(git -c:*)` (external diff/textconv programs and config-injected
  commands).

Does the action load workspace settings from the checkout? Yes for the Claude
Code CLI, and the action mitigates it. The CLI trusts its working directory: it
reads `.claude/settings.json`, `.claude/settings.local.json` and `.mcp.json`
before tool-permission gating and acts on hooks (including SessionStart), `env`,
`apiKeyHelper` and MCP server entries. `anthropics/claude-code-action` v1.0.259
(the pinned version) ships `src/github/operations/restore-config.ts`
(`restoreConfigFromBase`), whose own doc comment states that it replaces
`.claude/`, `.mcp.json`, `CLAUDE.md`, `.husky` and similar paths with the PR base
branch versions after a PR checkout. This was read from the action source on
GitHub, not exercised in a live run. Remaining exposure: everything outside those
paths (for example `bunfig.toml`, package scripts, Makefile) still comes from the
checkout, which is why fork code must not be in the workspace at all; the
same-repository gate above provides that for review events. For an
`issue_comment` mention on a fork PR the action itself fetches
`refs/pull/N/head` in tag mode, so keep the mention restricted to
`OWNER`/`MEMBER`/`COLLABORATOR` (already enforced) and do not re-enable the bot
for fork PRs without a separate review.
