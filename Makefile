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
NATIVE_ENV = EW_VALKEY_ADDR=localhost:6379 EW_NATS_URLS=nats://localhost:4222 EW_LOG_FORMAT=text   EW_EFFICACY_PSEUDONYM_KEY_FILE=$(CURDIR)/deploy/compose/secrets/efficacy_pseudonym_key \
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
.PHONY: up-app dev-secrets run-core-api run-realtime run-worker dev build
EXCEPT ?=
up-app: dev-secrets ## Run core-api, realtime, worker in containers (EXCEPT=<binary> to skip one)
	EW_GAME_LOGIN_TICKET_KEY=$${EW_GAME_LOGIN_TICKET_KEY:-$$(head -c 32 /dev/urandom | base64)} \
	  $(DC) --profile app up -d --build --wait $(filter-out $(EXCEPT),$(BINARIES))

dev-secrets: ## Generate the local secrets never committed (deploy/compose/secrets/, e.g. efficacy_pseudonym_key)
	bash scripts/dev/gen-dev-secrets.sh

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
.PHONY: migrate migrate-diff migrate-lint seed seed-identity seed-learning seed-game seed-check seed-content db-privileges db-backup db-restore
migrate: $(addprefix migrate-,$(MODULES)) ## Apply Atlas migrations to every module database

migrate-%: ## Apply Atlas migrations of one module (role <module>_migrator)
	$(MIGRATOR_ENV) $(GO) run ./cmd/migrate --module=$* --atlas="bash $(CURDIR)/scripts/dev/atlas.sh"

