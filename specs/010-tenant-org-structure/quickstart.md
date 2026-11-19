# Quickstart: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F003, F004.
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `sa.admin` (System Admin, MFA tắt ở local), `ad.donvi` (Tenant Admin `pgd-demo`), `ad.truong.a`, `ad.truong.b` (School Admin), `hs.an.3a`, `hs.em.4a`, `hs.tudo`.
- Token portal lấy bằng `POST /api/v1/auth/portal/login` (mật khẩu + OTP, Keycloak local qua module identity — ADR-0091); gọi API qua APISIX nội bộ cho `/admin/*`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sa.admin` gọi `createTenant` (`th-demo-x`, `school`) với `Idempotency-Key` | `201`; Keycloak có Organization alias `th-demo-x`, attribute `tenant_id`; audit `tenant.created` | F010-AC-01, F010-AC-05 |
| 2 | Gọi lại `createTenant` loại `common` | `409 CONFLICT` | F010-AC-02 |
| 3 | `ad.donvi` gọi `createTenant` | `403 PERMISSION_DENIED` (gateway nội bộ chặn từ public: 404) | F010-AC-04 |
| 4 | `updateTenant` `th-demo-x` → `disabled` | Organization `enabled=false`; resolve thành viên → `TENANT_ACCESS_DISABLED` | F010-AC-03 |
| 5 | `GET /catalog/provinces?country=VN` | 34 đơn vị `active` | F010-AC-06 |
| 6 | `importSchools` vào `pgd-demo` CSV có `vn.old.ha-giang` và một dòng thiếu tỉnh | Dòng 1 ánh xạ `vn.tuyen-quang` + cảnh báo; dòng 2 `pending_review` | F010-AC-08 |
| 7 | `ad.donvi` `updateTenantSchool` với tỉnh `inactive`; rồi `approveTenantSchool` với tỉnh `active` | `422`; sau đó `active`, audit | F010-AC-07, F010-AC-09 |
| 8 | `ad.truong.b` gọi `listClasses` của TH Demo A (tenant khác) và TH Demo C (cùng tenant) | `404` / `403 PERMISSION_DENIED` | F010-AC-17, F010-AC-18 |
| 9 | Chuyển `hs.an.3a` sang 3B; chuyển `hs.em.4a` sang THCS Demo B (cùng `pgd-demo`) | Một enrollment active; `enrollment.changed`; `hs.em.4a` giữ tenant và membership | F010-AC-11, F010-AC-13, F010-AC-14 |
| 10 | Thêm tay `hs.an.3a` vào Organization `pgd-demo`, chạy `org-membership-reconcile --once` | Member bị gỡ, issue `unexpected_org_member` | F010-AC-22 |
| 11 | `sa.admin` gọi `createCommonTenantStudents` khối 4 | User + enrollment `common-g4`, member Organization `common` | F010-AC-24 |
| 11b | `sa.admin` gọi `createCommonTenantStudents` lô 3 dòng (`hs.tudo1` khối 4, `hs.tudo2` khối 7, dòng 3 trùng `hs.an.3a`), rồi gọi lại cùng `Idempotency-Key` | `200`: 2 `created` có `initial_password` (khối 7 phải đổi mật khẩu lần đầu), 1 `failed` `USERNAME_TAKEN`; lần gọi lại: cùng kết quả, không có `initial_password`, `password_redacted = true` | F010-AC-28, F010-AC-29 |
| 12 | `hs.an.3a` gọi `getMyStudentProfile` | Trường/lớp/tỉnh song ngữ của chính mình | F010-AC-19 |
| 13 | Facade test: module learning gọi `listProjectionSnapshot` `entity_type=class`, `tenant_id` của `pgd-demo`, `limit=2` rồi theo `next_cursor` | Đủ lớp của tenant, có `source_version`; không lớp của tenant khác | F010-AC-27 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity
make migrate-lint MOD=identity
node scripts/specs/validate-reference-data.mjs
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), trace_id của kịch bản 1 và 9, ảnh Organization trong Keycloak admin console (local), log reconciliation.

## 5. Bằng chứng (2026-11-19, Windows 11 + Docker Desktop 27.1; postgres:17 testcontainers với mọi migration Atlas)

Kịch bản §2 được tự động hóa bằng test (gói `src/backend/modules/identity/...`); kịch bản thủ công trên stack chờ phần nối binary (OQ F010-2 trong spec).

| Kịch bản / tiêu chí | Test | Kết quả |
|---|---|---|
| 1, 2, 4 — AC-01, AC-02, AC-03 (Organization), AC-05 | `TestTenantLifecycle`, `command/tenant_test.go`, `command/org_sync_test.go`, `keycloak/admin_test.go` | pass |
| 3 — AC-04, AC-05 qua HTTP (403 thiếu MFA/role tenant, 428 thiếu key, replay, 422 `maxLength`, 428/412 `If-Match`) | `TestIdentityHTTP` (chuỗi `httpx` thật + `httpxtest`) | pass |
| 5 — AC-06 | `TestOrgSchema/CatalogSeed` | pass |
| 6 — AC-08 | `TestImportSchools` (tỉnh cũ → kế thừa + `province_outdated`, thiếu tỉnh → `pending_review`, mã tỉnh sai, trùng mã, dry-run) | pass |
| 7 — AC-07, AC-09 | `TestSchoolLifecycle`, `command/school_test.go` | pass |
| 8 — AC-10, AC-17, AC-18, AC-19 | `TestOrgQueries` | pass |
| 9 — AC-11…AC-15 | `TestTransferStudent`, `TestOrgSchema/EnrollmentInvariants` | pass |
| AC-16 | `TestOrgSchema/UserChecks` | pass |
| AC-20 | `TestMembershipSync` | pass |
| 10 — AC-22 | `TestReconcileOrganizations` | pass |
| AC-23 | `keycloak/admin_test.go` (`TestRetries`), consumer trả lỗi → Nak/DLQ (`pkg/outbox`) | pass |
| 11, 11b — AC-24, AC-25, AC-28, AC-29 | `TestCreateCommonStudents`, `TestIdentityHTTP` (`no-store`, replay `password_redacted`) | pass |
| AC-26 | `tests/contract/no_self_registration_test.go` | pass |
| 13 — AC-27 | `TestListProjectionSnapshot` | pass |
| Catalog write (ADMIN_PORTAL §3.16) | `TestCatalogWrites` | pass |
| Bảng route ↔ spec | `tests/contract/routes_test.go` (24 operation) | pass |

Chưa đủ: AC-03 phần resolve thành viên → `TENANT_ACCESS_DISABLED` và AC-21 (realm role nhân sự theo UserRole) cần resolver/`user_roles` của F012; `importSchools` bất đồng bộ (OQ F010-1, F011); kịch bản 4/10/12 trên stack thật (OQ F010-2).

Lệnh: `go test ./modules/identity/...` (unit + contract) và `TESTCONTAINERS_RYUK_DISABLED=true go test -tags integration -timeout 45m ./modules/identity/...` — 11/11 + 2 test tích hợp mới pass; gate local (vet, golangci-lint, ewlint, archgen, entlint, tenancyguard, gofmt, unit) `CHECK OK`.

## 6. Bằng chứng nối binary trên stack (OQ F010-2, 2026-11-20, Windows 11 + Docker Desktop; compose project riêng `ewwire`)

Chuẩn bị (không có `make` trên máy dev nên chạy lệnh tương đương của Makefile): hạ tầng `docker compose -p ewwire -f deploy/compose/docker-compose.yml -f deploy/compose/docker-compose.app.yml -f deploy/compose/docker-compose.observability.yml up -d --wait postgres valkey nats keycloak apisix apisix-internal mailpit floci cdn sis-mock` + `up s3-init`; migrate 5 module bằng `cmd/migrate` (Atlas trong container, `EW_PORTAL_MENU_CATALOG` trỏ `specs/portal/menu-catalog.yaml`); realm `eduworld` + user/organization dev bằng `keycloak-config` + `post-apply.mjs` + `seed-dev.mjs`; ứng dụng `--profile app up -d --wait core-api realtime worker` (worker `--profile=all`). Volume `postgres-data` dùng chung của project `eduworld` trên máy dev có quyền bảng cũ (thiếu `GRANT` của `identity_app`/`identity_platform`, `pg_default_acl` rỗng) nên smoke chạy trên database mới của project `ewwire`; volume đó cần dựng lại (`make down` + xóa volume) hoặc cấp lại quyền — không phải lỗi của phần nối.

| Kiểm tra | Kết quả |
|---|---|
| `GET :9100/readyz` (`core-api`) | `200`: `atlas:*`, `db:*` của 5 module, `jwks`, `nats`, `valkey` đều `ok` |
| `GET :9200/readyz` (`worker --profile=all`), `GET :9106/readyz` (`realtime`) | `200` |
| `GET /api/v1/game/client-config?platform=windows&version=0.1.0` (route public module game) | `200`, `min_client_version` `0.1.0`, `login_url`, `supported_locales` từ `x-module-env` |
| `POST /api/v1/auth/game/login` `hs.an.3a` sai mật khẩu | `401 INVALID_CREDENTIALS` (password grant thật của `eduworld-game-backend` tới Keycloak) |
| `POST /api/v1/auth/game/login` `hs.an.3a` đúng mật khẩu | `403 ACCOUNT_NOT_PROVISIONED` + logout phiên Keycloak (`eduworld_keycloak_token_call_duration_seconds_count{op="game_logout"}`): đúng ADR-0063 vì module identity chưa có seed học sinh (`modules/identity/seed/README.md`); lần gọi đầu sau khi Keycloak vừa khởi động vượt `keycloak_token_call_timeout` 3 s → `503 UNAVAILABLE` (retryable) |
| `POST /api/v1/auth/portal/login` `sa.admin` + OTP (`totp.mjs`) | `403 PORTAL_ACCESS_STAFF_ONLY` (Keycloak OTP grant thành công, identity chưa có tài khoản nhân sự) |
| `GET /api/v1/me/tenants`, `/me/permissions` với access token thật của `eduworld-portal` (`ad.truong.a`) | `403 TENANT_MISMATCH`: token qua JWKS của `http://keycloak:8180/realms/eduworld`, resolver Principal (facade `resolvePrincipal`) trả "không phải thành viên" |
| Không token / token hỏng | `401 UNAUTHENTICATED` (`missing bearer token` / `malformed token`) |
| Outbox relay (worker `events`) | `identity.infra.outbox_events`: 159 `audit.recorded`, 159 có `published_at`; JetStream `EVT_IDENTITY`, `EVT_LEARNING`, `EVT_GAME`, `EVT_COMPETITION`, `EVT_CHAT`, `DLQ_*`, `IDENTITY_SIS_JOBS` được tạo |
| Consumer (`identity.audit_sink`, một durable mỗi stream) | `infra.processed_events` 437 dòng `identity.audit_sink`; `audit.audit_logs`: `auth.account_not_provisioned` 4, `auth.portal_login` (denied) 2, `authz.denied` 4, `cross_tenant.accessed` 427; `eduworld_events_consumed_total{consumer="identity.audit_sink",result="ok"}` 439; `infra.dead_letters` 0. Lần smoke đầu phát hiện lỗi nối (tên durable `identity.audit_sink@EVT_*` ≠ tên consumer) — đã sửa trong commit `fix(wiring)` |
| Job (worker `jobs`) | `eduworld_job_runs_total{result="ok"}` trên `:9200/metrics` cho cả 16 job (identity 11, learning 3, game 2 — mỗi job chạy ngay khi khởi động rồi theo chu kỳ), ví dụ `platform-saga` 8, `session-timeout` 5, `import-jobs` 8 sau ~2 phút |

