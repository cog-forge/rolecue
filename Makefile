SHELL := /bin/sh
.DEFAULT_GOAL := check

PRE_COMMIT := $(CURDIR)/.tools/pre-commit/bin/pre-commit
PRE_COMMIT_VERSION := 4.7.0
TRIVY ?= trivy

.PHONY: setup tooling precommit check frontend-check frontend-audit ci image-scan

setup: tooling
	sh .githooks/install.sh

tooling: $(PRE_COMMIT)
	bun --no-env-file install --cwd frontend --frozen-lockfile --ignore-scripts
	cd api && go mod download
	$(MAKE) -C api tools

$(PRE_COMMIT):
	python3 -m venv .tools/pre-commit
	.tools/pre-commit/bin/python -m pip install 'pre-commit==$(PRE_COMMIT_VERSION)'

precommit: $(PRE_COMMIT)
	$(PRE_COMMIT) run --all-files

frontend-check:
	cd frontend && bun run format:check && bun run lint && bun run typecheck && bun test && bun run build

check: precommit
	$(MAKE) -C api check
	$(MAKE) frontend-check

ci: check
	$(MAKE) -C api test-race vuln
	$(MAKE) frontend-audit

# User-approved, tooling-only exception. It expires after 2026-11-17 UTC.
# No container vulnerability is excluded by this target.
frontend-audit:
	cd frontend && if [ "$$(date -u +%Y%m%d)" -le 20261117 ]; then \
		echo 'Temporary braces exception: GHSA-vfj7-8cjw-p6xm, expires 2026-11-17 UTC'; \
		bun audit --audit-level=high --ignore GHSA-vfj7-8cjw-p6xm; \
	else bun audit --audit-level=high; fi

# IMAGE must name an image already built/loaded into the local Docker daemon.
image-scan:
	@test -n "$(IMAGE)" || { echo 'Usage: make image-scan IMAGE=rolecue-api:local'; exit 1; }
	$(TRIVY) image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 "$(IMAGE)"
