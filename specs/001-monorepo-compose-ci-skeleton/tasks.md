# Tasks: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo `go.mod` duy nhất ở gốc (module path repository), `.gitignore` (`.env`, `data/`, `**/node_modules`), `.gitattributes` giữ nguyên — file: `go.mod`
- [ ] T002 [P] Ghim công cụ (ent, atlas, oapi-codegen, golangci-lint, go-arch-lint) và target `make tools` — file: `tools/tools.go`
- [ ] T003 [P] `.env.example` gốc theo LOCAL_DEVELOPMENT §4 (`EW_HTTP_ADDR`, `EW_ADMIN_ADDR`, `EW_WORKER_PROFILE`, `EW_DB_<MODULE>_*`, `EW_VALKEY_<MODULE>_*`, `EW_KC_ADMIN_CLIENT_*`, `EW_S3_*`, `OTEL_*`) — file: `.env.example`
- [ ] T004 [P] Cấu hình gitleaks với allowlist chỉ `change-me-local` — file: `.gitleaks.toml`
- [ ] T005 Makefile: `tools up down reset up-app[EXCEPT=<binary>] up-obs run-core-api run-realtime run-worker[PROFILE] dev gen gen-check arch-check migrate migrate-<module> migrate-diff migrate-lint seed seed-check lint test test-integration test-contract db-privileges db-backup db-restore env` — file: `Makefile`
- [ ] T006 [P] `.github/CODEOWNERS` theo module (`modules/<module>/`), `deploy/`, `migrations/`, `pkg/`, `cmd/`, spec internal; `.github/dependabot.yml` cho `github-actions`, `gomod`, `npm` (lịch tuần, nhóm patch) — file: `.github/dependabot.yml`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T007 `pkg/buildinfo` (biến ldflags `Binary`, `Profile`, `Version`, `Commit`, `BuildTime`) — file: `pkg/buildinfo/buildinfo.go`
- [ ] T008 `pkg/health`: `Registry`, check song song timeout 2 s, handler `/livez` `/readyz` `/healthz` `/version`, mount `/metrics` (promhttp, nhãn `binary`/`profile`), metric `eduworld_build_info` — file: `pkg/health/registry.go`
- [ ] T009 [P] Check dựng sẵn `PostgresCheck(module)`, `ValkeyCheck`, `NATSCheck`, `AtlasRevisionCheck(module, min)` (không lộ DSN trong output) — file: `pkg/health/checks.go`
- [ ] T010 Script init DB trên một cụm: tạo 5 database module + `keycloak` — file: `deploy/postgres/init/01-databases.sql`
- [ ] T011 Script tạo role `<module>_app/_platform/_migrator` + `chat_purger`, mật khẩu từ env, `REVOKE CONNECT … FROM PUBLIC`, `GRANT CONNECT` theo module — file: `deploy/postgres/init/02-roles.sh`
- [ ] T012 Khung module mẫu `identity` (`api/doc.go`, `internal/*/doc.go`, `ent/schema/`, `migrations/`, `atlas.hcl`, `.env.example`, `module.go` với `Register(deps)` rỗng) rồi sao cho `learning`, `game`, `competition`, `chat` — file: `modules/identity/module.go`
- [ ] T013 [P] Khung module `realtime` (không DB; `api/`, `internal/`, `module.go`) — file: `modules/realtime/module.go`
- [ ] T014 Ba composition root: `cmd/core-api` (nạp 5 module domain), `cmd/realtime` (nạp `realtime`), `cmd/worker` (cờ `--profile=events|chat|jobs|all`, nạp module theo profile); mỗi binary mở cổng quản trị — file: `cmd/core-api/main.go`
- [ ] T015 Migration baseline mỗi module: schema theo data-model §1, `infra.current_tenant_id()`, `REVOKE DELETE, TRUNCATE`, default privileges — file: `modules/identity/migrations/00000000000001_baseline.sql` — kiểm: `make migrate-lint MOD=identity`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Developer dựng toàn bộ stack local (Priority: P1) 🎯 MVP

