# Tasks: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Khởi tạo package domain/application/infrastructure cho tenant, catalog, school, enrollment trong module identity — file: `src/backend/modules/identity/internal/` — **xong 2026-11-19 — `internal/application/port`, `internal/infrastructure/keycloak`; domain/command/query thêm theo từng story**
- [x] T002 [P] Cấu hình client Keycloak Admin (`EW_KC_ADMIN_BASE_URL`, `EW_KC_SYNC_CLIENT_ID`, `EW_KC_SYNC_CLIENT_SECRET_FILE`) qua `src/backend/pkg/config`, không hardcode — file: `src/backend/modules/identity/internal/infrastructure/keycloak/config.go` — **xong 2026-11-19 — tên biến theo KEYCLOAK_SSO §8/LOCAL_DEVELOPMENT §4: `EW_KC_ADMIN_BASE_URL`, `EW_KC_ADMIN_CLIENT_ID`, `EW_KC_ADMIN_CLIENT_SECRET(_FILE)` (client `svc-identity`, F004-AC-04), `EW_EXT_CLIENT_TIMEOUT/RETRY_MAX/CB_FAILURE_RATIO`**

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Thêm `createTenant`, `updateTenant`, `listAdminTenantSchools`, `createAdminTenantSchool`, `updateAdminTenantSchool`, `createCommonTenantStudents` (có `maxLength`, `Idempotency-Key`, `x-internal`) — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=identity` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); `createCommonTenantStudents` áp theo quyết định Technical Architect 2026-11-17 (ACCOUNT_PROVISIONING §3.4)
- [x] T004 [P] Thêm payload schema `tenant.created`, `tenant.updated`, `school.updated`, `class.created`, `class.updated`, `enrollment.changed`, `student.transferred`, `tenant_membership.changed`, `user.provisioned` — file: `specs/events/payloads/identity/` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T005 ent schema `Tenant`, `Country`, `Province`, `ProvinceSuccession`, `School`, `AcademicYear`, `Class`, `Student`, `Enrollment`, `User` với `MaxLen` theo data-model — file: `src/backend/modules/identity/ent/schema/` — kiểm: `make migrate-diff MOD=identity name=f010_org`, `make migrate-lint MOD=identity` — **xong 2026-11-19 — 11 entity; mixin entx có `PK` (tên cột khóa theo DATABASE_DESIGN); DDL bảng sinh bằng `entx.DDL` (golden test `modules/identity/ent/ddl_test.go`) thay `make migrate-diff` (Atlas `ent://` cần Go trong image)**
- [x] T006 Migration RLS (policy T, không DELETE), FK ghép `(class_id, school_id)`, unique partial, view `identity.tenant_memberships_v`, `REVOKE DELETE, TRUNCATE` — file: `src/backend/modules/identity/migrations/` — **xong 2026-11-19 — `20261119000001_f010_org.sql` (CHECK, FK ghép, index biểu thức, RLS 5 bảng T, `_live`, `identity.tenant_memberships_v` phần học sinh — F012 thêm nhân sự); `atlas.sum` bằng `scripts/db/atlas-sum.mjs` (khớp `atlas migrate hash` trên 5 thư mục)**
- [x] T007 Seed migration: danh mục từ `specs/reference-data`, tenant `common`, School ảo, lớp ảo `common-g1…g9` — file: `src/backend/modules/identity/migrations/` — kiểm: `node scripts/specs/validate-reference-data.mjs` — **xong 2026-11-19 — `20261119000002_f010_seed.sql` sinh bằng `scripts/db/gen-identity-seed.mjs` (`--check`); id tenant chung cố định**
- [x] T008 [P] Adapter Keycloak Admin (organizations, members, users, realm roles, logout user) sau port `KeycloakAdmin`, bọc timeout + retry idempotent — file: `src/backend/modules/identity/internal/infrastructure/keycloak/admin.go` — **xong 2026-11-19 — `admin.go` + 6 unit test (fake Keycloak): idempotent theo alias, retry chỉ thao tác idempotent, 4xx không retry, 401 làm mới token, mật khẩu không vào lỗi**
- [x] T009 [P] Test RLS mọi bảng T mới: `app.tenant_id` = A không đọc dòng B; không set → 0 dòng; `DELETE` bị từ chối — file: `src/backend/modules/identity/tests/integration/rls_org_test.go` — **2026-11-19: test viết xong (`tests/integration/org_schema_integration_test.go`, gồm cả F010-AC-02/06/11/12/16 tầng DB); chưa chạy — Docker Desktop dừng** — **xong 2026-11-19 — `tests/integration/org_schema_integration_test.go` pass trên PostgreSQL thật (RLS 5 bảng T, không DELETE, seed, bất biến DB)**

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — System Admin tạo và quản lý tenant (Priority: P1) 🎯 MVP

