# EduWorld Makefile (F001 T005/T046, LOCAL_DEVELOPMENT §7). Targets are thin wrappers so the same
# commands run in CI (GitHub Actions) and on a developer machine. Requires GNU make + bash.
#
# Source layout (ADR-0092): src/backend (the single Go module: cmd/, modules/, pkg/), src/cms/frontend
# (Admin Portal), src/game (Godot). deploy/, specs/, docs/, scripts/, design/ stay at the root.
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

include tools/versions.mk

BACKEND  := src/backend
GO       := go -C $(BACKEND)
GOPKG    := github.com/tanguyen2020/eduworld-platform/src/backend
MODULES  := identity learning game competition chat
BINARIES := core-api realtime worker
COMPOSE_DIR := deploy/compose
DC       := docker compose -f $(COMPOSE_DIR)/docker-compose.yml -f $(COMPOSE_DIR)/docker-compose.app.yml -f $(COMPOSE_DIR)/docker-compose.observability.yml
GOBIN    := $(CURDIR)/bin
export GOBIN
export PATH := $(GOBIN):$(PATH)

# Build metadata (F001-AC-19): one version for the three binaries, from tag backend/vX.Y.Z or git describe.
EW_VERSION    ?= $(shell git describe --tags --match 'backend/v*' --always 2>/dev/null | sed 's#^backend/##' || echo dev)
EW_COMMIT     ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
EW_BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
export EW_VERSION EW_COMMIT EW_BUILD_TIME
LDFLAGS = -X $(GOPKG)/pkg/buildinfo.Version=$(EW_VERSION) \
          -X $(GOPKG)/pkg/buildinfo.Commit=$(EW_COMMIT) \
          -X $(GOPKG)/pkg/buildinfo.BuildTime=$(EW_BUILD_TIME)

# Local DSNs for native runs (containers get theirs from docker-compose.app.yml).
PW ?= change-me-local
define dsn
postgres://$(1)_$(2):$(PW)@localhost:5432/eduworld_$(1)?sslmode=disable
endef
NATIVE_ENV = EW_VALKEY_ADDR=localhost:6379 EW_NATS_URLS=nats://localhost:4222 EW_LOG_FORMAT=text \
  $(foreach m,$(MODULES),EW_DB_$(shell echo $(m) | tr a-z A-Z)_DSN=$(call dsn,$(m),app))
MIGRATOR_ENV = $(foreach m,$(MODULES),EW_DB_$(shell echo $(m) | tr a-z A-Z)_MIGRATOR_DSN=$(call dsn,$(m),migrator))
# Atlas runs in the compose network and reaches PostgreSQL as "postgres" (no host networking on Docker Desktop).
ATLAS_RUN = docker run --rm --network eduworld_default -v "$(CURDIR)/$(BACKEND)/modules:/modules" $(ATLAS_IMAGE)
ATLAS_DEV_URL ?= postgres://postgres:$(PW)@postgres:5432/atlas_dev?sslmode=disable

.PHONY: help
help: ## List targets
	@grep -hE '^[a-zA-Z0-9_%-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-22s %s\n", $$1, $$2}'

# ---------- tools / env ----------
.PHONY: tools env
tools: ## Install pinned Go tools into ./bin and pull the Atlas image
	bash scripts/dev/install-tools.sh

