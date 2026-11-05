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

## KAN-301

Container hardening (also covers the duplicate KAN-269).

Digest pinning. Every `FROM` in `api`, `auth` and `frontend` (production and
`.dev` Dockerfiles), every third-party `image:` in `.docker/compose.deploy.yaml`
and `.docker/compose.local.yaml`, and the `postgres` service image and the
`mcr.microsoft.com/playwright` job container in `ci.yml` now carry an
`@sha256:` digest next to the human-readable tag. The digests are the manifest
list (multi-arch index) digests resolved with `docker buildx imagetools inspect`
on 2026-11-05. The oven/bun stage copied into `auth/Dockerfile` and
`frontend/Dockerfile` is pinned too. Digests only change when someone edits
them; the Dependabot `docker` and `github-actions` ecosystems added in KAN-303
open the bump PRs. The Chainguard `wolfi-base` and `node` runtime bases were
already pinned.

Not pinned: the locally built `rolecue-*-dev:local` images in `compose.local.yaml`
(built from the repository), and the application images in
`compose.deploy.yaml`, which are referenced by an immutable run-specific tag
(`sha-<commit>-<run>-<attempt>`); KAN-303 additionally verifies their signature.

Blender Python patches. `api/blender-requirements.txt` hash-locks the packages
installed into Blender's bundled Python and `api/Dockerfile` installs them with
`--require-hashes --no-deps --only-binary=:all:`. Hashes are the sha256 of the
`py3-none-any` wheels from the PyPI JSON API. `packaging` is listed explicitly
because `wheel` requires it and `--no-deps` skips resolution. The image build
itself could not be run here (no daemon), so the first CI build is the real test.

