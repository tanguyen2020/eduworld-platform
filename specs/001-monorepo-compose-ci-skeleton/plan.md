# Implementation Plan: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Branch**: `001-monorepo-compose-ci-skeleton` · **Date**: 2026-11-16 (sửa 2026-11-17 theo ADR-0074, ADR-0088) · **Spec**: [spec.md](spec.md)

## Summary

Tạo khung monorepo theo ADR-0002 và modular monolith có ranh giới cưỡng chế theo [ADR-0074](../../docs/adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md): một `go.mod` ở gốc gom `src/backend/pkg/`, `src/backend/cmd/` (ba binary `core-api`, `realtime`, `worker --profile`) và `src/backend/modules/<module>/` (facade `api/`, `internal/` theo Clean Architecture + CQRS, `ent/`, `migrations/`, `module.go`). Kèm `Makefile` theo LOCAL_DEVELOPMENT §7, Docker Compose (hạ tầng với một cụm PostgreSQL 5 database + `keycloak`, NATS, Valkey, Keycloak, APISIX, floci; ba binary; observability), script init PostgreSQL tạo database + role theo module (ADR-0037, ADR-0074 mục 4), migration Atlas nền cho mỗi database (schema namespace, `infra.current_tenant_id()`, `REVOKE DELETE/TRUNCATE`), cổng quản trị (`src/backend/pkg/health`, `src/backend/pkg/buildinfo`), Dockerfile theo binary + image `migrate`, cấu hình `arch-check` chặn import chéo module, và CI/CD GitHub Actions theo [ADR-0088](../../docs/adr/ADR-0088-ci-cd-bang-github-actions.md) (lane theo path filter, branch protection, Dependabot, action ghim SHA, GitHub Environments, runner self-hosted cho deploy). Không có logic nghiệp vụ.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 (một module gốc); Bash/Make (GNU make); YAML (Compose v2 ≥ 2.24, GitHub Actions); Node ≥ 20 cho script kiểm workflow |
| Primary Dependencies | chi (`net/http`), pgx/v5, valkey-go, nats.go (ping), Atlas CLI, ent CLI, oapi-codegen, golangci-lint (`depguard`), go-arch-lint (ghim trong `src/backend/tools/tools.go`) — ADR-0025; action ghim SHA: `actions/checkout`, `actions/setup-go`, `dorny/paths-filter`, `docker/build-push-action`, `sigstore/cosign-installer`, `anchore/sbom-action`, `aquasecurity/trivy-action`, `gitleaks/gitleaks-action` |
| Storage | PostgreSQL 17 một container (một cụm) nhiều database (local); Valkey 8; NATS 2.10 JetStream; floci |
| Testing | `go test` + testcontainers-go (postgres:17); script kiểm CI (`scripts/ci/*.sh`, `scripts/ci/check-workflows.mjs`); fixture vi phạm ranh giới module |
| Target Platform | Linux container (amd64/arm64); máy dev Windows 11 (WSL2)/macOS/Linux; runner GitHub-hosted `ubuntu-latest`, `macos-latest` (iOS, dùng bởi F060/F133), self-hosted nhãn `deploy` |
| Project Type | Hạ tầng + platform kit tối thiểu + khung module |
| Performance Goals | Dựng stack ≤ 30 phút (LOCAL_DEVELOPMENT) |
| Constraints | AGENTS R4, R12, R13, R21, R29 · ADR-0024, ADR-0037, ADR-0039, ADR-0074, ADR-0088 |
| Scale/Scope | 3 binary (5 Swarm service ở staging/prod), 6 module, 5 database nghiệp vụ + `keycloak` trên một cụm |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Không có contract API mới (cổng quản trị ngoài OpenAPI); cấu hình arch-check sinh từ `x-allowed-callers` của spec internal có sẵn; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Go `internal/` + `make arch-check` (F001-AC-21…23); mỗi module có `internal/{domain,application/{command,query,port},infrastructure,presentation}` rỗng sẵn; `src/backend/pkg/` không import `src/backend/modules/`; không HTTP/token giữa module |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Database + role theo module trên một cụm, revoke `CONNECT` chéo, revoke `DELETE/TRUNCATE`, hàm `infra.current_tenant_id()` |
| IV Server-authoritative | ✅ | Không áp dụng (không có nghiệp vụ) |
| V Xác thực, phân quyền | ✅ | Cổng quản trị không route qua APISIX; không hardcode URL/secret (env + `_FILE`, GitHub Environment); job deploy không chạy cho PR từ fork |
| VI An toàn trẻ em | ✅ | Seed chỉ dữ liệu giả; log không PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Biến môi trường, cổng, target theo LOCAL_DEVELOPMENT §3.2, §4, §7; workflow theo DEPLOYMENT §10; không tự đặt tham số nghiệp vụ |
| VIII UI tách logic, token | ✅ | Không áp dụng |
| IX Test có bằng chứng | ✅ | Integration test role DB, test health, fixture ranh giới module, test path filter và `check-workflows` |