env: ## Copy every .env.example to .env (root, backend modules) if missing
	@for f in .env.example $(BACKEND)/modules/*/.env.example; do [ -f "$$f" ] || continue; t="$${f%.example}"; [ -f "$$t" ] || cp "$$f" "$$t"; done

# ---------- infrastructure ----------
.PHONY: up down reset up-obs
up: ## Start infrastructure (postgres valkey nats keycloak apisix mailpit floci s3-init cdn sis-mock)
	bash scripts/dev/check-docker.sh
	$(DC) up -d --wait postgres valkey nats keycloak apisix mailpit floci cdn sis-mock
	$(DC) up s3-init

down: ## Stop every container
	$(DC) --profile app --profile obs --profile media --profile tools --profile migrate down

reset: ## Stop and delete volumes and ./data (local only)
	$(DC) --profile app --profile obs --profile media --profile tools --profile migrate down -v
	rm -rf data/floci data/backups

up-obs: ## Start observability (otel-collector, Prometheus, Grafana, Loki, Tempo)
	$(DC) --profile obs up -d --wait

# ---------- binaries ----------
.PHONY: up-app run-core-api run-realtime run-worker dev build
EXCEPT ?=
up-app: ## Run core-api, realtime, worker in containers (EXCEPT=<binary> to skip one)
	$(DC) --profile app up -d --build --wait $(filter-out $(EXCEPT),$(BINARIES))

run-core-api: ## Run core-api natively (admin :9100, http :8100)
	$(NATIVE_ENV) $(GO) run -ldflags "$(LDFLAGS) -X $(GOPKG)/pkg/buildinfo.Binary=core-api" ./cmd/core-api

run-realtime: ## Run realtime natively (admin :9106, ws :8106)
	$(NATIVE_ENV) $(GO) run -ldflags "$(LDFLAGS) -X $(GOPKG)/pkg/buildinfo.Binary=realtime" ./cmd/realtime

PROFILE ?= all
run-worker: ## Run worker natively (PROFILE=events|chat|jobs|all, admin :9200)
	$(NATIVE_ENV) $(GO) run -ldflags "$(LDFLAGS) -X $(GOPKG)/pkg/buildinfo.Binary=worker" ./cmd/worker --profile=$(PROFILE)

dev: ## Run the three binaries natively (infra in containers)
	trap 'kill 0' EXIT; $(MAKE) run-core-api & $(MAKE) run-realtime & $(MAKE) run-worker PROFILE=all & wait

build: ## Build every binary into ./bin with build metadata
	@for b in $(BINARIES) migrate; do \
	  CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS) -X $(GOPKG)/pkg/buildinfo.Binary=$$b" -o $(GOBIN)/$$b ./cmd/$$b; done

# ---------- database ----------
.PHONY: migrate migrate-diff migrate-lint db-privileges db-backup db-restore
migrate: $(addprefix migrate-,$(MODULES)) ## Apply Atlas migrations to every module database

migrate-%: ## Apply Atlas migrations of one module (role <module>_migrator)
	$(MIGRATOR_ENV) $(GO) run ./cmd/migrate --module=$* --atlas="bash $(CURDIR)/scripts/dev/atlas.sh"

name ?= change
migrate-diff: ## Generate a migration from ent schema: MOD=<module> name=<x>
	$(ATLAS_RUN) migrate diff $(name) --dir file:///modules/$(MOD)/migrations --to ent:///modules/$(MOD)/ent/schema --dev-url "$(ATLAS_DEV_URL)"

migrate-lint: ## Validate migrations of MOD (or every module): atlas.sum integrity and SQL replay on a dev database
	@for m in $(or $(MOD),$(MODULES)); do \
	  $(ATLAS_RUN) migrate validate --dir file:///modules/$$m/migrations --dev-url "$(ATLAS_DEV_URL)"; done

db-privileges: ## Fail if an app role has DELETE/TRUNCATE or cross-database CONNECT (F001-AC-11)
	bash scripts/ci/db-privileges.sh

MOD ?=
FILE ?=
db-backup: ## Dump one module database: MOD=<module>
	bash scripts/ops/db-backup.sh $(MOD)

db-restore: ## Restore one module database locally: MOD=<module> FILE=<dump>
	bash scripts/ops/db-restore.sh $(MOD) $(FILE)

# ---------- code generation / checks ----------
.PHONY: gen gen-check arch-check lint test test-integration test-contract
gen: ## Generate code: architecture rules from x-allowed-callers, problem codes from ProblemCode (later: ent, oapi-codegen)
	$(GO) run ./tools/archgen
	$(GO) generate ./pkg/problem
	$(GO) run ./tools/allowedcallers
	$(GO) run ./tools/routematrix gen
	$(GO) run ./tools/infragen
	cd $(BACKEND)/pkg/testkit/sample/ent && go generate .

gen-check: gen ## Fail if generated files differ from the committed ones
	git diff --exit-code -- $(BACKEND)/.golangci.arch.yml $(BACKEND)/pkg/problem/codes_gen.go
	$(GO) run ./tools/allowedcallers --check
	$(GO) run ./tools/infragen --check
	git diff --exit-code -- $(BACKEND)/pkg/testkit/sample/ent $(BACKEND)/modules/*/migrations

