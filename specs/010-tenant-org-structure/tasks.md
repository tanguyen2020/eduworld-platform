# Tasks: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Khởi tạo package domain/application/infrastructure cho tenant, catalog, school, enrollment trong module identity — file: `modules/identity/internal/`
- [ ] T002 [P] Cấu hình client Keycloak Admin (`EW_KC_ADMIN_BASE_URL`, `EW_KC_SYNC_CLIENT_ID`, `EW_KC_SYNC_CLIENT_SECRET_FILE`) qua `pkg/config`, không hardcode — file: `modules/identity/internal/infrastructure/keycloak/config.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Thêm `createTenant`, `updateTenant`, `listAdminTenantSchools`, `createAdminTenantSchool`, `updateAdminTenantSchool`, `createCommonTenantStudents` (có `maxLength`, `Idempotency-Key`, `x-internal`) — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=identity`
- [ ] T004 [P] Thêm payload schema `tenant.created`, `tenant.updated`, `school.updated`, `class.created`, `class.updated`, `enrollment.changed`, `student.transferred`, `tenant_membership.changed`, `user.provisioned` — file: `specs/events/payloads/identity/` — kiểm: `make specs-lint`
- [ ] T005 ent schema `Tenant`, `Country`, `Province`, `ProvinceSuccession`, `School`, `AcademicYear`, `Class`, `Student`, `Enrollment`, `User` với `MaxLen` theo data-model — file: `modules/identity/ent/schema/` — kiểm: `make migrate-diff MOD=identity name=f010_org`, `make migrate-lint MOD=identity`
- [ ] T006 Migration RLS (policy T, không DELETE), FK ghép `(class_id, school_id)`, unique partial, view `identity.tenant_memberships_v`, `REVOKE DELETE, TRUNCATE` — file: `modules/identity/migrations/`
- [ ] T007 Seed migration: danh mục từ `specs/reference-data`, tenant `common`, School ảo, lớp ảo `common-g1…g9` — file: `modules/identity/migrations/` — kiểm: `node scripts/specs/validate-reference-data.mjs`
- [ ] T008 [P] Adapter Keycloak Admin (organizations, members, users, realm roles, logout user) sau port `KeycloakAdmin`, bọc timeout + retry idempotent — file: `modules/identity/internal/infrastructure/keycloak/admin.go`
- [ ] T009 [P] Test RLS mọi bảng T mới: `app.tenant_id` = A không đọc dòng B; không set → 0 dòng; `DELETE` bị từ chối — file: `modules/identity/tests/integration/rls_org_test.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — System Admin tạo và quản lý tenant (Priority: P1) 🎯 MVP

**Goal**: Tạo/sửa/bật-tắt tenant kèm Keycloak Organization.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Test F010-AC-01, F010-AC-02: tạo tenant ba loại, alias sai mẫu → 422, tenant `common` thứ hai → 409 — file: `modules/identity/tests/integration/tenant_command_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Test F010-AC-03: tắt tenant → Organization disabled, resolve → `TENANT_ACCESS_DISABLED` — file: `modules/identity/tests/integration/tenant_disable_test.go`
- [ ] T012 [P] [US1] Test authorization F010-AC-04 (role tenant, thiếu MFA, `azp` khác) và F010-AC-05 (thiếu `Idempotency-Key` → 428, replay cùng key → cùng kết quả, audit, chuỗi vượt `maxLength` → 422) — file: `modules/identity/tests/integration/tenant_authz_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Aggregate `Tenant` (loại, trạng thái, alias, invariant `common` duy nhất) — file: `modules/identity/internal/domain/tenant/tenant.go`
- [ ] T014 [US1] Command `CreateTenant`, `UpdateTenant`; query `ListTenants` (cursor, `q`, CrossTenantScope + audit) — file: `modules/identity/internal/application/command/tenant.go`
- [ ] T015 [US1] Handler `createTenant`, `updateTenant`, `listTenants`, `listAdminTenantSchools`, `createAdminTenantSchool`, `updateAdminTenantSchool` — file: `modules/identity/internal/presentation/http/public/admin_tenants.go`
- [ ] T016 [US1] Consumer `keycloak-membership` nhánh `tenant.created`/`tenant.updated`: tạo/cập nhật Organization, ghi `keycloak_org_id` — file: `modules/identity/internal/presentation/consumer/keycloak_membership.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Danh mục quốc gia, tỉnh/thành và trường (Priority: P1)

