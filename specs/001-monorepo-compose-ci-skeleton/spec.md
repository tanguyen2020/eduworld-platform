# Feature Specification: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Feature Branch**: `001-monorepo-compose-ci-skeleton` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: hạ tầng (`go.mod` gốc, `src/backend/cmd/`, `src/backend/modules/`, `Makefile`, `deploy/compose`, `deploy/postgres`, `.github/`) · **Mốc**: M-C0 · **Phụ thuộc**: —
**Input**: Dựng monorepo (ADR-0002) theo modular monolith có ranh giới cưỡng chế (ADR-0074): một Go module gốc, ba binary `core-api`, `realtime`, `worker` (`--profile`), khung 5 module domain + module `realtime`; môi trường Docker Compose local (ADR-0024); CI/CD GitHub Actions theo lane và path filter (ADR-0088); mọi binary có health/readiness — FR-OPS-001, FR-OPS-002, FR-OPS-004, FR-OPS-005, NFR-DEPL-001, NFR-DEPL-002, NFR-SEC-005.

## Clarifications

### Session 2026-11-16

- Q: Thư mục mã là `services/learning/` hay `src/backend/modules/learning/`? → A: `src/backend/modules/<module>/` (và `src/backend/modules/realtime/`). Chốt bởi [ADR-0074](../../docs/adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) mục 1 (thay đề xuất D1).
- Q: Tên biến môi trường DB/kết nối? → A: Bảng duy nhất ở [LOCAL_DEVELOPMENT §4](../../docs/11-operations/LOCAL_DEVELOPMENT.md): kết nối database theo module `EW_DB_<MODULE>_*` (vì `core-api`/`worker` nạp nhiều module), `EW_S3_*`, `OTEL_*`, `EW_WORKER_PROFILE`; secret ở staging/prod dùng hậu tố `_FILE`. Không còn `EW_SVC_*` (không gọi HTTP giữa module); client Keycloak duy nhất cho backend là `svc-identity` qua `EW_KC_ADMIN_CLIENT_*`.
- Q: Cổng quản trị: một cổng hay tách health/metrics? → A: Một cổng `EW_ADMIN_ADDR` phục vụ `/livez`, `/readyz`, `/healthz`, `/metrics`, `/version`.

### Session 2026-11-17