Runtime image slimming. The API and auth runtimes are already multi-stage
Wolfi images that run as uid 10001 with `apk --no-cache`; the Go binary, Bun
binary and production `node_modules` are the only application payload. Further
trimming (stripping Blender's bundled docs/Python test files, removing `apk`)
was not done: the Blender runtime contract `RUN` and `smoke.sh` depend on the
exact Blender tree, and the savings are not worth an untestable change.

Compose hardening (`compose.deploy.yaml`). All services get
`security_opt: no-new-privileges:true` and `cap_drop: [ALL]`, plus
`pids_limit` and `mem_limit` ceilings (postgres 2g, api 3g because of Blender,
frontend 768m, auth 512m, caddy and migrate 256m; tune per host). Capabilities
added back: postgres `CHOWN DAC_OVERRIDE FOWNER SETGID SETUID` (entrypoint fixes
PGDATA ownership and drops privileges), caddy `NET_BIND_SERVICE`. `read_only`
with tmpfs is on for postgres, migrate, auth, frontend and caddy. The api is not
read-only because Blender writes under `/home/rolecue`, which also holds the VRM
extension a tmpfs would hide.

Network. `rolecue-private` is intentionally not `internal: true`: api and auth
need outbound internet (Resend, PayOS, LLM, S3, Azure Speech, OAuth providers)
and caddy needs it for ACME. Only caddy publishes host ports. Use the host
firewall for egress filtering if required.

Needs validation on the production host (could not be run here): with
`read_only` and `cap_drop` a service that writes somewhere unexpected will
crash-loop. Roll out on staging first and check `docker logs` for
`Read-only file system` or `Operation not permitted`.

Time-sensitive waiver. The Makefile `frontend-audit` target ignores
`GHSA-vfj7-8cjw-p6xm` (braces, a tooling-only exception) via
`bun audit --ignore` while `date -u +%Y%m%d` is at most `20261117`. After
2026-11-17 UTC the audit fails until the dependency is upgraded or the waiver
is renewed with a written reason. Re-check the advisory before that date
instead of extending it by default.

## KAN-303

Supply-chain controls in `.github/dependabot.yml` and `.github/workflows/ci.yml`.

- Dependabot: weekly updates for `gomod` (/api), `bun` (/frontend, /auth),
  `docker` (each of /api, /auth, /frontend, which also bumps the `@sha256`
  digests), `docker-compose` (/.docker) and `github-actions` (/). Minor and
  patch bumps are grouped to limit PR noise. Needs a repository setting:
  Settings > Code security > enable Dependabot version updates (and security
  updates / alerts). The `bun` ecosystem requires a Dependabot version that
  supports it; if the config shows an error in the Dependabot tab, that is where
  to look.
- Trivy: the `images` job adds a filesystem scan of each component's
  Dockerfile and build context and an image scan, both with
  `scanners: misconfig,secret` and the existing `HIGH,CRITICAL` policy, exit code
  1. The vulnerability gate (`make image-scan`) is unchanged. No `.trivyignore`
  was added because no finding could be observed locally (no Trivy or Docker
  daemon); add narrowly scoped, justified entries if the first CI run reports
  false positives.
- Testing the misconfig gate (do not commit): on a throwaway branch append
  `USER root` as the last line of `api/Dockerfile` and open a PR. The
  "Scan Dockerfile and build context" step fails with DS-0002 (HIGH). To test the
  secret gate add a file containing an AWS-style key to `frontend/`.
- Signing and SBOM: the `publish` job (the only job with `id-token: write`)
  signs each pushed image by digest with keyless cosign and attaches the
  CycloneDX SBOM from the `images` job as a signed attestation
  (`cosign attest --type cyclonedx`). The SBOMs are also uploaded as the
  `sbom-release` artifact with 90-day retention (the per-run scan artifacts
  keep 14 days). This also closes the SBOM-retention item of KAN-307.
- Deploy: `publish` now exports the pushed digests. The `deploy` job runs
  `cosign verify` for each image digest against the identity
  `https://github.com/cog-forge/rolecue/.github/workflows/ci.yml@refs/heads/main`
  (issuer `https://token.actions.githubusercontent.com`) before pulling, and
  passes `API_IMAGE_DIGEST`, `AUTH_IMAGE_DIGEST` and `FRONTEND_IMAGE_DIGEST` to
  `compose.deploy.yaml`, which renders `repo:tag@sha256:...` when they are set
  (unset keeps the tag-only behaviour for the manual `make deploy-*` path).
  The self-hosted runner needs outbound access to the Sigstore public-good
  infrastructure (fulcio, rekor, TUF) and to github.com to install cosign.
- Not validated here: cosign signing and verification, the Trivy steps and
  Dependabot parsing all run only on GitHub; the workflow YAML was parsed
  locally but not executed. If the repository is renamed, update the
  certificate identity in the deploy job.

## KAN-302

Deploy secret exposure. No secret was added, removed or renamed.

Secret flow. GitHub environment `production` (secrets and vars) is read by the
`deploy` job on the self-hosted `rolecue-production` runner. They are now set as
step-level `env` on the single "Pull promoted images and deploy" step instead of
the whole job, so checkout, registry login, cosign installation and signature
verification (and any action they run) never see them. `docker compose`
interpolates every `${VAR:?...}` in the file, even for `pull`, so the full set
must be present for that one step. Inside the host, compose passes each value to
the container that needs it as a container environment variable (visible to
`docker inspect` for anyone in the docker group; this is unchanged and is the
reason the runner host must be treated as trusted for all production secrets).

Migrate. `DATABASE_URL` is no longer in the `migrate` command. The service now
uses `entrypoint: ["/bin/sh","-c"]` with `exec migrate ... -database "$DATABASE_URL" up`
and receives the URL as an environment variable, so it is absent from
`docker compose config`, the container's `Config.Cmd` and `docker ps`/`docker
inspect` command output. Limitation: golang-migrate offers no environment or file
form of `-database`, so the running `migrate` process still has the URL in its
argv (visible to `docker top` / `/proc` for host root and the docker group only).
The container is short-lived, read-only and has no capabilities. To remove it
completely, build the URL without the password and pass `PGUSER`/`PGPASSWORD`
(lib/pq reads them); that changes how `DATABASE_URL` is composed, so it is left
as a follow-up. If the Dockerfile for `migrate/migrate` ever loses `/bin/sh`
this entrypoint breaks; the image is digest-pinned, so that can only happen on a
deliberate bump.

Recommended GitHub settings (cannot be set from the repository; an admin must
configure them):

- Environment `production`: Required reviewers (at least one person who is not
  the pusher, with "Prevent self-review" on); Deployment branches and tags limited
  to `main` (selected branches, not "all"); keep "Allow administrators to bypass"
  off; move the production secrets out of repository-level secrets so only this
  environment holds them.
- Fork pull requests: workflows triggered by a fork PR run without repository or
  environment secrets and with a read-only `GITHUB_TOKEN`, and `deploy` is gated
  on `refs/heads/main` push/dispatch anyway. In Settings > Actions > General set
  "Fork pull request workflows" to require approval for all outside
  collaborators, and keep "Send write tokens to workflows from fork pull requests"
  and "Send secrets to workflows from fork pull requests" off.
- Self-hosted runner: dedicated host for `rolecue-production`, not shared with
  other repositories, ephemeral or at least cleaned between runs
  (`_work` can retain checked-out files), and restricted to this repository in
  its runner group so a pull-request workflow cannot target it.
- Note the NEXT_PUBLIC_* build secrets used in `images`/`e2e` are public values by
  design (they are embedded in the browser bundle), so they are not a concern here.

## KAN-307

Low-severity CI and Docker items.

- `claude.yml` allowlist (`git diff/log/show` accept `--output`): fixed by
  KAN-254, see that section.
- `claude.yml` checks out the PR head with the OAuth token: fixed by KAN-254
  (review events are limited to same-repository PRs; the action also restores
  `.claude/`, `.mcp.json` and `CLAUDE.md` from the base branch).
- SBOM retention: fixed by KAN-303 (signed attestation on the image plus a 90-day
  `sbom-release` artifact).
- Development defaults baked into the API runtime image (`configs/config.yaml`
  has `environment: development` and `debug: true`): `config.yaml` is not
  edited here. Production behaviour is selected at run time by
  `APP_ENVIRONMENT=production`, `APP_DEBUG=false` and the other overrides in
  `compose.deploy.yaml`; the API lane owns tightening the defaults in code.
  `ENV APP_ENVIRONMENT=production` was deliberately not added to
  `api/Dockerfile`: production mode makes configuration validation require
  `AUTH_SECURE_COOKIES=true` and a cookie domain (`internal/config/config.go`),
  and `.docker/smoke.sh` runs the image on plain HTTP without them, so the smoke
  test would fail. Accepted: compose already sets the variable for every real
  deployment. Revisit once smoke.sh passes production-valid cookie settings.
- Dev Dockerfiles (`api`, `auth`, `frontend`) ran as root and `api` made the
  whole `/go` tree world-writable. Now: each image ends with a non-root `USER`
  (`dev`, `bun`, `node`, all uid 1000), the API tool binaries in `/go/bin`
  are read-only for non-root users and only the module cache `/go/pkg` stays
  writable, and `/app` plus `node_modules` are owned by the dev user. Tradeoff:
  `compose.local.yaml` already runs the containers as `LOCAL_UID:LOCAL_GID`
  (default 1000) over bind mounts, which overrides `USER`; developers with another
  uid still need a writable `node_modules` named volume, so that directory
  keeps mode `1777` (sticky, so users cannot remove each other's files) rather
  than being tightened to 755. Not verified with a build (no Docker daemon).

## KAN-295 Object storage

- `S3_ENDPOINT` must be `https://`; plain `http://` is accepted only for loopback hosts (`localhost`,
  `*.localhost`, `127.0.0.0/8`, `::1`). Startup fails otherwise.
