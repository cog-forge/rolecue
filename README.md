# RoleCue

Go API (`api/`) and Next.js frontend (`frontend/`). See
[frontend setup](frontend/README.md) and [auth setup](auth/README.md) for credentials and local services.

Dev: `make -C api dev` runs PostgreSQL, Caddy, migrations and Go; run `cd auth && bun run dev`
(auth :3001) and `cd frontend && bun run dev` (Next :3002) in other terminals. Open `http://localhost:3000` through Caddy (routing in root `Caddyfile`).

## Development checks

Install Git (Git for Windows on Windows), Python 3.11+ with `venv`, Go 1.27.2+,
Bun 1.4.2 and Node 24.20.0+, with their executables on `PATH`.
Run from the repository root on macOS/Linux:

```sh
python3 .githooks/setup.py
```

On Windows, use PowerShell; WSL and Make are not required to install or run hooks:

```powershell
py .githooks/setup.py
```

`python .githooks/setup.py` also works when Python 3 is available as `python`.
With Make installed, `make setup` calls the same script (`PYTHON=python` can override
the default `python3`). It installs the pinned pre-commit runner, downloads frontend
dependencies with the frozen Bun lockfile and Go module dependencies, and enables
the hook for this Git worktree. Go lint/audit tools are installed when their Make
checks run; `make tooling` can preinstall them. Re-running setup is safe. Existing
custom hooks are preserved; setup stops with instructions if one needs integration.
Setup does not start services, read credentials or apply database migrations.
Git for Windows runs the small Git hook launcher using its bundled shell, including
when committing from PowerShell; setup itself does not invoke a Unix shell.

To check all tracked files without Make, run `py .githooks/setup.py run --all-files`
on Windows or `python3 .githooks/setup.py run --all-files` on macOS/Linux.
The remaining Make targets and Docker smoke scripts use a POSIX shell; the full
application/container CI runs on Linux. Only the hook job runs on all three OSes.

| Command | Purpose |
| --- | --- |
| `make precommit` | Run all staged-file hook checks against all tracked files |
| `make check` | Hooks, Go format/vet/lint/tests/build, frontend format/lint/types/tests/build |
| `make ci` | Above, plus Go race/vulnerability checks and Bun dependency audit |
| `make frontend-audit` | Bun HIGH/CRITICAL audit with the single approved, expiring tooling exception |
| `make -C api test-integration` | Integration tests against an explicitly supplied disposable `DATABASE_URL` |
| `cd frontend && bun run test:e2e` | Playwright suite after a production build and Chromium installation |
| `make image-scan IMAGE=rolecue-api:local` | Trivy HIGH/CRITICAL gate for a locally built image |

The commit hook checks only staged files: Go formatting, frontend Prettier/ESLint,
JSON/YAML syntax, conflict markers, private keys and accidental credential files.
It preserves partially staged work and does not automatically stage or reformat files.
Fix reported formatting with `make -C api fmt` or `cd frontend && bun run format`,
then review and stage the changes yourself.

`make ci` covers checks that need no service. CI additionally runs database integration,
Playwright and image checks. Database tests fail if the supplied database is missing or
unreachable; use a disposable database, never production. Live LLM/S3 tests have separate
build tags and are not part of the credential-free CI suite. Existing Playwright tests
exercise the UI with mocked API responses, not real OAuth/email/payment integrations.

## CI and GHCR

[The CI workflow](.github/workflows/ci.yml) runs on pull requests, pushes to `main`
and manual dispatch. It checks the apps, migrates a temporary PostgreSQL 18 database,
runs Playwright, then builds and smoke-tests both images. Trivy scans the final runtime
images and blocks **any HIGH/CRITICAL finding, including unfixed findings**. JSON reports
and CycloneDX SBOMs are retained for 14 days. Scanner errors also block publication.
Install Trivy 0.76.0 to reproduce the image policy locally; `TRIVY=/path/to/trivy` can
override the executable for `make image-scan`.