**Goal**: Tạo/sửa/bật-tắt tenant kèm Keycloak Organization.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Test F010-AC-01, F010-AC-02: tạo tenant ba loại, alias sai mẫu → 422, tenant `common` thứ hai → 409 — file: `src/backend/modules/identity/tests/integration/tenant_command_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `command/tenant_test.go` (unit) + `tests/integration/tenant_integration_test.go`**
- [x] T011 [P] [US1] Test F010-AC-03: tắt tenant → Organization disabled, resolve → `TENANT_ACCESS_DISABLED` — file: `src/backend/modules/identity/tests/integration/tenant_disable_test.go` — **xong 2026-11-19 — `command/org_sync_test.go` (Organization disabled khi tenant tắt); phần resolve → `TENANT_ACCESS_DISABLED` thuộc resolver F012**
- [x] T012 [P] [US1] Test authorization F010-AC-04 (role tenant, thiếu MFA, `azp` khác) và F010-AC-05 (thiếu `Idempotency-Key` → 428, replay cùng key → cùng kết quả, audit, chuỗi vượt `maxLength` → 422) — file: `src/backend/modules/identity/tests/integration/tenant_authz_test.go` — **xong 2026-11-19 — `tests/integration/http_integration_test.go` qua chuỗi httpx thật (403 thiếu MFA/role tenant, 428 thiếu key, replay, 422 maxLength, 428/412 If-Match)**

### Implementation cho User Story 1

- [x] T013 [P] [US1] Aggregate `Tenant` (loại, trạng thái, alias, invariant `common` duy nhất) — file: `src/backend/modules/identity/internal/domain/tenant/tenant.go` — **xong 2026-11-19 — `domain/tenant`**
- [x] T014 [US1] Command `CreateTenant`, `UpdateTenant`; query `ListTenants` (cursor, `q`, CrossTenantScope + audit) — file: `src/backend/modules/identity/internal/application/command/tenant.go` — **xong 2026-11-19 — `command/tenant.go`, `query/tenant.go` (cursor HMAC `pkg/page`)**
- [x] T015 [US1] Handler `createTenant`, `updateTenant`, `listTenants`, `listAdminTenantSchools`, `createAdminTenantSchool`, `updateAdminTenantSchool` — file: `src/backend/modules/identity/internal/presentation/http/public/admin_tenants.go` — **xong 2026-11-19 — `presentation/http` (Register + bảng Operations khớp spec, `tests/contract/routes_test.go`); mount vào core-api chờ resolver F012**
- [x] T016 [US1] Consumer `keycloak-membership` nhánh `tenant.created`/`tenant.updated`: tạo/cập nhật Organization, ghi `keycloak_org_id` — file: `src/backend/modules/identity/internal/presentation/consumer/keycloak_membership.go` — **xong 2026-11-19 — `command/org_sync.go` + consumer `identity.keycloak_membership`**

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Danh mục quốc gia, tỉnh/thành và trường (Priority: P1)

**Goal**: Danh mục chuẩn, import trường, duyệt trường.

**Independent Test**: Như spec.md US2.

### Tests cho User Story 2

- [x] T017 [P] [US2] Test F010-AC-06: seed đúng 34/63/kế thừa — file: `src/backend/modules/identity/tests/integration/catalog_seed_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `org_schema_integration_test.go` CatalogSeed (34/63/kế thừa)**
- [x] T018 [P] [US2] Test F010-AC-07, F010-AC-08, F010-AC-09: tỉnh inactive → 422; mã cũ ánh xạ + `province_outdated`; thiếu tỉnh → `pending_review`; duyệt → `active` + `school.updated` — file: `src/backend/modules/identity/tests/integration/school_import_test.go` — **xong 2026-11-19 — `school_integration_test.go`, `school_import_integration_test.go`**
- [x] T019 [P] [US2] Test tenant isolation + authorization F010-AC-10 cho `listTenantSchools`, `getTenantSchool`, `updateTenantSchool`, `approveTenantSchool`, `listCatalogSchools` (System Admin `tenant_ids` + audit `cross_tenant.accessed`; người dùng tenant gửi `tenant_ids` bị bỏ qua) — file: `src/backend/modules/identity/tests/integration/school_isolation_test.go` — **xong 2026-11-19 — `org_query_integration_test.go` (System Admin `tenant_ids` + audit `cross_tenant.accessed`)**

