SHELL := /bin/sh
.DEFAULT_GOAL := check

PYTHON ?= python3
TRIVY ?= trivy

.PHONY: setup tooling precommit check frontend-check frontend-audit ci image-scan image-build ensure-local-env dev dev-api dev-frontend dev-down deploy-pull deploy-up deploy-down

DEPLOY_ENV_FILE ?= .docker/deploy.env
DEPLOY_COMPOSE := --env-file "$(DEPLOY_ENV_FILE)" -p rolecue -f .docker/compose.deploy.yaml
LOCAL_COMPOSE := --env-file .env -p rolecue -f .docker/compose.local.yaml

setup:
	$(PYTHON) .githooks/setup.py

tooling: setup
	$(MAKE) -C api tools

precommit:
	$(PYTHON) .githooks/setup.py run --all-files

frontend-check:
	cd frontend && bun run format:check && bun run lint && bun run typecheck && bun test && bun run build

ensure-local-env:
	@test -f .env || { echo 'Copy .env.example to .env and set local values first.'; exit 1; }
	@test -f auth/.env || { echo 'Copy auth/.env.example to auth/.env first.'; exit 1; }

dev-api: ensure-local-env
	LOCAL_UID="$$(id -u)" LOCAL_GID="$$(id -g)" docker compose $(LOCAL_COMPOSE) up --build api

dev-frontend: ensure-local-env
	LOCAL_UID="$$(id -u)" LOCAL_GID="$$(id -g)" docker compose $(LOCAL_COMPOSE) up --build api frontend caddy

dev: dev-frontend

dev-down: ensure-local-env
	docker compose $(LOCAL_COMPOSE) down --remove-orphans

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