**Goal**: Danh mục chuẩn, import trường, duyệt trường.

**Independent Test**: Như spec.md US2.

### Tests cho User Story 2

- [ ] T017 [P] [US2] Test F010-AC-06: seed đúng 34/63/kế thừa — file: `modules/identity/tests/integration/catalog_seed_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Test F010-AC-07, F010-AC-08, F010-AC-09: tỉnh inactive → 422; mã cũ ánh xạ + `province_outdated`; thiếu tỉnh → `pending_review`; duyệt → `active` + `school.updated` — file: `modules/identity/tests/integration/school_import_test.go`
- [ ] T019 [P] [US2] Test tenant isolation + authorization F010-AC-10 cho `listTenantSchools`, `getTenantSchool`, `updateTenantSchool`, `approveTenantSchool`, `listCatalogSchools` (System Admin `tenant_ids` + audit `cross_tenant.accessed`; người dùng tenant gửi `tenant_ids` bị bỏ qua) — file: `modules/identity/tests/integration/school_isolation_test.go`

### Implementation cho User Story 2

- [ ] T020 [P] [US2] Domain `catalog` (Province hiệu lực, Succession) và `school.School` (tỉnh active, trạng thái) — file: `modules/identity/internal/domain/catalog/`, `modules/identity/internal/domain/school/`
- [ ] T021 [US2] Command catalog write, `ImportSchools` (dry-run, bất đồng bộ qua `IDENTITY_SIS_JOBS`), `UpdateTenantSchool`, `ApproveTenantSchool`; query catalog — file: `modules/identity/internal/application/command/school.go`
- [ ] T022 [US2] Handler catalog, `importSchools`, `getSchoolImport`, `/tenant/schools*` — file: `modules/identity/internal/presentation/http/public/catalog_schools.go`

## Phase 5: User Story 3 — Năm học, lớp, enrollment và chuyển lớp/chuyển trường (Priority: P1)

**Goal**: Một trường, một lớp; chuyển lớp/trường giữ lịch sử.

**Independent Test**: Như spec.md US3.

### Tests cho User Story 3

- [ ] T023 [P] [US3] Test F010-AC-11, F010-AC-12: hai enrollment active đồng thời → một thành công; lớp khác trường → từ chối — file: `modules/identity/tests/integration/enrollment_invariant_test.go` — phải FAIL trước khi code
- [ ] T024 [P] [US3] Test F010-AC-13, F010-AC-14, F010-AC-15: chuyển lớp, chuyển trường cùng tenant `unit`, chuyển tenant phát `tenant_membership.changed` — file: `modules/identity/tests/integration/transfer_test.go`
- [ ] T025 [P] [US3] Test F010-AC-16: `user_type` ngoài `student`/`staff` bị CHECK chặn; học sinh thiếu `primary_tenant_id` bị chặn — file: `modules/identity/tests/integration/user_type_test.go`
- [ ] T026 [P] [US3] Test tenant isolation + authorization F010-AC-17, F010-AC-18, F010-AC-19 cho `listClasses`, `listTenantSchoolClasses`, `listUnassignedStudents`, `getStudentProfile`, `getMyStudentProfile` (sinh từ OpenAPI) — file: `modules/identity/tests/integration/org_isolation_test.go`
- [ ] T042 [P] [US3] Facade test F010-AC-27: phân trang ổn định theo cursor, `source_version` khớp event, cờ `deleted`, chỉ tenant yêu cầu, không trường PII ngoài payload event; module ngoài `x-allowed-callers` bị `make arch-check` chặn — file: `modules/identity/tests/contract/list_projection_snapshot_test.go` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T027 [P] [US3] Aggregate `Enrollment` + domain service `TransferStudent` (đóng/mở cùng transaction) — file: `modules/identity/internal/domain/enrollment/`
- [ ] T028 [US3] Command `TransferStudent` (dùng bởi F011 apply); query `ListClasses`, `ListUnassignedStudents`, `GetStudentProfile` (audit), `GetMyStudentProfile` — file: `modules/identity/internal/application/`
- [ ] T029 [US3] Handler `listClasses`, `listTenantSchoolClasses`, `listUnassignedStudents`, `getStudentProfile`, `getMyStudentProfile` — file: `modules/identity/internal/presentation/http/public/org.go`
- [ ] T043 [US3] Contract `listProjectionSnapshot` trong `identity.internal.v1.yaml` (schema item theo payload event `tenant.*`, `school.*`, `class.synced`, `student.synced`, `enrollment.changed`) + query `ListProjectionSnapshot` (read context) + facade — file: `specs/openapi/modules/identity.internal.v1.yaml`, `modules/identity/internal/application/query/list_projection_snapshot.go`, `modules/identity/api/` — kiểm: `make specs-lint`, `make gen MOD=identity`, `make arch-check`

## Phase 6: User Story 4 — Membership Keycloak Organization đồng bộ tự động (Priority: P1)

**Goal**: Membership Keycloak luôn khớp DB.

**Independent Test**: Như spec.md US4.

### Tests cho User Story 4

- [ ] T030 [P] [US4] Test F010-AC-20: thứ tự gỡ → thêm → logout, phát `user.access_revoked` (Keycloak testcontainer) — file: `modules/identity/tests/integration/membership_sync_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US4] Test F010-AC-21: `user_role.changed` thêm/gỡ Organization và realm role `tenant_staff`/`platform_staff` — file: `modules/identity/tests/integration/membership_staff_test.go`
- [ ] T032 [P] [US4] Test F010-AC-22, F010-AC-23: reconciliation gỡ member thừa + issue; Keycloak lỗi → retry, DLQ, consumer idempotent — file: `modules/identity/tests/integration/membership_reconcile_test.go`

