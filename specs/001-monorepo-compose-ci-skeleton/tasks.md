# Tasks: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo `go.mod` duy nhất (`src/backend/go.mod`, ADR-0092), `.gitignore` (`.env`, `data/`, `**/node_modules`), `.gitattributes` giữ nguyên — file: `src/backend/go.mod` — ✅ Go module ở `src/backend/go.mod` (module path `github.com/tanguyen2020/eduworld-platform/src/backend`) theo ADR-0092; `.gitignore` thêm `/data/`, `/bin/`, `**/node_modules/`.
- [x] T002 [P] Ghim công cụ (ent, atlas, oapi-codegen, golangci-lint, go-arch-lint) và target `make tools` — file: `src/backend/tools/tools.go` — ✅ Lệch: ghim phiên bản trong `tools/versions.mk` + `scripts/dev/install-tools.sh` (`go install …@version` vào `./bin`) thay vì `tools/tools.go`, để công cụ không kéo phụ thuộc vào `go.mod` duy nhất; Atlas chạy bằng image `arigaio/atlas:0.38.0` (bản `go install` đã cũ).
- [ ] T003 [P] `.env.example` gốc theo LOCAL_DEVELOPMENT §4 (`EW_HTTP_ADDR`, `EW_ADMIN_ADDR`, `EW_WORKER_PROFILE`, `EW_DB_<MODULE>_*`, `EW_VALKEY_<MODULE>_*`, `EW_KC_ADMIN_CLIENT_*`, `EW_S3_*`, `OTEL_*`) — file: `.env.example` — ⛔ CHẶN: cài đặt quyền của phiên chặn đọc/ghi file `.env*` nên chưa tạo được `.env.example`; compose dùng giá trị mặc định `${VAR:-…}` nên vẫn chạy. Nội dung theo LOCAL_DEVELOPMENT §4.
- [x] T004 [P] Cấu hình gitleaks với allowlist chỉ `change-me-local` — file: `.gitleaks.toml` — ✅ `scripts/ci/test-gitleaks.sh` → pass; quét toàn repo bằng gitleaks v8.28.0 → no leaks found.
- [x] T005 Makefile: `tools up down reset up-app[EXCEPT=<binary>] up-obs run-core-api run-realtime run-worker[PROFILE] dev gen gen-check arch-check migrate migrate-<module> migrate-diff migrate-lint seed seed-check lint test test-integration test-contract db-privileges db-backup db-restore env` — file: `Makefile` — ✅ Target `seed`, `seed-check` chưa có: cần realm (F004) và seed module (F010+). Thêm `build`, `specs-lint`, `specs-breaking`, `docs-check`, `ci-local`.
- [x] T006 [P] `.github/CODEOWNERS` theo module (`src/backend/modules/<module>/`), `deploy/`, `migrations/`, `src/backend/pkg/`, `src/backend/cmd/`, spec internal; `.github/dependabot.yml` cho `github-actions`, `gomod`, `npm` (lịch tuần, nhóm patch) — file: `.github/dependabot.yml`

## Phase 2: Foundational (chặn mọi user story)

- [x] T007 `src/backend/pkg/buildinfo` (biến ldflags `Binary`, `Profile`, `Version`, `Commit`, `BuildTime`) — file: `src/backend/pkg/buildinfo/buildinfo.go`
- [x] T008 `src/backend/pkg/health`: `Registry`, check song song timeout 2 s, handler `/livez` `/readyz` `/healthz` `/version`, mount `/metrics` (promhttp, nhãn `binary`/`profile`), metric `eduworld_build_info` — file: `src/backend/pkg/health/registry.go`
- [x] T009 [P] Check dựng sẵn `PostgresCheck(module)`, `ValkeyCheck`, `NATSCheck`, `AtlasRevisionCheck(module, min)` (không lộ DSN trong output) — file: `src/backend/pkg/health/checks.go`
- [x] T010 Script init DB trên một cụm: tạo 5 database module + `keycloak` — file: `deploy/postgres/init/01-databases.sql`
- [x] T011 Script tạo role `<module>_app/_platform/_migrator` + `chat_purger`, mật khẩu từ env, `REVOKE CONNECT … FROM PUBLIC`, `GRANT CONNECT` theo module — file: `deploy/postgres/init/02-roles.sh`
- [x] T012 Khung module mẫu `identity` (`api/doc.go`, `internal/*/doc.go`, `ent/schema/`, `migrations/`, `atlas.hcl`, `.env.example`, `module.go` với `Register(deps)` rỗng) rồi sao cho `learning`, `game`, `competition`, `chat` — file: `src/backend/modules/identity/module.go`
- [x] T013 [P] Khung module `realtime` (không DB; `api/`, `internal/`, `module.go`) — file: `src/backend/modules/realtime/module.go`
- [x] T014 Ba composition root: `src/backend/cmd/core-api` (nạp 5 module domain), `src/backend/cmd/realtime` (nạp `realtime`), `src/backend/cmd/worker` (cờ `--profile=events|chat|jobs|all`, nạp module theo profile); mỗi binary mở cổng quản trị — file: `src/backend/cmd/core-api/main.go`
- [x] T015 Migration baseline mỗi module: schema theo data-model §1, `infra.current_tenant_id()`, `REVOKE DELETE, TRUNCATE`, default privileges — file: `src/backend/modules/identity/migrations/00000000000001_baseline.sql` — kiểm: `make migrate-lint MOD=identity`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Developer dựng toàn bộ stack local (Priority: P1) 🎯 MVP