arch-check: ## Module boundaries: Go internal/, x-allowed-callers, pkg/ and cmd/ rules, layers (ADR-0074)
	$(GO) build ./...
	$(GO) run ./tools/archgen --check
	$(GO) run ./tools/lint/ewlint ./... # bypassreason + scopeguard (ADR-0089 §3)
	$(GO) run ./tools/routematrix check # every implemented operation has tenant A/B + role tests (F002-AC-28)

lint: ## gofmt, go vet, golangci-lint + generated depguard rules (MOD=<module> to scope)
	test -z "$$(gofmt -l $(BACKEND)/$(if $(MOD),modules/$(MOD)))"
	$(GO) vet -tags integration ./...
	$(GO) run ./tools/lint/ewlint $(if $(MOD),./modules/$(MOD)/...,./...)
	cd $(BACKEND) && golangci-lint run $(if $(MOD),./modules/$(MOD)/...,./...)
	cd $(BACKEND) && golangci-lint run --config .golangci.arch.yml $(if $(MOD),./modules/$(MOD)/...,./...)

# -race needs cgo (always on in CI/Linux; often off on Windows without gcc).
RACE := $(if $(filter 1,$(shell go env CGO_ENABLED)),-race,)
test: ## Unit tests (MOD=<module> to scope; -race when cgo is available)
	$(GO) test $(RACE) -count=1 $(if $(MOD),./modules/$(MOD)/...,./...)

test-integration: ## Integration tests with testcontainers (Docker required)
	$(GO) test -count=1 -tags integration $(if $(MOD),./modules/$(MOD)/...,./...)

test-contract: ## Facade tests between modules (added with the first facade operation)
	$(GO) test -count=1 -run Facade ./modules/...

# ---------- specs / docs ----------
.PHONY: specs-lint specs-breaking docs-check
specs-lint: ## Contract checks (Spectral/AsyncAPI added with the spec toolchain)
	npm ci --prefix scripts --silent
	node scripts/specs/check-features.mjs
	node scripts/specs/check-cross-features.mjs
	node scripts/specs/check-portal-catalog.mjs
	node scripts/specs/check-problem-codes.mjs
	node scripts/specs/check-maxlength.mjs
	node scripts/specs/check-enums.mjs
	node scripts/specs/validate-schemas.mjs
	node scripts/openapi/split-public.mjs --check
	node scripts/openapi/sync-platform-ops.mjs --check

BASE_REF ?= origin/main
specs-breaking: ## oasdiff: breaking changes of public + internal OpenAPI vs $(BASE_REF)
	bash scripts/ci/specs-breaking.sh $(BASE_REF)

docs-check: ## Mermaid, links, traceability matrix
	npm ci --prefix scripts --silent
	node scripts/docs/check-mermaid.mjs
	node scripts/docs/check-links.mjs
	node scripts/docs/build-rtm.mjs --check

.PHONY: ci-local
ci-local: ## CI checks that need no GitHub: workflows, path filters, admin ports, arch fixtures, scanners
	node scripts/ci/check-workflows.mjs
	node --test scripts/ci/check-workflows.test.mjs
	bash scripts/ci/test-path-filters.sh
	bash scripts/ci/check-admin-ports.sh
	bash scripts/ci/test-arch-fixtures.sh
	bash scripts/ci/test-gitleaks.sh
	bash scripts/ci/test-vuln-scan.sh