## Project Structure

### Documentation (feature này)

```text
specs/001-monorepo-compose-ci-skeleton/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
Makefile  .gitignore  .gitattributes  .gitleaks.toml      # Makefile: target LOCAL_DEVELOPMENT §7 (BACKEND := src/backend)
tools/versions.mk                                         # phiên bản công cụ (ent, oapi-codegen, golangci-lint, go-arch-lint, govulncheck, image Atlas)
src/README.md                                             # bố cục src/ (ADR-0092)
src/backend/                                              # Go module duy nhất (ADR-0074, ADR-0092)
├── go.mod  go.sum  .golangci.arch.yml                    # .golangci.arch.yml do tools/archgen sinh (depguard)
├── tools/archgen/{main.go,main_test.go}                  # kiểm ranh giới module + quy tắc tầng từ x-allowed-callers
├── pkg/
│   ├── buildinfo/      buildinfo.go                       # binary, profile, version, commit, build_time (ldflags)
│   ├── health/         registry.go checks.go registry_test.go
│   ├── module/         registry.go                        # Registry truyền cho modules/<m>.Register (pool DB theo module, health)
│   ├── bootstrap/      bootstrap.go valkey.go healthcheck.go   # chạy một binary: admin port, Valkey/NATS, tắt êm, lệnh healthcheck
│   └── testkit/        cluster.go                         # testcontainers: cụm PostgreSQL + init + migration Atlas, Valkey, NATS
├── cmd/
│   ├── core-api/       main.go + health_integration_test.go
│   ├── realtime/       main.go
│   ├── worker/         main.go + profile_integration_test.go   # --profile=events|chat|jobs|all
│   └── migrate/        main.go                             # migrate --module=<m> | --all (Atlas)
├── modules/{identity,learning,game,competition,chat}/
│   ├── module.go  atlas.hcl
│   ├── api/doc.go
│   ├── internal/{domain,application/{command,query,port},infrastructure,presentation}/doc.go
│   ├── ent/schema/doc.go
│   ├── migrations/{00000000000001_baseline.sql,atlas.sum,migrations.go}
│   └── seed/README.md
├── modules/realtime/{module.go,api/,internal/}            # không ent/migrations
└── test/postgres/db_isolation_integration_test.go         # F001-AC-08…10
src/cms/frontend/README.md  src/game/README.md            # khung do F050, F060
deploy/docker/{core-api,realtime,worker,migrate}.Dockerfile
deploy/compose/{docker-compose.yml,docker-compose.app.yml,docker-compose.observability.yml}
deploy/postgres/init/{01-databases.sql,02-roles.sh,03-revoke.sql}
deploy/apisix/{config.yaml,apisix.yaml}  deploy/cdn/nginx.conf
deploy/observability/{otel-collector.yaml,prometheus.yml,loki.yaml,tempo.yaml,grafana/provisioning/datasources/}
scripts/dev/{install-tools.sh,check-docker.sh,atlas.sh}
scripts/ci/{check-workflows.mjs,check-workflows.test.mjs,path-filters.mjs,path-filters.test.mjs,test-path-filters.sh,module-matrix.mjs,
            check-admin-ports.{sh,mjs},db-privileges.{sh,sql},smoke-compose.sh,test-arch-fixtures.sh,test-up-app-except.sh,
            test-gitleaks.sh,test-vuln-scan.sh,check-image.sh,specs-breaking.sh,apply-repo-settings.sh}
scripts/ops/{db-backup.sh,db-restore.sh,test-backup-restore.sh}
.github/
├── workflows/{go.yml,specs.yml,docs.yml,security.yml,release.yml,deploy-config.yml}
├── module-filters.yml                                    # dorny/paths-filter: mỗi module một filter, "shared" bật mọi module
├── dependabot.yml                                        # github-actions, gomod (/src/backend), npm (/scripts)
└── CODEOWNERS
```

