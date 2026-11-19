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
.PHONY: up down reset up-obs up-mock
up: ## Start infrastructure (postgres valkey nats keycloak apisix apisix-internal mailpit floci s3-init cdn sis-mock)
	bash scripts/dev/check-docker.sh
	$(DC) up -d --wait postgres valkey nats keycloak apisix apisix-internal mailpit floci cdn sis-mock
	$(DC) up s3-init

down: ## Stop every container
	$(DC) --profile app --profile obs --profile media --profile tools --profile migrate --profile mock down

reset: ## Stop and delete volumes and ./data (local only)
	$(DC) --profile app --profile obs --profile media --profile tools --profile migrate --profile mock down -v
	rm -rf data/floci data/backups

up-mock: ## Start the labelled Prism MOCK API for the game client on :4011 (profile mock, F060-AC-20; not a backend)
	$(DC) --profile mock up -d --wait api-mock

up-obs: ## Start observability (otel-collector, Prometheus, Grafana, Loki, Tempo)
	$(DC) --profile obs up -d --wait

# ---------- binaries ----------
.PHONY: up-app run-core-api run-realtime run-worker dev build
EXCEPT ?=
up-app: ## Run core-api, realtime, worker in containers (EXCEPT=<binary> to skip one)
	EW_GAME_LOGIN_TICKET_KEY=$${EW_GAME_LOGIN_TICKET_KEY:-$$(head -c 32 /dev/urandom | base64)} \
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
.PHONY: migrate migrate-diff migrate-lint seed db-privileges db-backup db-restore
migrate: $(addprefix migrate-,$(MODULES)) ## Apply Atlas migrations to every module database

migrate-%: ## Apply Atlas migrations of one module (role <module>_migrator)
	$(MIGRATOR_ENV) $(GO) run ./cmd/migrate --module=$* --atlas="bash $(CURDIR)/scripts/dev/atlas.sh"

seed: ## Load the dev seed (module learning: subjects + curriculum GDPT2018_SAMPLE through the import use case, F021)
	EW_DB_LEARNING_DSN=$(call dsn,learning,app) EW_DB_LEARNING_PLATFORM_DSN=$(call dsn,learning,platform) $(GO) run ./modules/learning/cmd/seed

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
	cd $(BACKEND) && go run ./tools/facadegen --oapi=oapi-codegen
	$(GO) run ./tools/routematrix gen
	$(GO) run ./tools/infragen
	$(GO) run ./tools/sisgen
	cd $(BACKEND)/pkg/testkit/sample/ent && go generate .
	cd $(BACKEND)/modules/identity/ent && go generate .

gen-check: gen ## Fail if generated files differ from the committed ones
	git diff --exit-code -- $(BACKEND)/.golangci.arch.yml $(BACKEND)/pkg/problem/codes_gen.go
	$(GO) run ./tools/allowedcallers --check
	cd $(BACKEND) && go run ./tools/facadegen --check --oapi=oapi-codegen
	$(GO) run ./tools/infragen --check
	$(GO) run ./tools/sisgen --check
	git diff --exit-code -- $(BACKEND)/pkg/testkit/sample/ent $(BACKEND)/modules/*/ent $(BACKEND)/modules/*/migrations

arch-check: ## Module boundaries: Go internal/, x-allowed-callers, pkg/ and cmd/ rules, layers (ADR-0074)
	$(GO) build ./...
	$(GO) run ./tools/archgen --check
	$(GO) run ./tools/lint/ewlint ./... # bypassreason + scopeguard (ADR-0089 §3)
	$(GO) run ./tools/routematrix check # every implemented operation has tenant A/B + role tests (F002-AC-28)
	cd $(BACKEND) && go run ./tools/entlint # MaxLen, exactly one entx mixin (F003-AC-07)
	cd $(BACKEND) && go run ./tools/tenancyguard # RLS, explicit SQL, no DELETE/TRUNCATE (F003-AC-07)

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

test-contract: ## Contract tests: public/facade/event helpers, caller-side facade tests (generated api.Fake), module facade tests
	$(GO) test -count=1 ./pkg/testkit/contract/... ./cmd/internal/wiring/...
	$(GO) test -count=1 -run Facade ./modules/...

# ---------- identity provider (F004) ----------
.PHONY: keycloak-import keycloak-check keycloak-test
keycloak-import: ## Apply realm eduworld (keycloak-config-cli + post-apply; local also dev users/organizations)
	bash deploy/keycloak/scripts/apply.sh