### Implementation cho User Story 2

- [x] T020 [P] [US2] Domain `catalog` (Province hiệu lực, Succession) và `school.School` (tỉnh active, trạng thái) — file: `src/backend/modules/identity/internal/domain/catalog/`, `src/backend/modules/identity/internal/domain/school/` — **xong 2026-11-19 — `domain/catalog`, `domain/school`**
- [x] T021 [US2] Command catalog write, `ImportSchools` (dry-run, bất đồng bộ qua `IDENTITY_SIS_JOBS`), `UpdateTenantSchool`, `ApproveTenantSchool`; query catalog — file: `src/backend/modules/identity/internal/application/command/school.go` — **xong 2026-11-19 phần use case — `command/catalog.go` (5 thao tác catalog), `command/school_import.go` (`ImportSchools` theo lô, dry-run); job bất đồng bộ `importSchools`/`getSchoolImport` (202, lưu file, `IDENTITY_SIS_JOBS`) chờ bảng `sis.sis_imports` của F011 — Câu hỏi mở**
- [x] T022 [US2] Handler catalog, `importSchools`, `getSchoolImport`, `/tenant/schools*` — file: `src/backend/modules/identity/internal/presentation/http/public/catalog_schools.go` — **xong 2026-11-19 phần đồng bộ — handler catalog + `/tenant/schools*`; `importSchools`/`getSchoolImport` chưa mount (xem T021)**

## Phase 5: User Story 3 — Năm học, lớp, enrollment và chuyển lớp/chuyển trường (Priority: P1)

**Goal**: Một trường, một lớp; chuyển lớp/trường giữ lịch sử.

**Independent Test**: Như spec.md US3.

### Tests cho User Story 3

- [x] T023 [P] [US3] Test F010-AC-11, F010-AC-12: hai enrollment active đồng thời → một thành công; lớp khác trường → từ chối — file: `src/backend/modules/identity/tests/integration/enrollment_invariant_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `org_schema_integration_test.go` EnrollmentInvariants + `transfer_integration_test.go` (đồng thời)**
- [x] T024 [P] [US3] Test F010-AC-13, F010-AC-14, F010-AC-15: chuyển lớp, chuyển trường cùng tenant `unit`, chuyển tenant phát `tenant_membership.changed` — file: `src/backend/modules/identity/tests/integration/transfer_test.go` — **xong 2026-11-19 — `transfer_integration_test.go`**
- [x] T025 [P] [US3] Test F010-AC-16: `user_type` ngoài `student`/`staff` bị CHECK chặn; học sinh thiếu `primary_tenant_id` bị chặn — file: `src/backend/modules/identity/tests/integration/user_type_test.go` — **xong 2026-11-19 — `org_schema_integration_test.go` UserChecks**
- [x] T026 [P] [US3] Test tenant isolation + authorization F010-AC-17, F010-AC-18, F010-AC-19 cho `listClasses`, `listTenantSchoolClasses`, `listUnassignedStudents`, `getStudentProfile`, `getMyStudentProfile` (sinh từ OpenAPI) — file: `src/backend/modules/identity/tests/integration/org_isolation_test.go` — **xong 2026-11-19 — `org_query_integration_test.go`**
- [x] T042 [P] [US3] Facade test F010-AC-27: phân trang ổn định theo cursor, `source_version` khớp event, cờ `deleted`, chỉ tenant yêu cầu, không trường PII ngoài payload event; module ngoài `x-allowed-callers` bị `make arch-check` chặn — file: `src/backend/modules/identity/tests/contract/list_projection_snapshot_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `snapshot_integration_test.go` (khóa `data` ⊆ payload event)**

### Implementation cho User Story 3

