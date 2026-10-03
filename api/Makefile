SHELL := /bin/sh
.DEFAULT_GOAL := check

GOLANGCI_VERSION := v2.13.2
GOLANGCI_COMMIT := 27774aaf853a4fd21f1dd5e69439459dc1b26e68
GOVULNCHECK_VERSION := v1.8.0
SWAG_VERSION := v1.16.6
AIR_VERSION := v1.67.4
SQLC_VERSION := v1.31.1
MIGRATE_VERSION := v4.19.0
TOOLS_DIR := $(CURDIR)/.tools
GOLANGCI := $(TOOLS_DIR)/golangci-lint-$(GOLANGCI_VERSION)/golangci-lint
GOVULNCHECK := $(TOOLS_DIR)/govulncheck-$(GOVULNCHECK_VERSION)/govulncheck
SWAG := $(TOOLS_DIR)/swag-$(SWAG_VERSION)/swag
AIR := $(TOOLS_DIR)/air-$(AIR_VERSION)/air
SQLC := $(TOOLS_DIR)/sqlc-$(SQLC_VERSION)/sqlc
MIGRATE := $(TOOLS_DIR)/migrate-$(MIGRATE_VERSION)/migrate
ROOT_DIR := $(abspath ..)
ENV_FILE := $(ROOT_DIR)/.env
COMPOSE_FILE := $(ROOT_DIR)/.docker/compose.local.yaml
COMPOSE_ARGS := -f "$(COMPOSE_FILE)" -f "$(CURDIR)/compose.local.override.yaml"
MIGRATION_DIR := $(CURDIR)/migration
GO_FILES := $(shell find . -type d \( -name .tools -o -name tmp -o -name vendor \) -prune -o -type f -name '*.go' -print)

.PHONY: fmt fmt-check vet test test-race test-integration swagger build lint vuln check ensure-env db-up migrate migrate-force migrate-version run dev sqlc

ensure-env:
	@test -f "$(ENV_FILE)" || { printf '%s\n' "Missing $(ENV_FILE); copy ../.env.example to ../.env and set local values."; exit 1; }

db-up: ensure-env
	docker compose -p rolecue-authentication --env-file "$(ENV_FILE)" $(COMPOSE_ARGS) up -d --wait

# Applies checked-in migrations, then starts the API with the root env contract.
run: migrate
	@set -a; . "$(ENV_FILE)"; : "$${SERVER_PORT:=8080}"; export SERVER_PORT; set +a; go run ./cmd/api

# Like run, with local hot reload. Air is installed under api/.tools on first use.
dev: migrate $(AIR) $(SQLC)
	@set -a; . "$(ENV_FILE)"; : "$${SERVER_PORT:=8080}"; export SERVER_PORT; export PATH="$(dir $(SQLC)):$$(go env GOPATH)/bin:$$PATH"; set +a; "$(AIR)" -c .air.toml

migrate: db-up $(MIGRATE)
	@set -eu; set -a; . "$(ENV_FILE)"; set +a; : "$${DATABASE_URL:?DATABASE_URL must be set in $(ENV_FILE)}"; "$(MIGRATE)" -path "$(MIGRATION_DIR)" -database "$$DATABASE_URL" up

migrate-force: db-up $(MIGRATE)
	@set -eu; set -a; . "$(ENV_FILE)"; set +a; : "$${DATABASE_URL:?DATABASE_URL must be set in $(ENV_FILE)}"; : "$${v:?Specify version with v=<version>, e.g. make migrate-force v=2}"; "$(MIGRATE)" -path "$(MIGRATION_DIR)" -database "$$DATABASE_URL" force "$$v"

migrate-version: db-up $(MIGRATE)
	@set -eu; set -a; . "$(ENV_FILE)"; set +a; : "$${DATABASE_URL:?DATABASE_URL must be set in $(ENV_FILE)}"; "$(MIGRATE)" -path "$(MIGRATION_DIR)" -database "$$DATABASE_URL" version

swagger: $(SWAG)
	$(SWAG) init -g main.go -d cmd/api,internal/handler,internal/features/auth,internal/features/jd/domain,pkg/response,pkg/apperror --parseInternal -o docs

test-integration:
	go test -tags=integration -count=1 -timeout=5m ./internal/...

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@set -eu; files="$$(gofmt -l $(GO_FILES))"; if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

build:
	go build ./...

lint: $(GOLANGCI)
	$(GOLANGCI) run ./...

vuln: $(GOVULNCHECK)
	$(GOVULNCHECK) ./...

# Fast feedback; run test-race and vuln separately (as CI does).
check: fmt-check vet test lint build

$(GOLANGCI):
	@mkdir -p "$(dir $(GOLANGCI))"
	@set -eu; installer=$$(mktemp); trap 'rm -f "$$installer"' EXIT; \
		curl --fail --silent --show-error --location \
		"https://raw.githubusercontent.com/golangci/golangci-lint/$(GOLANGCI_COMMIT)/install.sh" -o "$$installer"; \
		sh "$$installer" -b "$(dir $(GOLANGCI))" $(GOLANGCI_VERSION)

$(GOVULNCHECK):
	GOBIN="$(dir $(GOVULNCHECK))" go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

$(SWAG):
	GOBIN="$(dir $(SWAG))" go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

$(AIR):
	GOBIN="$(dir $(AIR))" go install github.com/air-verse/air@$(AIR_VERSION)

sqlc: $(SQLC)
	"$(SQLC)" generate

$(SQLC):
	GOBIN="$(dir $(SQLC))" go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)

$(MIGRATE):
	GOBIN="$(dir $(MIGRATE))" go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)