- Every object the API writes (`PutObject`) and every presigned upload carries
  `x-amz-server-side-encryption`. `STORAGE_SSE` is `AES256` (default), `aws:kms` (optional
  `STORAGE_SSE_KMS_KEY_ID`; empty uses the bucket default key) or `none` (loopback development only).
  The presigned grant returns the header in `headers`, signed into the URL, and both browser uploaders
  (`putRecording`, `uploadPDF`) already send the grant's headers unchanged.
- **Bucket-default verification (manual, per environment).** The header only covers writes through the
  API. Confirm the bucket itself also encrypts, so a stray write cannot land in clear text:
  `aws s3api get-bucket-encryption --bucket "$S3_BUCKET" --endpoint-url "$S3_ENDPOINT"` must show a
  `ServerSideEncryptionConfiguration` rule (AES256 or aws:kms). If the provider has no bucket default
  (some S3-compatible stores return `ServerSideEncryptionConfigurationNotFoundError`), enable it in the
  provider console or accept that only API writes are encrypted. Then upload one object through the app
  and check `aws s3api head-object` reports `ServerSideEncryption`. A provider that rejects the header
  needs `STORAGE_SSE=none`, which is refused for non-loopback endpoints, so raise it with the provider
  instead. Not verified against the real provider from this machine.

## KAN-305 backend cleanups

- **Session cookie `__Host-` prefix: not adopted.** `__Host-` forbids a `Domain` attribute, but
  production requires `BETTER_AUTH_COOKIE_DOMAIN` (the app and auth origins are different subdomains,
  see `auth/src/env.ts`), so the prefix would never apply where it matters. Without a cookie domain
  (local, single-origin HTTPS) Better Auth already names the cookie `__Secure-<prefix>.session_token`.
  Switching the name would also break the Go side, which recognises the session cookie by name
  (`internal/pkg/authcookie`, the session verifier and the admin account transport). The cookie stays
  `HttpOnly`, `Secure` under HTTPS and `SameSite=Lax`. Revisit if the app and auth move to one origin.
- **Sessions.** Password change revokes every other session on the server
  (`forceRevokeOtherSessions`, PR #99), so the client no longer sends `revokeOtherSessions`; the
  dialog only informs. The 14-day sliding lifetime is now `AUTH_SESSION_MAX_AGE_DAYS` (1 to 90,
  default 14); sessions still refresh at most once a day.
- **Profile image.** `PROFILE_IMAGE_HOSTS` (comma separated, `*.example.com` allowed) limits where a
  profile image URL may point. Empty means Google and GitHub avatar hosts plus the storage endpoint host.
  Existing stored images are not rewritten.
- **Recording retention clock.** `applications.decided_at` (migration 000028) is set once when an
  Application becomes approved or rejected and drives the recording purge. Rows decided earlier were
  backfilled from `updated_at`.
- **Practice start ownership oracle.** Already fixed on main (ownership is checked before
  `FindUnfinishedPractice`; covered by `practice_ownership_oracle_integration_test.go`).
- **Company interviewer.** The query already returns only the avatar the posting locked
  (`required_avatar_id`, owned by the posting's recruiter); a regression test now pins that no other
  avatar of the recruiter, and no foreign avatar, is reachable. The grant is the posting's VRM by design.