- Q: Nền CI/CD? → A: GitHub Actions ([ADR-0088](../../docs/adr/ADR-0088-ci-cd-bang-github-actions.md)): mỗi lane một workflow trong `.github/workflows/` chạy theo `paths` filter; registry `ghcr.io`; GitHub Environments `staging`/`production`; runner GitHub-hosted, macOS cho iOS, self-hosted chỉ cho bước triển khai. Bỏ toàn bộ GitLab CI.
- Q: Có cổng internal API (82xx) không? → A: Không. Module gọi nhau trong tiến trình qua facade `src/backend/modules/<module>/api` (ADR-0074 mục 3). Cổng: `core-api` 8100 (admin host 9100), `realtime` 8106 (admin host 9106), `worker` admin host 9200; trong container admin luôn `:9090` ([LOCAL_DEVELOPMENT §3.2](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Q: Đơn vị build và phát hành? → A: Ba binary build từ cùng commit, chung version `backend/vX.Y.Z`; image `eduworld/core-api`, `eduworld/realtime`, `eduworld/worker`, `eduworld/migrate` trên `ghcr.io` ([DEPLOYMENT §3.1](../../docs/11-operations/DEPLOYMENT.md)).

### Session 2026-11-17 (triển khai)

- Q: Mã nguồn đặt ở đâu? → A: Trong `src/` theo [ADR-0092](../../docs/adr/ADR-0092-to-chuc-ma-nguon-trong-src.md): `src/backend` là Go module duy nhất (`go.mod`, `cmd/`, `modules/`, `pkg/`, `tools/`, `test/`), `src/cms/frontend` (Admin Portal), `src/game` (Godot); `deploy/`, `specs/`, `docs/`, `scripts/`, `design/` ở gốc.
- Q: Role `<module>_readonly/_relay/_partition_admin` tạo ở đâu? → A: Trong script init của cụm (`deploy/postgres/init/02-roles.sh`) cùng các role khác, vì role là đối tượng cấp cụm và `<module>_migrator` không được có `CREATEROLE`; F003 chỉ `GRANT` trong migration.
- Q: Kiểm tra ranh giới module dùng công cụ nào? → A: `tools/archgen --check` (phân tích `go list`) là kiểm tra chính cho import chéo module, `x-allowed-callers`, `pkg/`, `cmd/` và quy tắc tầng; `archgen` sinh thêm `src/backend/.golangci.arch.yml` (depguard) cho `make lint`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Developer dựng toàn bộ stack local (Priority: P1)

Developer mới clone repo, chạy `make up`, `make migrate`, `make seed`, `make up-app` và có hạ tầng + ba binary chạy, mỗi binary báo sống/sẵn sàng.

**Why this priority**: Mọi feature khác cần môi trường chạy được; là kịch bản demo M-C0.

**Independent Test**: Trên máy sạch đáp ứng LOCAL_DEVELOPMENT §1, chạy chuỗi lệnh trong quickstart; mọi container healthy và `curl` cổng quản trị của `core-api`, `realtime`, `worker` trả 200.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F001-AC-01 | KHI developer chạy `make up`, HỆ THỐNG PHẢI khởi động mọi dịch vụ hạ tầng profile default của [LOCAL_DEVELOPMENT §3.1](../../docs/11-operations/LOCAL_DEVELOPMENT.md) (một cụm PostgreSQL, NATS JetStream, Valkey, Keycloak, APISIX, floci và các container phụ) và mọi container đạt trạng thái healthy | FR-OPS-002 · LOCAL_DEVELOPMENT §3.1 | — |
| F001-AC-02 | HỆ THỐNG PHẢI có `.env.example` gốc và `src/backend/modules/<module>/.env.example` khai báo mọi biến bắt buộc của [LOCAL_DEVELOPMENT §4](../../docs/11-operations/LOCAL_DEVELOPMENT.md) với giá trị chỉ dùng local; `.env` thật PHẢI nằm trong `.gitignore` | FR-OPS-002 · FR-OPS-004 · AGENTS R4 | — |
| F001-AC-03 | KHI chạy `make migrate`, job `migrate --module=<module>` PHẢI áp migration Atlas của từng module vào đúng database của module đó bằng role `<module>_migrator`; `core-api`, `realtime`, `worker` PHẢI KHÔNG tự migrate khi khởi động | FR-OPS-002 · NFR-DEPL-002 · ADR-0039 · ADR-0074 | — |
| F001-AC-04 | Mọi binary (`core-api`, `realtime`, `worker` ở mọi profile) PHẢI mở cổng quản trị `EW_ADMIN_ADDR` phục vụ `/livez`, `/readyz`, `/healthz`, `/metrics`, `/version`; `/livez` PHẢI KHÔNG gọi phụ thuộc ngoài | FR-OPS-001 · [DEPLOYMENT §14](../../docs/11-operations/DEPLOYMENT.md) | — |
| F001-AC-05 | TRONG KHI database của một module mà binary nạp, Valkey hoặc NATS không truy cập được, `/readyz` của binary đó PHẢI trả `503` kèm tên check lỗi (vd. `db:learning`, `valkey`, `nats`) và `/livez` PHẢI vẫn trả `200`; readiness PHẢI KHÔNG phụ thuộc binary khác | FR-OPS-001 · [BACKEND_DESIGN §14](../../docs/07-backend/BACKEND_DESIGN.md) · ADR-0074 | — |
| F001-AC-06 | NẾU cổng quản trị bị khai báo route trên APISIX hoặc publish ra host ngoài mạng compose (trừ cổng host 9100, 9106, 9200 ở local), THÌ CI PHẢI fail | FR-OPS-001 · [API_DESIGN §10](../../docs/07-backend/API_DESIGN.md) | — |
| F001-AC-07 | KHI chạy `make up-obs`, HỆ THỐNG PHẢI khởi động otel-collector, Prometheus, Grafana, Loki, Tempo với datasource đã cấu hình sẵn | FR-OPS-002 · ADR-0023 | — |

---

### User Story 2 — Mỗi module một database, không xóa vật lý (Priority: P1)

DevOps cần bảo đảm từ ngày đầu: năm database module nằm trên một cụm PostgreSQL, role của module chỉ vào được database của module mình và không có quyền `DELETE`/`TRUNCATE`.

**Why this priority**: NFR-DEPL-002, ADR-0028 và ADR-0074 mục 4 là bất biến; ranh giới dữ liệu là điều kiện để tách module sau này; sửa sau phải migrate toàn bộ.

**Independent Test**: Integration test kết nối bằng từng role vào từng database và thử `DELETE`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F001-AC-08 | HỆ THỐNG PHẢI tạo trên một cụm PostgreSQL các database `eduworld_identity`, `eduworld_learning`, `eduworld_game`, `eduworld_competition`, `eduworld_chat`, `keycloak` và bộ role `<module>_app`, `<module>_platform`, `<module>_migrator` (+ `chat_purger`), mỗi role chỉ có `CONNECT` vào database của module mình | NFR-DEPL-002 · [DATABASE_DESIGN §1.1](../../docs/07-backend/DATABASE_DESIGN.md) · ADR-0074 | TC-SVC-007 |
| F001-AC-09 | NẾU role của module A kết nối database của module B, THÌ PostgreSQL PHẢI từ chối kết nối | NFR-DEPL-002 · ADR-0037 · ADR-0074 | TC-SVC-007 |
| F001-AC-10 | NẾU role `<module>_app` hoặc `<module>_platform` thực thi `DELETE` hoặc `TRUNCATE` trên bảng thuộc schema của module, THÌ PostgreSQL PHẢI trả lỗi quyền | DATABASE_DESIGN §3.0 · ADR-0028 | TC-TEN-016 |
| F001-AC-11 | Job CI `db-privileges` (trong `go.yml`) PHẢI fail khi có role ứng dụng mang `DELETE`/`TRUNCATE` (trừ `chat_purger` trên danh sách bảng DATABASE_DESIGN §3.0) hoặc có `CONNECT` chéo database | DATABASE_DESIGN §1.1 | TC-TEN-016 |

---

### User Story 3 — CI GitHub Actions theo lane, path filter và quét bảo mật (Priority: P1)

Tech Lead cần pull request chỉ chạy lane liên quan, luôn quét secret và dependency, nhánh `main` được bảo vệ, image build an toàn và chuỗi cung ứng được ghim.

**Why this priority**: Không có CI thì không có bằng chứng kiểm thử (constitution IX); quét secret là điều kiện R4; ADR-0088 chốt GitHub Actions.

**Independent Test**: Mở PR mẫu chỉ sửa `src/backend/modules/learning/**`; xác nhận danh sách workflow/job chạy; mở PR mẫu chỉ sửa `docs/**`; commit một secret giả và xác nhận check đỏ, PR không merge được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F001-AC-12 | KHI PR chỉ thay đổi `src/backend/modules/<module>/**`, workflow `go.yml` PHẢI chạy job matrix (`gen-check`, `lint`, `test`, `test-integration`, `migrate-lint`) chỉ cho module đó cùng job `arch` toàn repo, và PHẢI KHÔNG chạy matrix của module khác, `godot.yml`, `web.yml`; KHI thay đổi `src/backend/pkg/**`, `src/backend/cmd/**` hoặc `go.mod`, `go.yml` PHẢI chạy matrix của mọi module | FR-OPS-005 · ADR-0088 · [DEPLOYMENT §10.2](../../docs/11-operations/DEPLOYMENT.md) | — |
| F001-AC-13 | HỆ THỐNG PHẢI chạy workflow `security.yml` (gitleaks) ở mọi PR và mọi push, là required status check, và PHẢI chặn merge khi phát hiện secret | FR-OPS-004 · NFR-SEC-005 · ADR-0088 | — |
| F001-AC-14 | KHI PR chạy, `security.yml` PHẢI quét dependency (govulncheck, OSV/npm audit) và filesystem (trivy fs) theo ngưỡng [SECURITY_ARCHITECTURE §15](../../docs/10-security/SECURITY_ARCHITECTURE.md) và PHẢI fail khi vượt ngưỡng | NFR-SEC-005 | — |
| F001-AC-15 | KHI `release.yml` build image trên `main` hoặc tag `backend/vX.Y.Z`, HỆ THỐNG PHẢI build multi-stage `CGO_ENABLED=0` bốn image `core-api`, `realtime`, `worker`, `migrate`, chạy user non-root UID ≥ 10001, quét Trivy và chặn khi có CVE Critical đã có bản vá, đính SBOM, ký cosign, đẩy lên `ghcr.io`, tag bằng `git sha` + version backend và PHẢI KHÔNG dùng tag `latest` | NFR-SEC-005 · ADR-0088 · [DEPLOYMENT §3.1](../../docs/11-operations/DEPLOYMENT.md) | — |
| F001-AC-16 | Branch protection của `main` PHẢI yêu cầu review CODEOWNERS và các required status check `specs.yml`, `docs.yml`, `security.yml`, `go.yml` (`gen-check`, `lint`, `test`, `test-integration`, `migrate-lint`, `arch`, `db-privileges`) của lane liên quan; NẾU một check thất bại, THÌ PR PHẢI KHÔNG merge được | FR-OPS-005 · NFR-MAINT-002 · ADR-0088 | — |
| F001-AC-24 | Mỗi lane của ADR-0088 (`go.yml`, `specs.yml`, `docs.yml`, `security.yml`, `release.yml`, `deploy-config.yml`; `web.yml`, `design.yml`, `godot.yml` do F050, F005, F060 thêm) PHẢI là một workflow riêng trong `.github/workflows/` kích hoạt bằng `on.<event>.paths` theo bảng [DEPLOYMENT §10.2](../../docs/11-operations/DEPLOYMENT.md) (trừ `security.yml` luôn chạy); KHI PR chỉ thay đổi `docs/**`, HỆ THỐNG PHẢI chỉ chạy `docs.yml` và `security.yml` | FR-OPS-005 · ADR-0088 | — |
| F001-AC-25 | Script `check-workflows` PHẢI fail khi một bước `uses:` của action bên thứ ba không ghim bằng commit SHA đầy đủ, khi workflow thiếu `permissions` mặc định `contents: read`, hoặc khi job export/ký iOS không chạy trên runner macOS; HỆ THỐNG PHẢI có `.github/dependabot.yml` cho `github-actions`, `gomod`, `npm` | ADR-0088 · NFR-SEC-005 | — |
| F001-AC-26 | Job triển khai PHẢI chỉ chạy trên self-hosted runner nhãn `deploy` gắn GitHub Environment `staging` hoặc `production` (có required reviewers, secret triển khai/ký chỉ cấp cho environment đó) và PHẢI KHÔNG chạy cho sự kiện `pull_request` từ fork | ADR-0088 · FR-OPS-004 · AGENTS R4 | — |

---

### User Story 4 — Ba binary, module có ranh giới cưỡng chế (Priority: P1)

Backend dev thêm code vào một module mà không vô tình phụ thuộc vào phần bên trong module khác; chạy native một binary trong khi phần còn lại chạy container; ba binary phát hành chung một version.

**Why this priority**: ADR-0074 chỉ giữ được lợi ích (tách module sau rẻ) khi ranh giới được compiler và linter cưỡng chế từ commit đầu tiên; cần trước khi nhiều người làm song song ở M-C1.

**Independent Test**: Thêm import `src/backend/modules/learning/internal/...` vào module `game` → `go build` fail; thêm import `src/backend/modules/identity/api` vào module không có trong `x-allowed-callers` → `make arch-check` fail; `make up-app EXCEPT=core-api` + `make run-core-api`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F001-AC-17 | Repository PHẢI có một `go.mod` duy nhất (`src/backend/go.mod`, ADR-0092), ba binary `src/backend/cmd/core-api`, `src/backend/cmd/realtime`, `src/backend/cmd/worker` (cờ `--profile` nhận `events`, `chat`, `jobs`, `all`) và module `identity`, `learning`, `game`, `competition`, `chat`, `realtime` theo cấu trúc `src/backend/modules/<module>/{api/, internal/{domain,application,infrastructure,presentation}/, ent/, migrations/, module.go}` (module `realtime` không có `ent/`, `migrations/`); `go -C src/backend build ./cmd/...` PHẢI thành công | NFR-DEPL-001 · ADR-0074 | — |
| F001-AC-18 | KHI developer chạy `make up-app EXCEPT=<binary>`, HỆ THỐNG PHẢI chạy các binary còn lại trong container và cho phép chạy binary đó native bằng `make run-core-api`, `make run-realtime` hoặc `make run-worker PROFILE=<profile>` | NFR-DEPL-001 · LOCAL_DEVELOPMENT §3.2, §7 | — |
| F001-AC-19 | `GET /version` PHẢI trả `binary`, `version`, `commit`, `build_time` (và `profile` với `worker`) được gắn lúc build; ba binary build từ cùng commit PHẢI có cùng `version`, phát hành bằng tag `backend/vX.Y.Z` | NFR-DEPL-001 · DEPLOYMENT §2, §14 · ADR-0074 | — |
| F001-AC-21 | NẾU mã của một module import gói `src/backend/modules/<khác>/internal/...`, THÌ `go build` PHẢI fail (quy tắc `internal/` của Go) | NFR-MAINT-001 · ADR-0074 · AGENTS R13 | — |
| F001-AC-22 | NẾU module A import `src/backend/modules/<B>/api` mà A không có trong `x-allowed-callers` của operation nào trong `specs/openapi/modules/<B>.internal.v1.yaml`, THÌ `make arch-check` (cấu hình `depguard`/`go-arch-lint` sinh từ spec internal) PHẢI fail trong job `arch` của `go.yml`; import `src/backend/modules/<B>/api` của module có trong `x-allowed-callers` PHẢI được phép | NFR-MAINT-001 · ADR-0074 · AGENTS R13, R29 | — |
| F001-AC-23 | NẾU `src/backend/pkg/**` import `src/backend/modules/**`, hoặc `src/backend/cmd/<binary>` truy cập module khác ngoài hàm `Register` trong `src/backend/modules/<module>/module.go`, THÌ `make arch-check` PHẢI fail | NFR-MAINT-001 · ADR-0074 | — |

---

### User Story 5 — Sao lưu và khôi phục database local (Priority: P3)

Developer sao lưu/khôi phục database của một module trước thử nghiệm migration.

**Why this priority**: FR-OPS-002 yêu cầu backup script; production PITR do F130 làm.

**Independent Test**: `make db-backup MOD=identity`, xóa volume, `make db-restore` và kiểm số dòng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F001-AC-20 | KHI chạy `make db-backup MOD=<module>`, HỆ THỐNG PHẢI tạo bản dump chỉ của database module đó vào `data/backups/` (không commit); KHI chạy `make db-restore MOD=<module> FILE=<dump>`, HỆ THỐNG PHẢI khôi phục vào môi trường local | FR-OPS-002 · [BACKUP_RECOVERY](../../docs/11-operations/BACKUP_RECOVERY.md) | — |

### Edge Cases

- NẾU Docker không đủ RAM theo LOCAL_DEVELOPMENT §1, THÌ `make up` PHẢI dừng với thông báo yêu cầu cấu hình, không khởi động một phần im lặng.
- NẾU `make migrate` chạy khi Atlas revision của DB mới hơn code, THÌ job PHẢI fail và không sửa schema (F001-AC-03).
- NẾU Keycloak chưa sẵn sàng, `/readyz` vẫn chỉ dựa vào database của module nạp/Valkey/NATS (JWKS check do F002 thêm, F001-AC-05).
- NẾU `worker` chạy profile `chat`, readiness chỉ kiểm `eduworld_chat`, Valkey, NATS; profile `events`/`jobs` kiểm database của mọi module domain (F001-AC-05).
- NẾU workflow được kích hoạt bởi PR từ fork, THÌ không có secret của environment và không có job deploy (F001-AC-26).
- NẾU spec internal đổi `x-allowed-callers`, THÌ `make gen` sinh lại cấu hình arch-check và `gen-check` fail khi lệch (F001-AC-22).
- Authorization, tenant isolation ở mức endpoint, idempotency, Problem Details và giới hạn độ dài input: không áp dụng — F001 không tạo endpoint nghiệp vụ hay operation facade; cổng quản trị không qua APISIX (F001-AC-06); cô lập dữ liệu ở mức database bằng F001-AC-08…11.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-OPS-001 | `src/backend/pkg/health` + cổng quản trị cho `core-api`, `realtime`, `worker` | DEPLOYMENT §14, BACKEND_DESIGN §14 |
| FR-OPS-002 | Compose local, `.env.example`, job migrate theo module, backup/restore local | LOCAL_DEVELOPMENT §3–§7 |
| FR-OPS-004 | `.gitignore`, gitleaks, secret qua `_FILE`, secret theo GitHub Environment | SECURITY_ARCHITECTURE §6, DEPLOYMENT §10.3, §13 |
| FR-OPS-005 | Workflow GitHub Actions theo lane + path filter, branch protection, `check-workflows` | DEPLOYMENT §10, ADR-0088 |
| NFR-DEPL-001 | Một Go module, ba binary + profile, image theo binary, version backend chung, `up-app EXCEPT` | DEPLOYMENT §2, §3.1, §11, ADR-0074 |
| NFR-DEPL-002 | Một cụm PostgreSQL, database + role theo module, revoke chéo | DATABASE_DESIGN §1.1, MULTITENANCY §5.1, ADR-0074 mục 4 |
| NFR-SEC-005 | govulncheck, OSV/npm audit, trivy fs/image, SBOM, cosign, action ghim SHA, Dependabot | SECURITY_ARCHITECTURE §15, §16, ADR-0088 |
| NFR-MAINT-001 | Ranh giới module: Go `internal/`, `make arch-check` theo `x-allowed-callers` | ADR-0074 mục 1, AGENTS R13 |

### Key Entities

- **Binary**: `core-api` (REST public của mọi module, route `game-admin`, handler NATS request/reply), `realtime` (WebSocket, không DB), `worker` (profile `events`, `chat`, `jobs`; local `all`); mỗi binary một cổng quản trị và một image.
- **Module skeleton**: `src/backend/modules/<module>/` gồm facade `api/` (sinh từ spec internal), `internal/` theo Clean Architecture + CQRS, `ent/`, `migrations/`, `module.go` (`Register`).
- **Database của module**: database PostgreSQL riêng trên một cụm, bộ role `<module>_*`, các schema namespace của module.
- **Workflow CI**: một lane GitHub Actions (`.github/workflows/<lane>.yml`) với path filter, required status check, runner và environment theo ADR-0088.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `app_log_retention` | Cấu hình Loki trong stack `obs` local |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Developer mới dựng stack ≤ 30 phút | LOCAL_DEVELOPMENT (mục tiêu đầu tài liệu) |
| Không auto-migrate, expand/contract | AGENTS R12 · ADR-0039 |
| Image non-root, read-only rootfs, không shell runtime | DEPLOYMENT §3.1 |
| Không commit secret; secret qua Docker secrets/`_FILE`, GitHub Environment | AGENTS R4 · FR-OPS-004 · ADR-0088 |
| Không HTTP nội bộ, không token `svc-*` giữa module | ADR-0074 mục 3, 5 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Developer mới dựng xong hạ tầng + ba binary và có mọi `/readyz` = 200 trong ≤ 30 phút trên máy tham chiếu.
- **SC-002**: 100% role ứng dụng bị từ chối khi kết nối database module khác hoặc chạy `DELETE`/`TRUNCATE` (test tự động).
- **SC-003**: PR chỉ chạm một module kích hoạt 0 job matrix của module khác và 0 job `godot.yml`/`web.yml`; 100% PR có `security.yml`.
- **SC-004**: 0 image phát hành có CVE Critical đã có bản vá; 100% image có SBOM và chữ ký cosign; 100% action bên thứ ba ghim SHA.
- **SC-005**: 100% import vượt ranh giới module (Go `internal/`, ngoài `x-allowed-callers`) trong fixture bị chặn trước khi merge.

## Ngoài phạm vi

- Platform kit `config`, `observability`, `problem`, `httpx`, `authz`, `tenancy` → F002; `entx` (`UnitOfWork`), `outbox`, `modcall`, `testkit`, graceful shutdown đầy đủ → F003.
- Realm Keycloak và route APISIX (`/api/v1/*` → `core-api`, `/realtime/v1` → `realtime`) → F004 (F001 chỉ khai báo container `keycloak`, `apisix` trong compose).
- Lane `design.yml` → F005; `web.yml` → F050; `godot.yml` (export Windows/Android/Web trên runner Linux, iOS trên runner macOS) → F060, ký và phát hành 4 nền tảng → F133. F001 chỉ đặt quy ước runner/environment và kiểm bằng `check-workflows` (F001-AC-25).
- Seed dữ liệu nghiệp vụ của từng module → F010 trở đi; Swarm staging/production, job deploy thật, backup PITR, DR → F130; dashboard/alert → F131.
- Khung web (`src/cms/frontend/`) → F050; khung Godot (`src/game/`) → F060; design tokens → F005.

## Assumptions

- ADR-0074 và ADR-0088 Accepted (2026-11-17); cổng, tên biến, tên target theo LOCAL_DEVELOPMENT §3.2, §4, §7 và DEPLOYMENT §3.1, §10.
- Runner GitHub-hosted có Docker cho testcontainers; repository có quyền cấu hình branch protection/rulesets và GitHub Environments (Owner repo áp cấu hình bằng script `scripts/ci/apply-repo-settings.sh` dùng `gh api` [Proposed]; F001 kiểm lại bằng cùng script ở chế độ `--check`).
- Image `migrate` chứa migration của mọi module; job `migrate --module=<module>` là đơn vị áp migration ([DEPLOYMENT §3.1](../../docs/11-operations/DEPLOYMENT.md)).
- Ghi nhận khi triển khai (2026-11-17):
  - floci bật `FLOCI_SERVICES_S3_ENFORCE_AUTH` chỉ nhận credential mặc định `test`/`test` → local/CI dùng giá trị này (staging/production theo ADR-0075).
  - `sis-mock` dùng `stoplight/prism:4` (prism 5 lỗi khi khởi động).
  - Atlas chạy bằng image ghim `arigaio/atlas:0.38.0` trong mạng compose; Atlas cảnh báo bản này sắp hết hỗ trợ → nâng phiên bản trong `tools/versions.mk` khi có tag mới.
  - CI dùng Go `stable` mới nhất (bản patch vá CVE của thư viện chuẩn); máy dev nên cập nhật Go lên bản patch mới nhất.
  - Required status check của `main` chỉ gồm job `security.yml` (luôn chạy). Lane có path filter (`go`, `specs`, `docs`) không đặt required trực tiếp được (workflow bị bỏ qua sẽ treo "pending"); cần thêm job cổng không có path filter tổng hợp kết quả lane đã chạy [Proposed, việc tiếp theo].
  - Không commit file secret giả hay module có lỗ hổng: `test-gitleaks.sh`, `test-vuln-scan.sh` sinh fixture lúc chạy.