### Implementation cho User Story 4

- [ ] T033 [US4] Consumer `keycloak-membership` nhánh `tenant_membership.changed`, `user_role.changed`, `user.status_changed` — file: `modules/identity/internal/presentation/consumer/keycloak_membership.go`
- [ ] T034 [US4] Job `org-membership-reconcile` (hằng đêm, lệnh `--once`) — file: `modules/identity/internal/presentation/job/org_membership_reconcile.go`

## Phase 7: User Story 5 — Tenant chung và tài khoản học sinh do platform tạo (Priority: P2)

**Goal**: Học sinh tenant chung có lớp ảo theo khối.

**Independent Test**: Như spec.md US5.

### Tests cho User Story 5

- [ ] T035 [P] [US5] Test F010-AC-24, F010-AC-25: tạo học sinh khối 4 → `common-g4`, member `common`, audit; thiếu khối → 422 — file: `modules/identity/tests/integration/common_tenant_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US5] Test F010-AC-26: quét OpenAPI không có route tự đăng ký/tạo học sinh với `azp = eduworld-game-backend` — file: `modules/identity/tests/contract/no_self_registration_test.go`

### Implementation cho User Story 5

- [ ] T037 [US5] Command `CreateCommonTenantStudents` (Keycloak user local, role `student`, member `common`, `external_identities`, enrollment lớp ảo, outbox `user.provisioned`) — file: `modules/identity/internal/application/command/common_tenant.go`
- [ ] T038 [US5] Handler `createCommonTenantStudents` — file: `modules/identity/internal/presentation/http/public/admin_common_tenant.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T039 [P] Cập nhật DATABASE_DESIGN nếu cột/enum thay đổi (`enrollments.source` thêm `platform_common`), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md`
- [ ] T040 [P] Metric `eduworld_identity_org_sync_total`, `eduworld_auth_common_tenant_total`; span Keycloak — file: `modules/identity/internal/infrastructure/keycloak/metrics.go`
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T003–T009) → US1 → US2, US3 song song → US4 (cần T008, T016) → US5 (cần T008, T027) → Polish.
- T028 `TransferStudent` là port dùng lại bởi F011 (apply SIS).

## Definition of Done

- [ ] Mọi tiêu chí F010-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `identity`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