**Goal**: `make up` → `make migrate` → `make up-app` chạy được, mọi binary có health.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T016 [P] [US1] Test F001-AC-04: `/livez`, `/readyz`, `/healthz`, `/version` trả đúng mã và JSON; `/livez` không gọi check — file: `src/backend/pkg/health/registry_test.go` — phải FAIL trước khi code — ✅ Test viết trước, xác nhận FAIL (build failed) rồi PASS: `go test ./pkg/health/...` → 8 test pass.
- [x] T017 [P] [US1] Test F001-AC-05: readiness 503 kèm `db:<module>`/`valkey`/`nats` khi container dừng, livez vẫn 200; `worker --profile=chat` chỉ kiểm `eduworld_chat` — file: `src/backend/cmd/core-api/health_integration_test.go` — phải FAIL trước khi code — ✅ PASS với testcontainers (init script + migration Atlas thật): `TestCoreAPIReadinessFollowsDependencies`; profile `chat`/`events` ở `src/backend/cmd/worker/profile_integration_test.go`. Viết sau phần code binary (không chạy FAIL trước).
- [x] T018 [P] [US1] Test F001-AC-03: `core-api` khởi động trên DB chưa migrate trả readiness 503 (`atlas:<module>`) và không tạo bảng — file: `src/backend/cmd/core-api/no_automigrate_integration_test.go` — ✅ PASS: `TestCoreAPIDoesNotAutoMigrate` (cùng file `health_integration_test.go`) — `atlas:<module>` = `not_migrated`, không tạo schema.
- [x] T019 [P] [US1] Script kiểm F001-AC-06: APISIX config và compose không route/publish cổng quản trị ngoài 9100/9106/9200 local — file: `scripts/ci/check-admin-ports.sh` — ✅ PASS trên cấu hình thật; cấu hình cố ý sai (publish 19100:9090, route APISIX tới :9090) → exit 1, 2 lỗi.
- [x] T020 [P] [US1] Smoke test F001-AC-01, F001-AC-02, F001-AC-07: `make up` healthy, `.env` bị ignore, stack obs có datasource — file: `scripts/ci/smoke-compose.sh` — ✅ `scripts/ci/smoke-compose.sh` → 13/13 ok (9 dịch vụ healthy, s3-init exit 0, `.env` bị ignore, Grafana có Prometheus/Loki/Tempo).

### Implementation cho User Story 1

- [x] T021 [US1] Compose hạ tầng LOCAL_DEVELOPMENT §3.1 (một container PostgreSQL mount `deploy/postgres/init`, NATS JetStream, Valkey, Keycloak, APISIX, floci, mailpit, sis-mock; healthcheck từng container; profile default/media/tools/mock) — file: `deploy/compose/docker-compose.yml` — ✅ Lệch đã ghi Assumptions: floci local dùng credential mặc định `test`/`test`; `sis-mock` dùng `stoplight/prism:4` (prism 5 lỗi khởi động).
- [x] T022 [US1] Compose ba binary (profile `app`: `core-api` 8100, `realtime` 8106, `worker --profile=all`; admin `EW_ADMIN_ADDR=:9090` trong container, host 9100/9106/9200; env từ `.env`) và service one-off `migrate` — file: `deploy/compose/docker-compose.app.yml`
- [x] T023 [P] [US1] Compose observability + cấu hình collector/Prometheus/Loki (`app_log_retention`)/Tempo/Grafana provisioning — file: `deploy/compose/docker-compose.observability.yml`
- [x] T024 [US1] Wiring health trong `module.go`: mỗi module nạp đăng ký `db:<module>` + `atlas:<module>`; binary đăng ký Valkey/NATS — file: `src/backend/modules/identity/module.go`
- [x] T025 [US1] Binary `migrate --module=<module>` gọi `atlas migrate apply` bằng `EW_DB_<MODULE>_MIGRATOR_DSN`; target `make migrate` chạy lần lượt 5 module — file: `src/backend/cmd/migrate/main.go` — ✅ Chặn khi revision DB mới hơn code; chạy trong image `migrate` và native (`--atlas="bash scripts/dev/atlas.sh"`).
- [ ] T026 [P] [US1] `.env.example` từng module (DSN 3 role, Valkey ACL user của module) — file: `src/backend/modules/learning/.env.example` — ⛔ CHẶN: như T003 (`modules/<module>/.env.example`).

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Mỗi module một database, không xóa vật lý (Priority: P1)