keycloak-check: ## Static checks of the realm as code (clients, flows, secrets, settings)
	node deploy/keycloak/scripts/check-realm.mjs
	node --test deploy/keycloak/scripts/check-realm.test.mjs

keycloak-test: ## Realm behaviour against the running Keycloak (direct grants, amr, idempotent apply)
	KC_INTEGRATION=1 node --test deploy/keycloak/tests/realm.integration.test.mjs

.PHONY: apisix-render apisix-check apisix-smoke
apisix-render: ## Render deploy/apisix/standalone/{public,internal}/apisix.yaml from routes.yaml (ENV=local)
	node deploy/apisix/scripts/render.mjs --env=$(or $(ENV),local)

apisix-check: ## Gateway config checks: rendered files fresh, routes vs spec, no body logging, no hardcode
	node deploy/apisix/scripts/render.mjs --check
	node deploy/apisix/scripts/check-routes.mjs
	node deploy/apisix/scripts/check-no-body-logging.mjs
	node deploy/apisix/scripts/check-no-hardcode.mjs
	node --test deploy/apisix/scripts/

apisix-smoke: ## Gateway behaviour against the running stack (public :9080, internal :9180)
	APISIX_SMOKE=1 node --test deploy/apisix/tests/smoke.test.mjs

# ---------- design tokens (F005, ADR-0047) ----------
PNPM ?= pnpm
.PHONY: design-tokens design-tokens-check
design-tokens: ## Generate Godot Theme/UiTokens/3D resources, Ant Design theme + CSS variables from design/ (checks first)
	cd design && $(PNPM) install --frozen-lockfile
	cd design && $(PNPM) build

design-tokens-check: ## Fail if generated theme files drift from design/; aliases, business colors, contrast, sizes, fonts, semver vs $(BASE_REF)
	cd design && $(PNPM) install --frozen-lockfile
	cd design && $(PNPM) test
	cd design && DESIGN_BASE_REF=$(BASE_REF) $(PNPM) check

# ---------- web: Admin Portal + ops-ui (F050, ADR-0022, ADR-0047) ----------
WEB_DIR := src/cms/frontend
.PHONY: web-install web-lint web-test web-build storybook web-storybook-check web-visual portal-catalog
web-install: ## Install the pnpm workspace of src/cms/frontend (frozen lockfile)
	cd $(WEB_DIR) && $(PNPM) install --frozen-lockfile

portal-catalog: web-install ## Generate apps/portal/src/generated/menu-catalog.ts from specs/portal/menu-catalog.yaml (F051, ADR-0090)
	cd $(WEB_DIR) && $(PNPM) gen:catalog

web-lint: web-install ## ESLint + stylelint (no hardcoded style/game import/env URL, i18n), typecheck, gen:check of the API client and the menu catalog
	cd $(WEB_DIR) && $(PNPM) lint && $(PNPM) typecheck && $(PNPM) gen:check

web-test: web-install ## Vitest with one worker: portal (apiClient, routes, i18n), ops-ui, ESLint rules
	cd $(WEB_DIR) && $(PNPM) test

web-build: web-install ## Production build of the portal + check of the 5 lazy workspace chunks
	cd $(WEB_DIR) && $(PNPM) build

storybook: web-install ## Storybook of packages/ops-ui on :6006 (light/dark/compact)
	cd $(WEB_DIR) && $(PNPM) storybook

web-storybook-check: web-install ## Build Storybook (Light/Dark/Compact per component), axe on every story (needs Playwright Chromium)
	cd $(WEB_DIR) && $(PNPM) storybook:build
	cd $(WEB_DIR) && node tooling/storybook-a11y.mjs

web-visual: web-install ## Playwright visual regression of the stories (100% and 200% zoom), one worker
	cd $(WEB_DIR) && $(PNPM) test:visual

# ---------- game client: Godot (F060, GODOT_ARCHITECTURE §11-§12, ADR-0014) ----------
# Export paths are relative to the project folder (Godot resolves them against --path).
# GODOT is the Godot 4.x binary of the pinned minor (src/game/.godot-version); pass your own path, e.g.
#   make godot-test GODOT="/c/path/to/Godot_v4.7.2-stable_win64_console.exe"
GODOT    ?= godot
GDLINT   ?= gdlint
GAME_DIR := src/game
GODOT_HL  = "$(GODOT)" --headless --path $(GAME_DIR)
GAME_ENV ?= mock
.PHONY: godot-import godot-lint godot-test godot-smoke godot-screenshots godot-fixtures godot-export-check godot-export-windows godot-export-android
godot-import: ## Import the Godot project headless (global class cache, fonts); needed once after checkout
	$(GODOT_HL) --import