**Structure Decision**: Theo ADR-0074 mục 1–2 và [DEPLOYMENT §3.1](../../docs/11-operations/DEPLOYMENT.md). `src/backend/pkg/health` và `src/backend/pkg/buildinfo` thuộc F001 vì FR-OPS-001; các gói khác của `src/backend/pkg/` do F002/F003 thêm vào cùng `go.mod`. Lane `design.yml`, `web.yml`, `godot.yml` do F005, F050, F060 thêm theo cùng quy ước.

## Phase 0: Research

Không có. ADR-0002, ADR-0024, ADR-0037, ADR-0039, ADR-0074, ADR-0088 và DEPLOYMENT/LOCAL_DEVELOPMENT đã đủ.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | Thư mục rỗng có `doc.go` mô tả luật import |
| application | — | `command/`, `query/`, `port/` rỗng |
| infrastructure | `persistence/pool.go` (pgxpool từ `EW_DB_<MODULE>_DSN`/`EW_DB_<MODULE>_URL_FILE`) | Dùng cho readiness; ent client do F003 |
| presentation | — | Route nghiệp vụ do feature sau thêm qua `module.go` |
| api (facade) | `doc.go` | Interface + DTO sinh từ spec internal do F003 (`make gen`) và feature sở hữu operation |
| cmd | `src/backend/cmd/core-api`: router public `:8100` (chưa route nghiệp vụ), admin `EW_ADMIN_ADDR`; `src/backend/cmd/realtime`: WS `:8106` (chưa có handler), admin; `src/backend/cmd/worker`: chọn profile từ `--profile`/`EW_WORKER_PROFILE`, admin | Mỗi binary gọi `Register` của module nó nạp; `src/backend/pkg/health.Handler` mount `/livez /readyz /healthz /metrics /version` |
| migrate | Image `eduworld/migrate`: `migrate --module=<module>` chạy `atlas migrate apply --url $EW_DB_<MODULE>_MIGRATOR_DSN --dir file://modules/<module>/migrations` | Một job cho mỗi database module |

`src/backend/pkg/health`: `Registry.Register(name string, check func(ctx) error, kind Readiness|Info)`; mỗi module nạp đăng ký check `db:<module>` + `atlas:<module>`; `/readyz` chạy check readiness song song timeout 2 s, trả `{"status":"ok|fail","checks":{name:{"status","error_code"}}}` (không lộ DSN); `/healthz` trả chi tiết (F003 đăng ký thêm). `/metrics` dùng `promhttp` với registry riêng có nhãn `binary` (và `profile` với worker).

**Ranh giới module (`make arch-check`)**: (1) Go `internal/` chặn import `src/backend/modules/<khác>/internal/...` ở compiler; (2) `src/backend/tools/archgen` đọc `x-allowed-callers` của mọi operation trong `specs/openapi/modules/*.internal.v1.yaml` và sinh rule `depguard` (`.golangci.arch.yml`) + `go-arch-lint` cho phép `src/backend/modules/<A>/**` import `src/backend/modules/<B>/api` chỉ khi A thuộc tập caller của B; (3) `src/backend/pkg/**` không import `src/backend/modules/**`; `src/backend/cmd/**` chỉ import `src/backend/modules/<m>` (gói gốc chứa `Register`). File sinh nằm trong `make gen`, nên `gen-check` bắt lệch spec ↔ rule. Fixture `scripts/ci/fixtures/arch/` chứa ba ca vi phạm cho test.

### CI/CD GitHub Actions (ADR-0088, DEPLOYMENT §10)