Lệnh tự động của phần nối: `go test ./cmd/internal/wiring/` (`TestCoreAPIMountsEveryImplementedOperation`: mọi operation có trong `presentation/http` của identity, learning, game và trong spec module được mount đúng một lần dưới module sở hữu; `TestWorkerProfilesRegisterRelaysConsumersAndJobs`: relay, mọi consumer khai báo trong `presentation/consumer` + `infrastructure/messaging`, mọi job chạy đúng profile; `TestWorkerChatProfileLoadsChatOnly`), `go test ./pkg/module/` (pool theo role, vai trò theo binary/profile, hook audit theo module, khóa cursor).

Lúc đó đăng nhập game/portal chưa **thành công** vì module identity chưa có seed — đã giải quyết bằng seed dev (§6.1).

### 6.1. Seed identity và đăng nhập thành công (2026-11-20, compose project riêng `ewseed`)

Chuẩn bị như trên với `COMPOSE_PROJECT_NAME=ewseed` (Atlas qua bản sao `scripts/dev/atlas.sh` dùng mạng `ewseed_default`): hạ tầng → realm (`keycloak-config` + `post-apply.mjs` + `seed-dev.mjs`: 33 user, 4 Organization) → migrate 5 module → seed identity (lệnh của `make seed-identity`) → seed learning → `up-app` → `scripts/dev/seed-check.sh`. Không chèn dòng bằng SQL: mọi dữ liệu đi qua use case (Clarifications "seed identity").