The user approved one dependency-audit exception on 2026-10-17:
[`GHSA-vfj7-8cjw-p6xm`](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm), an unpatched
`braces` glob-recursion DoS in ESLint/shadcn tooling. It expires after **2026-11-17 UTC**;
the Make target automatically returns to the full audit then. The standalone frontend
image excludes this package. This exception does not apply to Trivy or any other CVE.

Only a successful main push/manual run publishes images. Pull requests never publish.
The publish job loads the exact images that passed scanning; it does not rebuild them.
The stable aggregate check name is **CI checks**; require it in your `main` branch rule
to enforce checks before merging. This change does not alter repository branch rules.

- `ghcr.io/cog-forge/rolecue-api`
- `ghcr.io/cog-forge/rolecue-frontend`

Both use `latest` and a unique `sha-<commit>-<run-id>-<attempt>` tag. Digests are recorded
in the workflow summary. Prefer the unique tag/digest when selecting a version.
GHCR authentication uses the workflow's `GITHUB_TOKEN`; no personal access token is
needed. An organization/package policy can still restrict publishing. Packages may
start private; configure their access in GitHub Packages before pulling elsewhere.

Set repository Actions Secret **NEXT_PUBLIC_API_BASE_URL** to the desired public API
URL. Its current accepted value is `http://localhost:8080`. It is embedded into the
frontend bundle at build time and is visible to browsers. To change it, update the
Secret and start a **new CI workflow run on main**; deploy the resulting new image
separately. Retrying only the publish job reuses the old artifacts. Fork PR checks use
a test URL and do not need this Secret. Never put auth/DB credentials in build arguments.

## Container builds

```sh
docker build -t rolecue-api:local api
docker build --build-arg NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 \
  -t rolecue-frontend:local frontend
```

The images target Linux amd64 and run as non-root. The frontend uses Next standalone
with a pinned minimal Chainguard Node runtime. The API uses a pinned Wolfi base with
Poppler, Blender 5.2.2 and VRM extension 4.10.2; downloaded Blender/VRM artifacts are
checksum-verified and Blender's embedded Python libraries receive pinned security
updates. Keep base digests and these versions current as new advisories appear.
Trivy covers detected OS/language packages; it cannot prove that every library embedded
inside a downloaded native binary is free of vulnerabilities.

Provide runtime credentials through container environment variables using the existing
root/frontend `.env.example` contracts. API startup connects to its configured DB; it
does not run migrations. Avatar conversion still requires its enable flag and explicit
license/commercial-usage settings. For a local smoke test using an isolated database:

```sh
DATABASE_URL=postgres://ci:ci-only@localhost:55432/rolecue_ci?sslmode=disable \
  sh .docker/smoke.sh api rolecue-api:local
sh .docker/smoke.sh frontend rolecue-frontend:local
```

These smoke tests use Linux host networking and temporary ports 18080/13000. They
clean up only the containers they create. This task publishes build artifacts to GHCR;
it does not deploy, SSH to servers or roll out containers.

## Recruiter Question Bank delivery

Apply migration `000013_posting_question_bank_delivery` before running the updated API.
`QUESTION_BANK_SIZE` controls generation (50 in `.env.example`); the frontend uses the
server's value. There is one shared question-generation service for posting and practice JDs.

The Recruiter flow is: save the reviewed requirements and optional `refinement_note` →
`POST /job-postings/:id/questions/generate` → review/edit the generated questions →
`POST /job-postings/:id/submit` (or `/resubmit` after rejection). Submit and resubmit take no
body. Generating the bank does not submit the posting: a pending posting reaches the Admin
queue only after an explicit submit. Failed generation can be retried on the same posting; a
second generate on an existing bank returns `409 QUESTION_BANK_EXISTS`.

Submission needs a non-empty bank (`409 QUESTION_BANK_NOT_READY` otherwise), and Admins can
approve or reject only submitted postings. Deleting the final question withdraws the posting
from the queue; it must be submitted again. Questions can be added by hand only after the bank
has been generated. A rejected posting can regenerate and edit its bank before resubmitting.

Migration backfill keeps existing pending postings that have questions in the Admin queue;
pending postings with an empty bank go back to preparation.