**Goal**: Role theo module, revoke chéo và revoke xóa trên một cụm.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T027 [P] [US2] Integration test F001-AC-08, F001-AC-09: duyệt mọi cặp (role, database), chỉ DB của module kết nối được — file: `src/backend/test/postgres/db_isolation_integration_test.go` — phải FAIL trước khi code — ✅ File thực tế: `src/backend/test/postgres/db_isolation_integration_test.go` (Go test phải nằm trong Go module) → PASS (37 role × 6 database).
- [x] T028 [P] [US2] Integration test F001-AC-10: `DELETE`/`TRUNCATE` bằng `<module>_app`, `<module>_platform` bị từ chối trên bảng tạm trong mọi schema — file: `src/backend/test/postgres/no_delete_integration_test.go` — ✅ PASS (cùng file T027): INSERT/UPDATE/SELECT được, DELETE/TRUNCATE → 42501 trên mọi schema của 5 module.
- [x] T029 [P] [US2] Truy vấn kiểm F001-AC-11 trên `pg_catalog` (grant DELETE/TRUNCATE, CONNECT chéo) + target `make db-privileges` — file: `scripts/ci/db-privileges.sql` — ✅ `scripts/ci/db-privileges.sh` → ok; cố ý `GRANT DELETE` cho `learning_app` → exit 1 đúng dòng vi phạm.

### Implementation cho User Story 2

- [x] T030 [US2] Hoàn thiện revoke/default privileges trong baseline 5 module và script `03-revoke.sql` — file: `deploy/postgres/init/03-revoke.sql`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — CI GitHub Actions theo lane, path filter và quét bảo mật (Priority: P1)

**Goal**: Workflow theo lane và path filter, quét secret/dependency, branch protection, image an toàn, chuỗi cung ứng ghim.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T031 [P] [US3] Test F001-AC-12, F001-AC-24: script giả lập danh sách file thay đổi, đánh giá `on.*.paths` của mọi workflow và `.github/module-filters.yml`, so với bảng kỳ vọng (`src/backend/modules/learning/**` → `go.yml` matrix `[learning]`; `src/backend/pkg/**` → mọi module; `docs/**` → `docs.yml` + `security.yml`) — file: `scripts/ci/test-path-filters.sh` — phải FAIL trước khi code — ✅ `bash scripts/ci/test-path-filters.sh` → 19/19 pass (viết trước, FAIL do chưa có `path-filters.mjs`).
- [x] T032 [P] [US3] Test F001-AC-13, F001-AC-16: fixture secret giả trên nhánh thử làm `security.yml` đỏ; `apply-repo-settings.sh --check` xác nhận required checks và CODEOWNERS review — file: `scripts/ci/fixtures/fake-secret.txt` — ✅ Lệch: không commit file secret giả (sẽ làm `security.yml` đỏ trên `main`); `scripts/ci/test-gitleaks.sh` sinh secret lúc chạy → gitleaks phát hiện, placeholder được phép. Kiểm trên PR thật và `apply-repo-settings.sh --check` chờ GitHub.
- [x] T033 [P] [US3] Test F001-AC-14: module fixture có dependency chứa lỗ hổng đã biết làm `govulncheck`/OSV và `trivy fs` fail — file: `scripts/ci/fixtures/vuln-module/go.mod` — ✅ Lệch: module có lỗ hổng sinh tạm trong `scripts/ci/test-vuln-scan.sh` (không thêm `go.mod` thứ hai vào repo) → govulncheck báo GO-2022-1059.
- [x] T034 [P] [US3] Test F001-AC-15: `docker inspect` kiểm `User`, không shell, nhãn version; SBOM tồn tại; `cosign verify` — file: `scripts/ci/check-image.sh` — ✅ `scripts/ci/check-image.sh` → 4 image ok (UID 10001, nhãn version, không shell); SBOM + `cosign verify` chỉ kiểm được trong `release.yml`.
- [x] T035 [P] [US3] Test F001-AC-25, F001-AC-26: fixture workflow dùng `uses: …@v4`, thiếu `permissions`, job iOS trên `ubuntu-latest`, job `environment: production` trên runner GitHub-hosted hoặc không chặn fork → `check-workflows.mjs` fail đúng dòng — file: `scripts/ci/check-workflows.test.mjs` — phải FAIL trước khi code — ✅ `node --test scripts/ci/check-workflows.test.mjs` → 10/10 pass (viết trước, FAIL do chưa có module).