| Kiểm tra | Kết quả |
|---|---|
| Seed identity lần 1 | `tenants=2 schools=3 sis_connections=2 csv_imports=3 students_linked=14 platform_staff=9 tenant_staff=6 grants=16 locked=1 changed=true`; DB: 3 tenant (gồm `common`), 4 trường, 16 lớp, 29 user, 29 `external_identities`, 16 `user_roles`, 14 enrollment, 3 sync run |
| Seed identity lần 2 và 3 | `changed=false` (mọi bộ đếm 0); `infra.outbox_events` giữ 203, số dòng không đổi |
| Seed learning | `subjects_created=3 curricula=1` |
| `GET :9100/readyz`, `GET :9200/readyz` | `200`, `200` |
| `make seed-check` | `ref.students complete (identity=14 learning=14 game=14)` |
| `POST /api/v1/auth/game/login` (gateway `:9080`) `hs.an.3a`, `hs.dung.7a`, `hs.em.4a` | `200` + token; `bootstrap.tenant_id` = `th-demo-a`, `pgd-demo`, `pgd-demo`; lớp `3A`, `7A`, `4A`. Lần gọi đầu mỗi user lúc máy tải cao có thể `503 UNAVAILABLE` (password grant vượt `keycloak_token_call_timeout` 3 s, retryable), gọi lại `200` |
| Game login `hs.locked` / `hs.nosis` | `403 ACCOUNT_LOCKED` / `403 ACCOUNT_NOT_PROVISIONED` |
| `POST /api/v1/game/sessions` (`hs.an.3a`, `X-Tenant-Id`, `X-Device-Id`, `Idempotency-Key`) | `201`, `game_session_id` trả về |
| `POST /api/v1/auth/portal/login` `ad.truong.a` | `200`; `GET /me/tenants` `200` [`th-demo-a`]; `GET /me/permissions` (`X-Tenant-Id` = `th-demo-a`) `200`: role `school_admin` scope trường `DEMOA1`, 31 permission |
| Portal login `sa.admin` không OTP / có OTP (`totp.mjs`) | `401 INVALID_CREDENTIALS` / `200`; `GET /me/permissions` (platform) `200`: `system_admin`, 123 permission; `GET /me/tenants` `200` `[]` |