# Dev seed (LOCAL_DEVELOPMENT §5): the identity seed links the demo users of the realm import and needs the local
# issuer of the tokens core-api verifies (container: keycloak:8180; core-api run natively → EW_OIDC_ISSUERS=localhost).
SEED_ENV = EW_ENV=local EW_OIDC_ISSUERS=$${EW_OIDC_ISSUERS:-http://keycloak:8180/realms/eduworld} \
  EW_KC_ADMIN_BASE_URL=$${EW_KC_ADMIN_BASE_URL:-http://localhost:8180/admin/realms/eduworld} \
  EW_KC_ADMIN_CLIENT_ID=svc-identity EW_KC_ADMIN_CLIENT_SECRET=$${KC_SVC_IDENTITY_SECRET:-$(PW)}
seed: keycloak-import migrate seed-identity seed-learning seed-game ## Dev seed: realm import → migrate → identity → learning → game; then make up-app && make seed-check

seed-identity: ## Identity dev seed through its use cases (modules/identity/seed: bootstrap sa.admin, tenants, schools, SIS csv students, staff, roles)
	$(SEED_ENV) EW_DB_IDENTITY_DSN=$(call dsn,identity,app) EW_DB_IDENTITY_PLATFORM_DSN=$(call dsn,identity,platform) $(GO) run ./modules/identity/cmd/seed

seed-learning: ## Learning dev seed (subjects + curriculum GDPT2018_SAMPLE through the import use case, F021)
	EW_DB_LEARNING_DSN=$(call dsn,learning,app) EW_DB_LEARNING_PLATFORM_DSN=$(call dsn,learning,platform) $(GO) run ./modules/learning/cmd/seed

seed-game: ## Game dev seed (character catalog v1 + default item definitions through the publish use case, F070)
	EW_DB_GAME_DSN=$(call dsn,game,app) EW_DB_GAME_PLATFORM_DSN=$(call dsn,game,platform) $(GO) run ./modules/game/cmd/seed

seed-content: ## DEV published content Toán khối 3 + ADR-0029 assignment of it and GDPT2018_SAMPLE to th-demo-a/pgd-demo + DEV school assignments for DEMOA1 khối 3, through the API (after make up-app && make seed-check; EW_SEED_PASSWORD = demo password)
	EW_ENV=local EW_SEED_API_URL=$${EW_SEED_API_URL:-http://localhost:9080/api/v1} EW_SEED_INTERNAL_API_URL=$${EW_SEED_INTERNAL_API_URL:-http://localhost:9180/api/v1} \
	  $(GO) run ./modules/learning/cmd/seed-content

seed-game-content: ## DEV mini-game configs + their quest (seed/minigames) through the Game Operations Console (F078: create, review, publish, assign to the demo tenants; after make seed-content; EW_SEED_PASSWORD = demo password)
	EW_ENV=local EW_SEED_API_URL=$${EW_SEED_API_URL:-http://localhost:9080/api/v1} EW_SEED_INTERNAL_API_URL=$${EW_SEED_INTERNAL_API_URL:-http://localhost:9180/api/v1} \
	  $(GO) run ./modules/game/cmd/seed-content

seed-check: ## Wait until the worker projected every seeded student into ref.students of learning and game (after make up-app)
	bash scripts/dev/seed-check.sh

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

# ---------- staging/production on Docker Swarm (F130, DEPLOYMENT §4–§13, BACKUP_RECOVERY) ----------
# Run on the self-hosted runner `deploy` (DOCKER_HOST = Swarm manager over TLS); swarm-validate and test-ops need no swarm.
.PHONY: swarm-validate swarm-up deploy-service deploy-rollback secrets-sync restore-test ops-job test-ops test-ops-integration
ENV ?= staging
SERVICE ?=
VERSION ?=
MODE ?= full
DB ?= eduworld_learning
swarm-validate: ## Static checks of deploy/swarm/<env>: docker stack config, policy (placement, networks, secrets), scale schedule (ENV=staging|production; default both)
	bash scripts/deploy/validate-stacks.sh $(if $(filter command line,$(origin ENV)),--env=$(ENV))

swarm-up: ## First setup of an environment: node labels, overlay networks, then the infra, data and ops stacks (ENV=<env>)
	node scripts/deploy/label-nodes.mjs deploy/swarm/nodes/$(ENV).yaml
	bash deploy/swarm/networks.sh
	bash scripts/deploy/deploy-service.sh --env=$(ENV) --stack=infra
	bash scripts/deploy/deploy-service.sh --env=$(ENV) --stack=data
	bash scripts/deploy/deploy-service.sh --env=$(ENV) --stack=ops

deploy-service: ## Migrate jobs then deploy the app stack, or one service: ENV=<env> VERSION=vX.Y.Z [SERVICE=<swarm-service>]
	bash scripts/deploy/deploy-service.sh --env=$(ENV) --stack=app --version=$(VERSION) $(if $(SERVICE),--service=$(SERVICE))

deploy-rollback: ## Roll one backend Swarm service back to its previous spec (≤ 15 min): SERVICE=<swarm-service> ENV=<env>
	bash scripts/deploy/rollback.sh --env=$(ENV) --service=$(SERVICE)

secrets-sync: ## SOPS → versioned Docker secrets (<name>_v<N>, previous version kept): ENV=<env>
	bash scripts/deploy/secrets-sync.sh --env=$(ENV)

restore-test: ## Restore test in an isolated scratch cluster with the restore credential: ENV=<env> MODE=full|module [DB=eduworld_<module>]
	bash scripts/ops/run-ops-job.sh --env=$(ENV) --like=backup-tools --restore -- restore_test.sh --mode=$(MODE) $(if $(filter module,$(MODE)),--db=$(DB)) --actor="$(or $(EW_ACTOR),$(USER))" --purpose="$(or $(EW_PURPOSE),monthly restore test)"

ops-job: ## One-off runbook job copying a service: ENV=<env> LIKE=<swarm-service> CMD="…" [RESTORE=1] [SECRET=<name>]
	bash scripts/ops/run-ops-job.sh --env=$(ENV) --like=$(LIKE) $(if $(RESTORE),--restore) $(if $(SECRET),--secret=$(SECRET)) -- $(CMD)

test-ops: ## Unit tests of the deploy/backup/runbook scripts and the Swarm policy checks (stubbed docker, pgbackrest, rclone, nats)
	node --test tests/ops/

test-ops-integration: ## Backup, PITR, restore test, object storage copy/restore, Docker API proxy against throwaway containers
	bash tests/ops/integration/run.sh

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
	$(GO) run ./tools/routegroups
	node scripts/realtime/gen-routes.mjs # routes + client schemas of the realtime gateway from the AsyncAPI (F090)
	cd $(BACKEND)/pkg/testkit/sample/ent && go generate .
	cd $(BACKEND)/modules/identity/ent && go generate .

gen-check: gen ## Fail if generated files differ from the committed ones
	git diff --exit-code -- $(BACKEND)/.golangci.arch.yml $(BACKEND)/pkg/problem/codes_gen.go
	$(GO) run ./tools/allowedcallers --check
	cd $(BACKEND) && go run ./tools/facadegen --check --oapi=oapi-codegen
	$(GO) run ./tools/infragen --check
	$(GO) run ./tools/sisgen --check
	node scripts/realtime/gen-routes.mjs --check
	$(GO) run ./tools/routegroups --check # apisix.route_groups of getIntegrationOverview from deploy/apisix/routes/routes.yaml (OQ F059-12)
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
.PHONY: godot-import godot-lint godot-test godot-smoke godot-screenshots godot-fixtures godot-export-check godot-export-android godot-export-web godot-web-preview godot-web-test godot-export-ios godot-art-bake godot-art-capture
godot-import: ## Import the Godot project headless (global class cache, fonts); needed once after checkout
	$(GODOT_HL) --import

godot-lint: godot-import ## gdlint + typed check + theme override/layer/signal/art lints + baked art + brand images + translation keys + pseudo-locale + fixtures + no Admin UI
	cd $(GAME_DIR) && $(GDLINT) scripts scenes tools tests
	$(GODOT_HL) -s res://tools/check_typed.gd
	$(GODOT_HL) -s res://tools/lint_theme_overrides.gd
	$(GODOT_HL) -s res://tools/lint_layers.gd
	$(GODOT_HL) -s res://tools/lint_signals.gd
	$(GODOT_HL) -s res://tools/lint_art.gd
	$(GODOT_HL) -s res://tools/art/bake_art.gd -- --check
	$(GODOT_HL) -s res://tools/brand_assets.gd -- --check
	$(GODOT_HL) -s res://tools/extract_pot.gd -- --check
	$(GODOT_HL) -s res://tools/pseudo_locale.gd -- --check
	node $(GAME_DIR)/tools/gen_fixtures.mjs --check
	bash $(GAME_DIR)/tools/check_no_admin_ui.sh

godot-art-bake: godot-import ## F067: bake the first-party low-poly art (tools/art/*_art.gd) into src/game/assets/models (ADR-0102)
	$(GODOT_HL) -s res://tools/art/bake_art.gd

godot-art-capture: ## F067: render the Main Hub from fixed camera shots to PNG (needs a renderer; OUT=<dir>)
	"$(GODOT)" --path $(GAME_DIR) -s res://tools/capture_hub.gd -- --out="$(OUT)"

godot-test: godot-import ## GdUnit4 headless (unit, scene, contract, integration) + artifact scan tests (exports a release PCK)
	$(GODOT_HL) -s res://addons/gdUnit4/bin/GdUnitCmdTool.gd --ignoreHeadlessMode -c -a res://tests -rd res://reports
	GODOT="$(GODOT)" bash $(GAME_DIR)/tools/tests/scan_artifact_test.sh

godot-smoke: godot-import ## Boot headless with GAME_ENV (default mock) and require Boot -> Splash
	mkdir -p $(GAME_DIR)/build
	$(GODOT_HL) res://scenes/bootstrap/Boot.tscn --quit-after 300 -- --env=$(GAME_ENV) 2>&1 | tee $(GAME_DIR)/build/smoke.log | tail -n 5
	grep -q '"event":"splash_shown"' $(GAME_DIR)/build/smoke.log

godot-screenshots: godot-import ## UiGallery 4 densities x vi/en/xx-PS + high contrast vs baselines (needs a renderer; UPDATE=1 rewrites baselines)
	"$(GODOT)" --path $(GAME_DIR) $(GODOT_RENDER_FLAGS) -s res://tools/screenshot_gallery.gd -- $(if $(UPDATE),--update-baseline)

# F078 US5: manual render of a 3D asset with the script of the game-renderer container (DESIGN_SYSTEM §7). Needs a
# renderer (not --headless: the dummy renderer writes empty images); on Linux CI wrap with xvfb-run and pass
# GODOT_RENDER_FLAGS="--rendering-method gl_compatibility --rendering-driver opengl3". Paths are made absolute in the
# form the Godot binary understands (pwd -W under Git Bash on Windows).
OUT ?= build/renders
.PHONY: render-thumbnails
render-thumbnails: godot-import ## Render thumbnail.png (512x512) + turntable.png (24 frames, 3072x2048): make render-thumbnails INPUT=path/model.glb [OUT=build/renders]
	@test -n "$(INPUT)" -a -f "$(INPUT)" || { echo "usage: make render-thumbnails INPUT=path/to/model.glb [OUT=build/renders]" >&2; exit 2; }
	@mkdir -p "$(OUT)"
	@in="$$(cd "$$(dirname "$(INPUT)")" && { pwd -W 2>/dev/null || pwd; })/$$(basename "$(INPUT)")"; \
	out="$$(cd "$(OUT)" && { pwd -W 2>/dev/null || pwd; })"; \
	script=res://tools/thumbnail_renderer/render_thumbnails.gd; \
	"$(GODOT)" --path $(GAME_DIR) $(GODOT_RENDER_FLAGS) -s $$script -- --input "$$in" --output "$$out" \
	  --kind render_thumbnail --size 512 --frames 1; \
	"$(GODOT)" --path $(GAME_DIR) $(GODOT_RENDER_FLAGS) -s $$script -- --input "$$in" --output "$$out" \
	  --kind render_turntable --size 512 --frames 24; \
	echo "render-thumbnails: $$out/thumbnail.png $$out/turntable.png"

godot-fixtures: ## Regenerate contract fixtures, messages.pot and the pseudo-locale after a spec or UI string change
	node $(GAME_DIR)/tools/gen_fixtures.mjs
	$(GODOT_HL) -s res://tools/extract_pot.gd
	$(GODOT_HL) -s res://tools/pseudo_locale.gd

godot-export-check: godot-import ## Export the release PCK of the Android preset (no export template needed) and scan it for secrets, OIDC keys, mock config
	mkdir -p $(GAME_DIR)/build/check
	$(GODOT_HL) --export-pack "Android" build/check/EduWorld.pck
	bash $(GAME_DIR)/tools/scan_artifact.sh $(GAME_DIR)/build/check

godot-export-android: godot-import ## Export the Android APK, debug-signed (templates, Android SDK, JDK 17 required) and scan it
	mkdir -p $(GAME_DIR)/build/android
	$(GODOT_HL) --install-android-build-template
	$(GODOT_HL) --export-debug "Android" build/android/EduWorld.apk
	bash $(GAME_DIR)/tools/scan_artifact.sh $(GAME_DIR)/build/android

# F133 T002/T028: Web release export (Compatibility renderer, single-threaded, custom shell src/game/web/shell.html).
# The shell stylesheet is generated from design/game-tokens (make design-tokens) and published next to index.html.
godot-export-web: godot-import ## Export the Web release to build/web (web_nothreads template), add the shell stylesheet, check and scan it
	mkdir -p $(GAME_DIR)/build/web
	$(GODOT_HL) --export-release "Web" build/web/index.html
	cp $(GAME_DIR)/web/shell_tokens.css $(GAME_DIR)/build/web/
	bash $(GAME_DIR)/tools/ci/check_web_export.sh $(GAME_DIR)/build/web
	bash $(GAME_DIR)/tools/scan_artifact.sh $(GAME_DIR)/build/web

godot-web-preview: ## Serve build/web on http://localhost:8060 (the origin the local gateway allows on the game routes)
	node $(GAME_DIR)/tools/web/serve_web.mjs $(GAME_DIR)/build/web --port=8060

godot-web-test: ## Web shell tests: jsdom (always) + Chromium/WebKit on build/web when Playwright browsers are installed
	node --test $(GAME_DIR)/web/tests/

# iOS: Godot writes the Xcode project; building, signing and the IPA need Xcode on the macOS runner (F133 T019).
godot-export-ios: godot-import ## Export the iOS Xcode project to build/ios (no signing; team id comes from the macOS signing job)
	mkdir -p $(GAME_DIR)/build/ios
	$(GODOT_HL) --export-release "iOS" build/ios/EduWorld.ipa

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