### Implementation cho User Story 3

- [x] T036 [US3] `go.yml`: `changes` (paths-filter ghim SHA) → matrix `test` (`gen-check`, `lint`, `test`, `test-integration` testcontainers, `migrate-lint` với `MOD`) → `arch` (`make arch-check`, `make test-contract`, `go -C src/backend build ./cmd/...`) → `db-privileges`; filter `.github/module-filters.yml` (`src/backend/pkg/**`, `src/backend/cmd/**`, `go.mod` bật mọi module) — file: `.github/workflows/go.yml` — ✅ Go lane theo `src/backend/**`; CI dùng Go `stable` mới nhất (thư viện chuẩn 1.27.0 trên máy dev có CVE đã vá ở bản patch).
- [x] T037 [P] [US3] `security.yml` luôn chạy: gitleaks, govulncheck, OSV-Scanner/npm audit, trivy fs (ngưỡng SECURITY_ARCHITECTURE §15) — file: `.github/workflows/security.yml`
- [x] T038 [P] [US3] `specs.yml` (`make specs-lint`, `make specs-breaking`, `check-features.mjs`, `check-cross-features.mjs`, `split-public.mjs --check`, `sync-platform-ops.mjs --check`) và `docs.yml` (`make docs-check`) — file: `.github/workflows/specs.yml`
- [x] T039 [P] [US3] Dockerfile theo binary multi-stage `CGO_ENABLED=0 -trimpath`, ldflags buildinfo, distroless non-root 10001, read-only; `migrate.Dockerfile` (Atlas + `src/backend/modules/*/migrations`) — file: `deploy/docker/core-api.Dockerfile`
- [x] T040 [US3] `release.yml`: `push` `main`/tag `backend/v*` → matrix build 4 image, trivy image (chặn Critical có fix), Syft SBOM, cosign (`id-token: write`), push `ghcr.io/<org>/eduworld/<image>` tag `git sha` + version; khung job deploy `runs-on: [self-hosted, deploy]` + `environment` + chặn fork (F130 điền bước deploy) — file: `.github/workflows/release.yml` — ✅ Job deploy là khung, chỉ chạy khi `vars.DEPLOY_ENABLED == true` (tránh treo khi chưa có runner `deploy`).
- [x] T041 [P] [US3] `deploy-config.yml` (`docker compose config` mọi file compose; chỗ cho F004/F130) — file: `.github/workflows/deploy-config.yml`
- [x] T042 [US3] `check-workflows.mjs` (SHA pin, `permissions`, runner macOS cho job iOS, runner `deploy` + environment + chặn fork cho job deploy) chạy trong `specs.yml`; `apply-repo-settings.sh` (branch protection `main`, Environments `staging`/`production` có required reviewers, cấm force-push) với `--check` — file: `scripts/ci/check-workflows.mjs` — ✅ `check-workflows.mjs` → 6 workflow, 0 lỗi. `apply-repo-settings.sh` chưa chạy (máy chưa có `gh`); required checks chỉ gồm job của `security.yml` — xem Assumptions về job cổng cho lane có path filter.

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Ba binary, module có ranh giới cưỡng chế (Priority: P1)

**Goal**: Một `go.mod`, ba binary + profile, ranh giới module được compiler và linter chặn, chạy lai native + container.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T043 [P] [US4] Test F001-AC-17, F001-AC-21, F001-AC-22, F001-AC-23: fixture `game` import `src/backend/modules/learning/internal/x` (build fail), module ngoài `x-allowed-callers` import `src/backend/modules/identity/api` (`make arch-check` fail), module trong `x-allowed-callers` import được, `src/backend/pkg/` import `src/backend/modules/` (fail), `src/backend/cmd/` import `src/backend/modules/<m>/internal` (fail); `go -C src/backend build ./cmd/...` pass — file: `scripts/ci/test-arch-fixtures.sh` — phải FAIL trước khi code — ✅ `bash scripts/ci/test-arch-fixtures.sh` → 8/8 pass, fixture tự dọn.
- [x] T044 [P] [US4] Test F001-AC-18, F001-AC-19: `make up-app EXCEPT=core-api` không khởi chạy container `core-api`; `/version` của ba binary cùng `version`/`commit`, `worker` có `profile` — file: `scripts/ci/test-up-app-except.sh` — ✅ `bash scripts/ci/test-up-app-except.sh` → ok: EXCEPT=core-api chỉ chạy realtime + worker; `/version` cùng version/commit, worker profile=all; `/readyz` 200; APISIX `/readyz` 404.