godot-lint: godot-import ## gdlint + typed check + theme override/layer/signal lints + translation keys + pseudo-locale + fixtures + no Admin UI
	cd $(GAME_DIR) && $(GDLINT) scripts scenes tools tests
	$(GODOT_HL) -s res://tools/check_typed.gd
	$(GODOT_HL) -s res://tools/lint_theme_overrides.gd
	$(GODOT_HL) -s res://tools/lint_layers.gd
	$(GODOT_HL) -s res://tools/lint_signals.gd
	$(GODOT_HL) -s res://tools/extract_pot.gd -- --check
	$(GODOT_HL) -s res://tools/pseudo_locale.gd -- --check
	node $(GAME_DIR)/tools/gen_fixtures.mjs --check
	bash $(GAME_DIR)/tools/check_no_admin_ui.sh

godot-test: godot-import ## GdUnit4 headless (unit, scene, contract, integration) + artifact scan tests (exports a release PCK)
	$(GODOT_HL) -s res://addons/gdUnit4/bin/GdUnitCmdTool.gd --ignoreHeadlessMode -c -a res://tests -rd res://reports
	GODOT="$(GODOT)" bash $(GAME_DIR)/tools/tests/scan_artifact_test.sh

godot-smoke: godot-import ## Boot headless with GAME_ENV (default mock) and require Boot -> Splash
	mkdir -p $(GAME_DIR)/build
	$(GODOT_HL) res://scenes/bootstrap/Boot.tscn --quit-after 300 -- --env=$(GAME_ENV) 2>&1 | tee $(GAME_DIR)/build/smoke.log | tail -n 5
	grep -q '"event":"splash_shown"' $(GAME_DIR)/build/smoke.log

godot-screenshots: godot-import ## UiGallery 4 densities x vi/en/xx-PS + high contrast vs baselines (needs a renderer; UPDATE=1 rewrites baselines)
	"$(GODOT)" --path $(GAME_DIR) $(GODOT_RENDER_FLAGS) -s res://tools/screenshot_gallery.gd -- $(if $(UPDATE),--update-baseline)

godot-fixtures: ## Regenerate contract fixtures, messages.pot and the pseudo-locale after a spec or UI string change
	node $(GAME_DIR)/tools/gen_fixtures.mjs
	$(GODOT_HL) -s res://tools/extract_pot.gd
	$(GODOT_HL) -s res://tools/pseudo_locale.gd

godot-export-check: godot-import ## Export the release PCK (no export template needed) and scan it for secrets, OIDC keys, mock config
	mkdir -p $(GAME_DIR)/build/check
	$(GODOT_HL) --export-pack "Windows Desktop" build/check/EduWorld.pck
	bash $(GAME_DIR)/tools/scan_artifact.sh $(GAME_DIR)/build/check

godot-export-windows: godot-import ## Export the Windows release (export templates of the pinned version required) and scan it
	mkdir -p $(GAME_DIR)/build/windows
	$(GODOT_HL) --export-release "Windows Desktop" build/windows/EduWorld.exe
	bash $(GAME_DIR)/tools/scan_artifact.sh $(GAME_DIR)/build/windows

godot-export-android: godot-import ## Export the Android APK, debug-signed (templates, Android SDK, JDK 17 required) and scan it
	mkdir -p $(GAME_DIR)/build/android
	$(GODOT_HL) --install-android-build-template
	$(GODOT_HL) --export-debug "Android" build/android/EduWorld.apk
	bash $(GAME_DIR)/tools/scan_artifact.sh $(GAME_DIR)/build/android

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
	bash scripts/ci/test-design-rules.sh
	bash scripts/ci/test-web-rules.sh
	bash scripts/ci/test-godot-rules.sh
	bash scripts/ci/check-admin-ports.sh
	bash scripts/ci/test-arch-fixtures.sh
	bash scripts/ci/test-gitleaks.sh
	bash scripts/ci/test-vuln-scan.sh