| Workflow | Kích hoạt (`paths`) | Job | Ghi chú |
|---|---|---|---|
| `go.yml` | `src/backend/modules/**`, `src/backend/pkg/**`, `src/backend/cmd/**`, `go.mod`, `go.sum`, `specs/openapi/modules/**`, `specs/events/**`, `specs/asyncapi/**`, `deploy/postgres/**` | `changes` (`dorny/paths-filter` + `.github/module-filters.yml`) → matrix `test` theo module (`make gen-check/lint/test/test-integration/migrate-lint MOD=<module>`) → `arch` (`make arch-check`, `make test-contract`, `go -C src/backend build ./cmd/...`) → `db-privileges` | `src/backend/pkg/**`, `src/backend/cmd/**`, `go.mod` bật mọi module |
| `specs.yml` | `specs/**` | `make specs-lint`, `make specs-breaking`, `node scripts/specs/check-features.mjs`, `node scripts/specs/check-cross-features.mjs`, `scripts/openapi/split-public.mjs --check`, `sync-platform-ops.mjs --check` | |
| `docs.yml` | `docs/**`, `README.md`, `scripts/docs/**`, `specs/openapi/eduworld-api.v1.yaml` | `make docs-check` | |
| `security.yml` | luôn chạy (`pull_request`, `push`) | gitleaks, govulncheck, OSV-Scanner/npm audit, trivy fs | Required status check |
| `release.yml` | `push` `main` (`src/backend/modules/**`, `src/backend/pkg/**`, `src/backend/cmd/**`, `deploy/docker/**`), tag `backend/v*` | `build-image` matrix 4 image → trivy image → Syft SBOM → cosign → push `ghcr.io/<org>/eduworld/<image>`; job deploy chỉ là khung (`runs-on: [self-hosted, deploy]`, `environment`) cho F130 | `permissions: packages: write, id-token: write` chỉ ở job build |
| `deploy-config.yml` | `deploy/**` | `docker compose config` cho mọi file compose; F004/F130 bổ sung APISIX/Keycloak/Swarm diff | |

Quy ước chung (kiểm bằng `scripts/ci/check-workflows.mjs`): `permissions: contents: read` mặc định; mọi `uses:` bên thứ ba ghim SHA 40 ký tự kèm comment version; job có `ios` trong tên/`matrix.target` chạy `runs-on: macos-*`; job `environment: staging|production` chỉ `runs-on: [self-hosted, deploy]` và có điều kiện `github.event.pull_request.head.repo.fork != true`; `concurrency` theo ref. `apply-repo-settings.sh --check` so branch protection `main` (required checks theo F001-AC-16, CODEOWNERS review, cấm force-push) và Environments `staging`/`production` (required reviewers) với cấu hình mong muốn.

### Tích hợp và phụ thuộc

- Feature phụ thuộc: — · Feature tiêu thụ: F002–F005, F050, F060 và mọi feature module.
- Compose: profile `default` (hạ tầng), `app` (`core-api`, `realtime`, `worker --profile=all`), `obs`, `media` (clamav), `tools` (nats-box), `mock` (dành cho F060).
- `make seed` = gọi `make keycloak-import` (F004 cung cấp realm) → `make migrate` → `seed` từng module nếu có `seed/` → `make seed-check`.

### Bảo mật, tenant, idempotency, audit

- `deploy/postgres/init`: tạo role theo [DATABASE_DESIGN §1.1](../../docs/07-backend/DATABASE_DESIGN.md), `REVOKE CONNECT ON DATABASE … FROM PUBLIC`, `GRANT CONNECT` chỉ database của module; migration baseline `REVOKE DELETE, TRUNCATE` + `ALTER DEFAULT PRIVILEGES` (DATABASE_DESIGN §3.0).
- Secret local chỉ `change-me-local`; gitleaks allowlist đúng chuỗi này trong `.env.example` và realm dev. Secret CI chỉ trong GitHub Environment; workflow PR không đọc secret.
- Không endpoint nghiệp vụ → không idempotency/audit.

### Quan sát

- Log khởi động JSON (`binary`, `profile`, `modules`, `version`, `commit`); metric `eduworld_build_info{binary,profile,version,commit}` = 1; stack `obs` nhận OTLP ở `otel-collector:4317` (instrument đầy đủ ở F002).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