- [x] T027 [P] [US3] Aggregate `Enrollment` + domain service `TransferStudent` (đóng/mở cùng transaction) — file: `src/backend/modules/identity/internal/domain/enrollment/` — **xong 2026-11-19 — `domain/enrollment`**
- [x] T028 [US3] Command `TransferStudent` (dùng bởi F011 apply); query `ListClasses`, `ListUnassignedStudents`, `GetStudentProfile` (audit), `GetMyStudentProfile` — file: `src/backend/modules/identity/internal/application/` — **xong 2026-11-19 — `command/transfer.go`, `query/org.go`; audit đọc hồ sơ học sinh chưa làm (read context không ghi outbox) — [TBD]**
- [x] T029 [US3] Handler `listClasses`, `listTenantSchoolClasses`, `listUnassignedStudents`, `getStudentProfile`, `getMyStudentProfile` — file: `src/backend/modules/identity/internal/presentation/http/public/org.go` — **xong 2026-11-19 — `presentation/http/handlers.go`**
- [x] T043 [US3] Contract `listProjectionSnapshot` trong `identity.internal.v1.yaml` (schema item theo payload event `tenant.*`, `school.*`, `class.synced`, `student.synced`, `enrollment.changed`) + query `ListProjectionSnapshot` (read context) + facade — file: `specs/openapi/modules/identity.internal.v1.yaml`, `src/backend/modules/identity/internal/application/query/list_projection_snapshot.go`, `src/backend/modules/identity/api/` — kiểm: `make specs-lint`, `make gen MOD=identity`, `make arch-check` — **2026-11-17:** phần contract đã áp (`listProjectionSnapshot` trong `identity.internal.v1.yaml`, schema `ProjectionSnapshotItem`/`ProjectionSnapshotPage`); còn query `ListProjectionSnapshot` + facade — **xong 2026-11-19 — `query/snapshot.go`, `presentation/facade`**

## Phase 6: User Story 4 — Membership Keycloak Organization đồng bộ tự động (Priority: P1)

**Goal**: Membership Keycloak luôn khớp DB.

**Independent Test**: Như spec.md US4.

### Tests cho User Story 4