### Implementation cho User Story 4

- [x] T045 [US4] `src/backend/tools/archgen`: đọc `x-allowed-callers` mọi operation của `specs/openapi/modules/*.internal.v1.yaml`, sinh `.golangci.arch.yml` (depguard theo module) + `.go-arch-lint.yml`; nối vào `make gen`/`gen-check` và `make arch-check` — file: `src/backend/tools/archgen/main.go` — ✅ Lệch: `tools/archgen --check` (phân tích `go list`) là kiểm tra chính cho ranh giới module và quy tắc tầng; sinh `src/backend/.golangci.arch.yml` (depguard) cho `make lint`; không sinh `.go-arch-lint.yml`. Unit test `tools/archgen` pass.
- [x] T046 [US4] Target `up-app EXCEPT=<binary>` (lọc service compose), `run-core-api`, `run-realtime`, `run-worker PROFILE=`, `dev`; biến `EW_UPSTREAM_CORE_API`, `EW_UPSTREAM_REALTIME` truyền cho APISIX local — file: `Makefile`
- [x] T047 [P] [US4] Gắn ldflags `Binary`/`Profile`/`Version` (từ tag `backend/vX.Y.Z` hoặc `git describe`) trong Dockerfile và `release.yml` — file: `deploy/docker/worker.Dockerfile`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Sao lưu và khôi phục database local (Priority: P3)

**Goal**: Backup/restore một database module local.

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [x] T048 [P] [US5] Test F001-AC-20: backup → reset → restore, so số bảng và `atlas_schema_revisions` — file: `scripts/ops/test-backup-restore.sh` — phải FAIL trước khi code — ✅ `scripts/ops/test-backup-restore.sh` → ok (42 dòng, revision giữ nguyên, quyền `identity_app` giữ nguyên); FAIL trước khi có script.

### Implementation cho User Story 5

- [x] T049 [US5] `db-backup.sh` (`pg_dump -Fc` một database vào `data/backups/<module>-<ts>.dump`) và `db-restore.sh` (`pg_restore --clean` vào local, chặn khi `EW_ENV` ≠ `local`) — file: `scripts/ops/db-backup.sh` — ✅ Sửa lỗi phát hiện khi test: bản đầu dùng `--no-acl` làm mất quyền của role ứng dụng sau khi khôi phục; nay giữ ACL và test kiểm quyền.

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T050 [P] Đối chiếu LOCAL_DEVELOPMENT §3.2 (cổng 8100/8106, admin 9100/9106/9200), §4 (biến `EW_DB_<MODULE>_*`), §7 (target `arch-check`, `run-*`, `db-backup`, `db-restore`, `db-privileges`) với code; ghi lệch vào SPEC_RECONCILIATION_BACKLOG nếu có — file: `docs/11-operations/LOCAL_DEVELOPMENT.md` — ✅ LOCAL_DEVELOPMENT đã đối chiếu: floci `test`/`test`, prism 4, Atlas qua image, `make tools`, make trên Windows, Ryuk; cổng, biến `EW_DB_<MODULE>_*`, target khớp code.
- [x] T051 [P] Cập nhật DATABASE_DESIGN §1.2 thêm schema `catalog` (identity) và `analytics` (game) — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ Bảng §1.2 đã có `catalog` và `analytics`; sửa thêm lỗi chèn trùng ~130 dòng đầu DATABASE_DESIGN.
- [x] T052 [P] Đối chiếu DEPLOYMENT §10.2 (tên workflow theo ADR-0088) với `.github/workflows/` — file: `docs/11-operations/DEPLOYMENT.md` — ✅ DEPLOYMENT §10.2 khớp 6 workflow thật (path, job, Go `stable`, 4 image, OSV).
- [x] T053 [P] CHANGELOG mục F001 — file: `CHANGELOG.md` — ✅ CHANGELOG: mục F001 và ADR-0092.
- [x] T054 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/001-monorepo-compose-ci-skeleton/quickstart.md` — ✅ Bằng chứng ở quickstart §5.

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