Ghi chú của lần chạy:
- Smoke chạy lúc 04:50 ICT, trong `platform_curfew` mặc định 21:30–06:30: login game trả `403 CURFEW_ACTIVE` đúng thiết kế. Đã đổi giờ giới nghiêm bằng `putPlatformCurfewSettings` (`sa.admin`, `If-Match`, lý do) thành 21:30–04:00 cho module identity; module game không theo `platform_settings.changed` mà đọc `EW_PLATFORM_CURFEW` (`createGameSession` vẫn `403 CURFEW_ACTIVE`) nên `core-api` được tạo lại với override compose `EW_PLATFORM_CURFEW=21:30-04:00` chỉ trên stack `ewseed` [câu hỏi mở: game đọc giới nghiêm từ cài đặt platform].
- Route có bearer token qua APISIX (`:9080`, `:9180`) trả `401` từ gateway: `deploy/apisix/routes/env/local.env` có `EW_OIDC_ISSUERS=http://localhost:8180/...` trong khi token do backend lấy có `iss = http://keycloak:8180/...` (Clarifications "nối binary"); smoke gọi `core-api :8100` trực tiếp cho `/game/sessions`, `/me/*`, `/admin/settings/curfew` [câu hỏi mở: thêm issuer `keycloak:8180` vào allowlist của APISIX local].
- `sa.admin` `/me/tenants` = `[]` (System Admin không có membership; chọn tenant qua `listTenants`).

Lệnh tự động: `go test ./modules/identity/internal/presentation/devseed/` (plan khớp realm import, từ chối plan sai), `go test ./modules/identity/internal/infrastructure/keycloak/ -run DevAdopter`, `go test -tags integration ./modules/identity/tests/integration/ -run TestDevSeedIsIdempotent` (PostgreSQL mọi migration, seed hai lần, lần hai không ghi outbox/dòng nào).