- [x] T030 [P] [US4] Test F010-AC-20: thứ tự gỡ → thêm → logout, phát `user.access_revoked` (Keycloak testcontainer) — file: `src/backend/modules/identity/tests/integration/membership_sync_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `membership_integration_test.go` (thứ tự gỡ → thêm → logout, `user.access_revoked`)**
- [ ] T031 [P] [US4] Test F010-AC-21: `user_role.changed` thêm/gỡ Organization và realm role `tenant_staff`/`platform_staff` — file: `src/backend/modules/identity/tests/integration/membership_staff_test.go` — **một phần** — đã có: `command/membership_sync.go` đồng bộ thành viên tổ chức; còn lại: gán/gỡ realm role `tenant_staff`/`platform_staff` theo UserRole (cần bảng `identity.user_roles` của F012)
- [x] T032 [P] [US4] Test F010-AC-22, F010-AC-23: reconciliation gỡ member thừa + issue; Keycloak lỗi → retry, DLQ, consumer idempotent — file: `src/backend/modules/identity/tests/integration/membership_reconcile_test.go` — **xong 2026-11-19 — `membership_integration_test.go` TestReconcileOrganizations + `admin_test.go` (retry/không ack khi Keycloak lỗi)**

### Implementation cho User Story 4

- [x] T033 [US4] Consumer `keycloak-membership` nhánh `tenant_membership.changed`, `user_role.changed`, `user.status_changed` — file: `src/backend/modules/identity/internal/presentation/consumer/keycloak_membership.go` — **xong 2026-11-19 — consumer `tenant_membership.changed`, `user_role.changed` (F012 phát); `user.status_changed` thuộc F016**
- [ ] T034 [US4] Job `org-membership-reconcile` (hằng đêm, lệnh `--once`) — file: `src/backend/modules/identity/internal/presentation/job/org_membership_reconcile.go` — **một phần** — use case `command/org_reconcile.go` xong + test; issue `unexpected_org_member` ghi bằng audit `reconciliation_issue.raised` (bảng `sis.reconciliation_issues` của F011); lịch 24 h đã nối (OQ F010-2); lệnh `--once` và ghi issue vào bảng → task bổ sung "OQ chốt 2026-11-20"

## Phase 7: User Story 5 — Tenant chung và tài khoản học sinh do platform tạo (Priority: P2)

**Goal**: Học sinh tenant chung có lớp ảo theo khối.

**Independent Test**: Như spec.md US5.

### Tests cho User Story 5

- [x] T035 [P] [US5] Test F010-AC-24, F010-AC-25: tạo học sinh khối 4 → `common-g4`, member `common`, audit; thiếu khối → 422 — file: `src/backend/modules/identity/tests/integration/common_tenant_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `common_tenant_integration_test.go`**
- [x] T044 [P] [US5] Test F010-AC-28, F010-AC-29: lô 3 dòng (1 trùng username → `failed` `USERNAME_TAKEN`, không reconciliation issue), `200` + số đếm; `initial_password` chỉ ở lần đầu, `Cache-Control: no-store`, không có trong log/span/DB; khối 6 → Keycloak `temporary = true`, khối 4 → `false`; replay cùng `Idempotency-Key` → không `initial_password`, `password_redacted = true`; quá `common_student_batch_max` dòng → 422 — file: `src/backend/modules/identity/tests/integration/common_tenant_batch_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `common_tenant_integration_test.go` + `http_integration_test.go`**
- [x] T036 [P] [US5] Test F010-AC-26: quét OpenAPI không có route tự đăng ký/tạo học sinh với `azp = eduworld-game-backend` — file: `src/backend/modules/identity/tests/contract/no_self_registration_test.go` — **xong 2026-11-19 — `tests/contract/no_self_registration_test.go`**

### Implementation cho User Story 5

- [x] T037 [US5] Command `CreateCommonTenantStudents` (từng dòng độc lập; chuẩn hóa username; sinh mật khẩu ngẫu nhiên, Keycloak user local `temporary` theo khối, role `student`, member `common`, `external_identities` (gồm dòng `issuer = platform_common` cho `external_ref`, trùng → `IDENTITY_CONFLICT` theo dòng), enrollment lớp ảo, outbox `user.provisioned`; kết quả lưu idempotency không có mật khẩu; đọc `common_student_batch_max`) — file: `src/backend/modules/identity/internal/application/command/common_tenant.go` — **xong 2026-11-19 — `command/common_tenant.go`**
- [x] T038 [US5] Handler `createCommonTenantStudents` (`Cache-Control: no-store`, redaction `initial_password` trong log/trace, replay đặt `password_redacted`) — file: `src/backend/modules/identity/internal/presentation/http/public/admin_common_tenant.go` — **xong 2026-11-19 — handler `createCommonTenantStudents` (`Cache-Control: no-store`, replay `password_redacted`)**

## Phase 8: Polish & Cross-Cutting

- [x] T039 [P] Cập nhật DATABASE_DESIGN nếu cột/enum thay đổi (`enrollments.source` thêm `platform_common`), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md` — **xong 2026-11-19 — DATABASE_DESIGN (trạng thái trường, `enrollments.source`, `effective_from` nullable, view `security_invoker`, quyền infra `_platform`), ADR-0089 (làm rõ), MONITORING, CHANGELOG**
- [x] T040 [P] Metric `eduworld_identity_org_sync_total`, `eduworld_auth_common_tenant_total`; span Keycloak — file: `src/backend/modules/identity/internal/infrastructure/keycloak/metrics.go` — **xong 2026-11-19 — `eduworld_identity_org_sync_total{op,result}` trong adapter (test `TestOrgSyncMetric`), `eduworld_auth_common_tenant_total{result}` ở handler; MONITORING cập nhật**
- [x] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **xong 2026-11-19 — bằng chứng test tự động ở quickstart §5; kịch bản trên stack chờ OQ F010-2**; 2026-11-20: binary đã nối, smoke trên stack ở quickstart §6 (kịch bản thành công cần seed identity — chưa có)

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T045 **OQ chốt 2026-11-20** (audit đọc hồ sơ) `getStudentProfile` ghi audit `student_profile.viewed` qua `audit.Recorder`; test có audit khi đọc thành công, không audit khi 403/404 — file: `src/backend/modules/identity/internal/application/query/`, `tests/integration/` — kiểm: `make test-integration MOD=identity` — cỡ S
- [ ] T046 **OQ chốt 2026-11-20** (T034) Lệnh `--once` cho job `org-membership-reconcile` và ghi issue `unexpected_org_member` vào `sis.reconciliation_issues` — file: `src/backend/cmd/worker/`, `src/backend/modules/identity/internal/application/command/org_reconcile.go` — kiểm: integration — cỡ S

## Dependencies & Execution Order

- Setup → Foundational (T003–T009) → US1 → US2, US3 song song → US4 (cần T008, T016) → US5 (cần T008, T027) → Polish.
- T028 `TransferStudent` là port dùng lại bởi F011 (apply SIS).

## Definition of Done

- [ ] Mọi tiêu chí F010-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `identity`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