**Goal**: `make up` → `make migrate` → `make up-app` chạy được, mọi binary có health.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [ ] T016 [P] [US1] Test F001-AC-04: `/livez`, `/readyz`, `/healthz`, `/version` trả đúng mã và JSON; `/livez` không gọi check — file: `pkg/health/registry_test.go` — phải FAIL trước khi code
- [ ] T017 [P] [US1] Test F001-AC-05: readiness 503 kèm `db:<module>`/`valkey`/`nats` khi container dừng, livez vẫn 200; `worker --profile=chat` chỉ kiểm `eduworld_chat` — file: `cmd/core-api/health_integration_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US1] Test F001-AC-03: `core-api` khởi động trên DB chưa migrate trả readiness 503 (`atlas:<module>`) và không tạo bảng — file: `cmd/core-api/no_automigrate_integration_test.go`
- [ ] T019 [P] [US1] Script kiểm F001-AC-06: APISIX config và compose không route/publish cổng quản trị ngoài 9100/9106/9200 local — file: `scripts/ci/check-admin-ports.sh`
- [ ] T020 [P] [US1] Smoke test F001-AC-01, F001-AC-02, F001-AC-07: `make up` healthy, `.env` bị ignore, stack obs có datasource — file: `scripts/ci/smoke-compose.sh`

### Implementation cho User Story 1

- [ ] T021 [US1] Compose hạ tầng LOCAL_DEVELOPMENT §3.1 (một container PostgreSQL mount `deploy/postgres/init`, NATS JetStream, Valkey, Keycloak, APISIX, floci, mailpit, sis-mock; healthcheck từng container; profile default/media/tools/mock) — file: `deploy/compose/docker-compose.yml`
- [ ] T022 [US1] Compose ba binary (profile `app`: `core-api` 8100, `realtime` 8106, `worker --profile=all`; admin `EW_ADMIN_ADDR=:9090` trong container, host 9100/9106/9200; env từ `.env`) và service one-off `migrate` — file: `deploy/compose/docker-compose.app.yml`
- [ ] T023 [P] [US1] Compose observability + cấu hình collector/Prometheus/Loki (`app_log_retention`)/Tempo/Grafana provisioning — file: `deploy/compose/docker-compose.observability.yml`
- [ ] T024 [US1] Wiring health trong `module.go`: mỗi module nạp đăng ký `db:<module>` + `atlas:<module>`; binary đăng ký Valkey/NATS — file: `modules/identity/module.go`
- [ ] T025 [US1] Binary `migrate --module=<module>` gọi `atlas migrate apply` bằng `EW_DB_<MODULE>_MIGRATOR_DSN`; target `make migrate` chạy lần lượt 5 module — file: `cmd/migrate/main.go`
- [ ] T026 [P] [US1] `.env.example` từng module (DSN 3 role, Valkey ACL user của module) — file: `modules/learning/.env.example`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Mỗi module một database, không xóa vật lý (Priority: P1)

**Goal**: Role theo module, revoke chéo và revoke xóa trên một cụm.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [ ] T027 [P] [US2] Integration test F001-AC-08, F001-AC-09: duyệt mọi cặp (role, database), chỉ DB của module kết nối được — file: `deploy/postgres/tests/db_isolation_integration_test.go` — phải FAIL trước khi code
- [ ] T028 [P] [US2] Integration test F001-AC-10: `DELETE`/`TRUNCATE` bằng `<module>_app`, `<module>_platform` bị từ chối trên bảng tạm trong mọi schema — file: `deploy/postgres/tests/no_delete_integration_test.go`
- [ ] T029 [P] [US2] Truy vấn kiểm F001-AC-11 trên `pg_catalog` (grant DELETE/TRUNCATE, CONNECT chéo) + target `make db-privileges` — file: `scripts/ci/db-privileges.sql`

### Implementation cho User Story 2

- [ ] T030 [US2] Hoàn thiện revoke/default privileges trong baseline 5 module và script `03-revoke.sql` — file: `deploy/postgres/init/03-revoke.sql`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — CI GitHub Actions theo lane, path filter và quét bảo mật (Priority: P1)

**Goal**: Workflow theo lane và path filter, quét secret/dependency, branch protection, image an toàn, chuỗi cung ứng ghim.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [ ] T031 [P] [US3] Test F001-AC-12, F001-AC-24: script giả lập danh sách file thay đổi, đánh giá `on.*.paths` của mọi workflow và `.github/module-filters.yml`, so với bảng kỳ vọng (`modules/learning/**` → `go.yml` matrix `[learning]`; `pkg/**` → mọi module; `docs/**` → `docs.yml` + `security.yml`) — file: `scripts/ci/test-path-filters.sh` — phải FAIL trước khi code
- [ ] T032 [P] [US3] Test F001-AC-13, F001-AC-16: fixture secret giả trên nhánh thử làm `security.yml` đỏ; `apply-repo-settings.sh --check` xác nhận required checks và CODEOWNERS review — file: `scripts/ci/fixtures/fake-secret.txt`
- [ ] T033 [P] [US3] Test F001-AC-14: module fixture có dependency chứa lỗ hổng đã biết làm `govulncheck`/OSV và `trivy fs` fail — file: `scripts/ci/fixtures/vuln-module/go.mod`
- [ ] T034 [P] [US3] Test F001-AC-15: `docker inspect` kiểm `User`, không shell, nhãn version; SBOM tồn tại; `cosign verify` — file: `scripts/ci/check-image.sh`
- [ ] T035 [P] [US3] Test F001-AC-25, F001-AC-26: fixture workflow dùng `uses: …@v4`, thiếu `permissions`, job iOS trên `ubuntu-latest`, job `environment: production` trên runner GitHub-hosted hoặc không chặn fork → `check-workflows.mjs` fail đúng dòng — file: `scripts/ci/check-workflows.test.mjs` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T036 [US3] `go.yml`: `changes` (paths-filter ghim SHA) → matrix `test` (`gen-check`, `lint`, `test`, `test-integration` testcontainers, `migrate-lint` với `MOD`) → `arch` (`make arch-check`, `make test-contract`, `go build ./cmd/...`) → `db-privileges`; filter `.github/module-filters.yml` (`pkg/**`, `cmd/**`, `go.mod` bật mọi module) — file: `.github/workflows/go.yml`
- [ ] T037 [P] [US3] `security.yml` luôn chạy: gitleaks, govulncheck, OSV-Scanner/npm audit, trivy fs (ngưỡng SECURITY_ARCHITECTURE §15) — file: `.github/workflows/security.yml`
- [ ] T038 [P] [US3] `specs.yml` (`make specs-lint`, `make specs-breaking`, `check-features.mjs`, `check-cross-features.mjs`, `split-public.mjs --check`, `sync-platform-ops.mjs --check`) và `docs.yml` (`make docs-check`) — file: `.github/workflows/specs.yml`
- [ ] T039 [P] [US3] Dockerfile theo binary multi-stage `CGO_ENABLED=0 -trimpath`, ldflags buildinfo, distroless non-root 10001, read-only; `migrate.Dockerfile` (Atlas + `modules/*/migrations`) — file: `deploy/docker/core-api.Dockerfile`
- [ ] T040 [US3] `release.yml`: `push` `main`/tag `backend/v*` → matrix build 4 image, trivy image (chặn Critical có fix), Syft SBOM, cosign (`id-token: write`), push `ghcr.io/<org>/eduworld/<image>` tag `git sha` + version; khung job deploy `runs-on: [self-hosted, deploy]` + `environment` + chặn fork (F130 điền bước deploy) — file: `.github/workflows/release.yml`
- [ ] T041 [P] [US3] `deploy-config.yml` (`docker compose config` mọi file compose; chỗ cho F004/F130) — file: `.github/workflows/deploy-config.yml`
- [ ] T042 [US3] `check-workflows.mjs` (SHA pin, `permissions`, runner macOS cho job iOS, runner `deploy` + environment + chặn fork cho job deploy) chạy trong `specs.yml`; `apply-repo-settings.sh` (branch protection `main`, Environments `staging`/`production` có required reviewers, cấm force-push) với `--check` — file: `scripts/ci/check-workflows.mjs`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Ba binary, module có ranh giới cưỡng chế (Priority: P1)

**Goal**: Một `go.mod`, ba binary + profile, ranh giới module được compiler và linter chặn, chạy lai native + container.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [ ] T043 [P] [US4] Test F001-AC-17, F001-AC-21, F001-AC-22, F001-AC-23: fixture `game` import `modules/learning/internal/x` (build fail), module ngoài `x-allowed-callers` import `modules/identity/api` (`make arch-check` fail), module trong `x-allowed-callers` import được, `pkg/` import `modules/` (fail), `cmd/` import `modules/<m>/internal` (fail); `go build ./cmd/...` pass — file: `scripts/ci/test-arch-fixtures.sh` — phải FAIL trước khi code
- [ ] T044 [P] [US4] Test F001-AC-18, F001-AC-19: `make up-app EXCEPT=core-api` không khởi chạy container `core-api`; `/version` của ba binary cùng `version`/`commit`, `worker` có `profile` — file: `scripts/ci/test-up-app-except.sh`

### Implementation cho User Story 4

- [ ] T045 [US4] `tools/archgen`: đọc `x-allowed-callers` mọi operation của `specs/openapi/modules/*.internal.v1.yaml`, sinh `.golangci.arch.yml` (depguard theo module) + `.go-arch-lint.yml`; nối vào `make gen`/`gen-check` và `make arch-check` — file: `tools/archgen/main.go`
- [ ] T046 [US4] Target `up-app EXCEPT=<binary>` (lọc service compose), `run-core-api`, `run-realtime`, `run-worker PROFILE=`, `dev`; biến `EW_UPSTREAM_CORE_API`, `EW_UPSTREAM_REALTIME` truyền cho APISIX local — file: `Makefile`
- [ ] T047 [P] [US4] Gắn ldflags `Binary`/`Profile`/`Version` (từ tag `backend/vX.Y.Z` hoặc `git describe`) trong Dockerfile và `release.yml` — file: `deploy/docker/worker.Dockerfile`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Sao lưu và khôi phục database local (Priority: P3)

**Goal**: Backup/restore một database module local.

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [ ] T048 [P] [US5] Test F001-AC-20: backup → reset → restore, so số bảng và `atlas_schema_revisions` — file: `scripts/ops/test-backup-restore.sh` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T049 [US5] `db-backup.sh` (`pg_dump -Fc` một database vào `data/backups/<module>-<ts>.dump`) và `db-restore.sh` (`pg_restore --clean` vào local, chặn khi `EW_ENV` ≠ `local`) — file: `scripts/ops/db-backup.sh`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [ ] T050 [P] Đối chiếu LOCAL_DEVELOPMENT §3.2 (cổng 8100/8106, admin 9100/9106/9200), §4 (biến `EW_DB_<MODULE>_*`), §7 (target `arch-check`, `run-*`, `db-backup`, `db-restore`, `db-privileges`) với code; ghi lệch vào SPEC_RECONCILIATION_BACKLOG nếu có — file: `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [ ] T051 [P] Cập nhật DATABASE_DESIGN §1.2 thêm schema `catalog` (identity) và `analytics` (game) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T052 [P] Đối chiếu DEPLOYMENT §10.2 (tên workflow theo ADR-0088) với `.github/workflows/` — file: `docs/11-operations/DEPLOYMENT.md`
- [ ] T053 [P] CHANGELOG mục F001 — file: `CHANGELOG.md`
- [ ] T054 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/001-monorepo-compose-ci-skeleton/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1, US2, US3, US4 (song song được sau T015) → US5 → Polish.
- T012 trước T014, T015, T024; T014 trước T017, T018, T022, T044; T021 trước T017, T020; T036 trước T043 (job `arch`); T045 trước T036 hoàn tất (`make arch-check`).
- F002, F003, F004, F005, F050, F060 bắt đầu khi Phase 2 + US1 xong; F005/F050/F060 thêm `design.yml`/`web.yml`/`godot.yml` theo quy ước `check-workflows` (T042).

## Definition of Done

- [ ] Mọi tiêu chí F001-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make arch-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
