# Tasks: F016 — Vòng đời tài khoản: khóa, thu hồi truy cập, dọn học sinh không đăng nhập, ẩn danh hóa

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

> Keycloak khi dọn đã chốt (Technical Architect 2026-11-16): giữ user `enabled`, gỡ membership Organization, thu hồi phiên; đăng nhập lại → khôi phục.

## Phase 1: Setup

- [x] T001 Adapter facade `game`, `learning` cho identity qua `src/backend/pkg/modcall` (`Idempotency-Key`, Principal gốc, deadline; thử lại do engine saga, không retry trong adapter) — file: `src/backend/modules/identity/internal/infrastructure/modules/game.go`, `src/backend/modules/identity/internal/infrastructure/modules/learning.go` — **Done: `internal/infrastructure/participants/participants.go` (Game/Learning, GameAnonymize/LearningAnonymize qua modcall); TestParticipantsFacade pass**
- [x] T002 [P] Thêm permission `tenant.user.lock` vào catalog, gán `tenant_admin`, `school_admin` — file: `src/backend/pkg/authz/catalog.go` — **Done: `tenant.user.lock` trong catalog.go/roles.go, ADMIN_PORTAL §2.2; TestUserLifecycle (School Admin scope trường) pass**

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Thêm `unlockUser`, `deactivateUser`, `lockTenantUser`, `unlockTenantUser` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=identity` — **Done: 4 operation trong identity.v1.yaml; TestLifecycleOperationsMatchSpec pass**
- [x] T004 [P] Thêm payload schema `user.access_restored`, `student.cleanup_scheduled`, `student.restored`, `user.anonymized`, `student.cleaned_up` (F016 sở hữu; F013 T045 chỉ dùng fixture) — file: `specs/events/payloads/identity/` — kiểm: `make specs-lint` — **2026-11-17:** đã áp `student.cleaned_up`, `student.restored` (F013/F014 tiêu thụ qua fixture ở M-C1); còn `user.access_restored`, `student.cleanup_scheduled`, `user.anonymized` — **Done: payload user.access_restored, student.cleanup_scheduled, user.anonymized; validate-schemas 0 lỗi**
- [x] T048 Contract `resetStudentPassword` (`StudentPasswordResetRequest`/`StudentPasswordResetResult`, `x-portal-action: action.school_lms.classes.reset_student_password`), permission `lms.student.password_reset` (ADMIN_PORTAL §2.2), `user.access_revoked.reason` thêm `password_changed` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/events/payloads/identity/user.access_revoked.v1.schema.json` — **xong 2026-11-17** (quyết định Technical Architect; kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`)
- [x] T005 ent schema `CleanupExclusion`, `StudentCleanupRun`, `SagaRun`, `SagaStep` (nếu F045 chưa tạo), `DeletionRequest`, `ErasureLog`; cột `status_reason`, `status_changed_by` trên `identity.users` — file: `src/backend/modules/identity/ent/schema/` — kiểm: `make migrate-diff MOD=identity name=f016_lifecycle`, `make migrate-lint MOD=identity` — **Done (SQL viết tay thay ent, OQ F016-4): migration 20261125000001_f016_lifecycle.sql + 20261126000002_f016_data_requests.sql, RLS; tenancyguard pass**
- [x] T006 [P] Engine saga: chạy bước tuần tự, retry backoff, bù khi `blocked`/`failed`, idempotent theo `saga_id`, alert `SagaStuck` — file: `src/backend/modules/identity/internal/application/saga/orchestrator.go` — **Done: `internal/application/saga/engine.go`; TestSagaEngine pass (retry backoff, bù, ResumeDue, SagaStuck gauge)**

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Khóa, mở khóa, vô hiệu hóa và thu hồi truy cập (Priority: P1) 🎯 MVP

**Goal**: Tài khoản bị khóa không dùng được hệ thống trong giới hạn thời gian.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F016-AC-01, F016-AC-02: lock ghi status + audit + outbox cùng transaction; REST `403 ACCOUNT_LOCKED` ≤ 60 s ở identity và module game — file: `src/backend/modules/identity/tests/integration/lock_user_test.go` — phải FAIL trước khi code — **Done: TestUserLifecycle (lock → status + audit + outbox cùng transaction) pass**
- [ ] T036 [P] [US1] Test end-to-end F016-AC-24 (core-api + worker-events, module identity + game): khóa thật, SIS `suspended`, dọn thật học sinh đang có session + phiên game → session `account_locked`/`cleanup` (consumer F014), phiên game thu hồi, refresh → `401 REFRESH_TOKEN_INVALID` (consumer F013), tất cả ≤ 1 phút — file: `src/backend/test/e2e/identity/account_lifecycle_game_revoke_test.go` — phải FAIL trước khi code — Một phần: mỗi chặng đã kiểm (payload đúng schema đọc từ outbox; consumer F013/F014 có test fixture). Kiểm end-to-end core-api + worker chờ nối binary (OQ F010-2, OQ F016-7)
- [x] T008 [P] [US1] Test F016-AC-03, F016-AC-04: Keycloak `enabled=false` + logout ≤ 1 phút (retry khi Keycloak lỗi); unlock → `user.access_restored`, bật lại Keycloak — file: `src/backend/modules/identity/tests/integration/access_revoke_propagation_test.go` — **Done: TestUserLifecycle (consumer KeycloakAccess disable + logout, unlock enable) pass**
- [x] T009 [P] [US1] Test tenant isolation + authorization F016-AC-05, F016-AC-06 cho `lockTenantUser`, `unlockTenantUser`, `lockUser`, `unlockUser`, `deactivateUser`: trường khác/tenant khác → 403/404; khóa nhân sự bằng School Admin → 403; thiếu `reason` → 422; lý do trung tính với học sinh — file: `src/backend/modules/identity/tests/integration/lock_authz_test.go` — **Done: TestUserLifecycle (tenant khác 404, thiếu quyền 403) pass**
- [x] T046 [P] [US1] Test F016-AC-25, F016-AC-26, F016-AC-27: `resetStudentPassword` bởi School Admin/Tenant Admin/System Admin (`common`) → mật khẩu mới đăng nhập được, `temporary` theo khối, `initial_password` một lần + `no-store`, không có trong log/span/DB; replay → `password_redacted`; phiên game bị thu hồi (`password_changed`, refresh → `401 REFRESH_TOKEN_INVALID`), `user.access_revoked` không làm Keycloak `enabled=false`; học sinh gọi → 403, trường/tenant khác → 404, thiếu `reason` → 422 (TC-AUTH-087) — file: `src/backend/modules/identity/tests/integration/reset_student_password_test.go` — phải FAIL trước khi code — **Done: TestUserLifecycle (resetStudentPassword no-store, password_redacted khi replay, học sinh tự gọi 403, password_changed không disable) pass**

### Implementation cho User Story 1

- [x] T010 [P] [US1] Domain `UserStatus` (chuyển trạng thái hợp lệ) — file: `src/backend/modules/identity/internal/domain/lifecycle/status.go` — **Done ([Confirmed] bảng chuyển trạng thái `transitions` trong command/user_lifecycle.go, dùng chung cho staff; SIS dùng sis.StatusEffect)**
- [x] T011 [US1] Command `LockUser`, `UnlockUser`, `DeactivateUser` + handler — file: `src/backend/modules/identity/internal/application/command/user_lifecycle.go`, `src/backend/modules/identity/internal/presentation/http/public/user_lifecycle.go` — **Done: command/user_lifecycle.go + http/lifecycle.go**
- [x] T012 [US1] Consumer `keycloak-provisioner` nhánh `user.access_revoked` (disable + logout), `user.access_restored` (enable) — file: `src/backend/modules/identity/internal/presentation/consumer/keycloak_provisioner.go` — **Done: consumer/keycloak_access.go (access_revoked, access_restored, student.cleaned_up, user.anonymized)**
- [x] T047 [US1] Command `ResetStudentPassword` + handler (sinh mật khẩu, Keycloak Admin API `reset-password` với `temporary` theo khối, thu hồi `game_auth_sessions` `password_changed`, outbox `user.access_revoked`, audit, redaction `initial_password`, replay không mật khẩu); nhánh consumer `password_changed` không disable user — file: `src/backend/modules/identity/internal/application/command/reset_student_password.go`, `src/backend/modules/identity/internal/presentation/http/public/student_password.go` — **Done: command/student_password.go**
- [x] T013 [US1] Contract test producer: `LockUser`, apply SIS và saga dọn phát `user.status_changed`, `user.access_revoked`, `student.cleaned_up` đúng payload schema (`user.*` owner F011 T005, `student.cleaned_up` T004); consumer module game kết thúc session thuộc F014 T039 — file: `src/backend/modules/identity/tests/contract/lifecycle_events_producer_test.go` — **Done: payload sinh ra được đọc lại từ outbox và đưa vào consumer thật (TestSisStatusPropagation, TestUserAnonymize); validate-schemas pass**

## Phase 4: User Story 2 — Lan truyền trạng thái từ SIS (Priority: P1)

### Tests cho User Story 2

- [x] T014 [P] [US2] Test F016-AC-07, F016-AC-08: import SIS `suspended`/`transferred_out` → thu hồi như US1 trong một chu kỳ; session/đăng nhập sau bị từ chối — file: `src/backend/modules/identity/tests/integration/sis_status_propagation_test.go` — phải FAIL trước khi code — **Done: tests/integration/sis_status_propagation_integration_test.go TestSisStatusPropagation pass**

### Implementation cho User Story 2

- [x] T015 [US2] Bảo đảm apply SIS (F011) dùng cùng domain `UserStatus` và chuỗi event; reconciliation Keycloak cho trạng thái lệch — file: `src/backend/modules/identity/internal/application/command/sis_apply_status.go` — **Done: SisRun.History ghi identity.user_status_history (source sis), cùng event user.status_changed/user.access_revoked với US1**

## Phase 5: User Story 3 — Dọn học sinh không đăng nhập game (Priority: P2)

### Tests cho User Story 3

- [x] T016 [P] [US3] Test F016-AC-09, F016-AC-13: chọn ứng viên theo `COALESCE(last_login_at, provisioned_at)` và số ngày hiệu lực; soft-delete `inactive_cleanup`; user Keycloak vẫn `enabled`, không còn membership Organization, mọi phiên/refresh token bị thu hồi, token cũ bị từ chối ≤ 60 s (TC-AUTH-020); không câu `DELETE` (log truy vấn) — file: `src/backend/modules/identity/tests/integration/student_cleanup_test.go` — phải FAIL trước khi code — **Done: TestStudentCleanup (COALESCE(last_login_at, provisioned_at), số ngày tenant 90) pass**
- [x] T017 [P] [US3] Test F016-AC-10, F016-AC-11: báo trước + `student.cleanup_scheduled`; loại trừ có `reason` + audit được tôn trọng — file: `src/backend/modules/identity/tests/integration/cleanup_notice_exclusion_test.go` — **Done: TestStudentCleanup (student.cleanup_scheduled, preview, loại trừ có reason) pass**
- [x] T018 [P] [US3] Test F016-AC-12: vượt ngưỡng → `awaiting_approval` + alert; `approveStudentCleanupRun` chỉ `platform.student_cleanup.approve` — file: `src/backend/modules/identity/tests/integration/cleanup_approval_test.go` — **Done: TestStudentCleanup (awaiting_approval, chỉ platform.student_cleanup.approve duyệt) pass**
- [x] T019 [P] [US3] Test F016-AC-14, F016-AC-15: game trả `blocked` → bù `restoreStudentData`, `skipped_active`; tắt module game giữa saga → hội tụ, không chạy hai lần — file: `src/backend/modules/identity/tests/integration/cleanup_saga_test.go` — **Done: TestStudentCleanup (blocked → bù restoreStudentData, skipped_active) + TestSagaEngine (retry) pass**
- [x] T020 [P] [US3] Test tenant isolation + authorization F016-AC-16 cho `previewTenantStudentCleanup`, `createStudentCleanupExclusions`, `deleteStudentCleanupExclusions`, `listStudentCleanupRuns` — file: `src/backend/modules/identity/tests/integration/cleanup_authz_test.go` — **Done: TestStudentCleanup (preview chỉ tenant phiên, School Admin 403 khi loại trừ) pass**
- [x] T021 [P] [US3] Facade test game/learning `cleanupStudentData` (`done`/`blocked`/`nothing_to_do`, chỉ module `identity` theo `x-allowed-callers`, idempotent) — file: `src/backend/modules/game/tests/integration/platform_students_test.go`, `src/backend/modules/learning/tests/integration/platform_students_test.go` — **Done: TestStudentLifecycleFacade (game), TestPlatformStudentsFacade (learning), TestParticipantsFacade (identity) pass**

### Implementation cho User Story 3

- [x] T022 [P] [US3] Domain `CleanupPolicy`, `CleanupCandidate` — file: `src/backend/modules/identity/internal/domain/lifecycle/cleanup.go` — **Done ([Confirmed] CleanupCandidate.DueAt trong port/write/cleanup.go; chính sách từ settings.StudentCleanup)**
- [x] T023 [US3] Command `ScheduleCleanupNotices`, `RunStudentCleanup`, `ApproveStudentCleanupRun`, exclusions; query preview/runs; job `student-inactive-cleanup` — file: `src/backend/modules/identity/internal/application/command/student_cleanup.go`, `src/backend/modules/identity/internal/presentation/job/student_inactive_cleanup.go` — **Done: command/student_cleanup.go, student_cleanup_tenant.go; job/lifecycle.go**
- [x] T024 [US3] Endpoint internal `cleanupStudentData`, `restoreStudentData` ở module game và module learning — file: `src/backend/modules/game/internal/presentation/http/internal/platform_students.go`, `src/backend/modules/learning/internal/presentation/http/internal/platform_students.go` — **Done: game command/student_lifecycle.go + facade; learning facade/platform_students.go**
- [x] T025 [US3] Consumer `keycloak-provisioner` nhánh `student.cleaned_up`: gỡ membership Organization + logout mọi phiên, **không** đặt `enabled=false`; nhánh `student.restored`: thêm lại membership Organization — file: `src/backend/modules/identity/internal/presentation/consumer/keycloak_provisioner.go` — **Done: KeycloakAccess.CleanedUp (logout, giữ enabled; gỡ Organization qua tenant_membership.changed → worker F010)**

## Phase 6: User Story 4 — Khôi phục học sinh bị dọn (Priority: P2)

### Tests cho User Story 4

- [x] T026 [P] [US4] Test F016-AC-17, F016-AC-18: đăng nhập lại hợp lệ SIS (password grant thành công vì user Keycloak `enabled`) → saga khôi phục + `student.restored`, membership Organization được thêm lại, dữ liệu cũ dùng lại; SIS không hợp lệ → `ACCOUNT_DEACTIVATED` — file: `src/backend/modules/identity/tests/integration/restore_on_login_test.go` — phải FAIL trước khi code — **Done: tests/integration/student_restore_integration_test.go TestStudentRestore (đăng nhập lại → khôi phục; SIS inactive → ACCOUNT_DEACTIVATED) pass**
- [x] T027 [P] [US4] Test F016-AC-19: `restoreTenantTrashItem` (`student`) bởi Tenant Admin chạy cùng saga + audit; School Admin `403 PERMISSION_DENIED`; tenant khác 404 — file: `src/backend/modules/identity/tests/integration/restore_from_trash_test.go` — **Done: TestStudentRestore (RestoreStudentByAdmin: tenant khác/học sinh không bị dọn 404); route + School Admin 403 kiểm ở F045 TestPlatformAdminCrossTenantTrash**

### Implementation cho User Story 4

- [x] T028 [US4] Command `RestoreCleanedStudent` + nhánh trong `GameLogin` (F013) và handler Thùng rác tenant cho `student` — file: `src/backend/modules/identity/internal/application/command/restore_student.go` — **Done: command/student_restore.go + nhánh GameAuth.restoreOnLogin**

## Phase 7: User Story 5 — Ẩn danh hóa theo retention và yêu cầu (Priority: P3)

### Tests cho User Story 5

- [x] T029 [P] [US5] Test F016-AC-20, F016-AC-21: saga `user_anonymize` ghi đè PII, `anon.<user_id>`, Keycloak disable + ẩn danh, `anonymizeUserData` game/learning, erasure log, không `DELETE`, chỉ hoàn tất khi mọi bước `done` — file: `src/backend/modules/identity/tests/integration/anonymize_test.go` — phải FAIL trước khi code — **Done: tests/integration/anonymize_integration_test.go TestUserAnonymize (PII, anon.<user_id>, erasure log, Keycloak, không DELETE, retention) pass**
- [x] T030 [P] [US5] Test F016-AC-22 (authorization `platform.data_request.write`, `X-Tenant-Id` đích, audit mã yêu cầu) và F016-AC-23 (payload event/projection không chứa họ tên, ngày sinh, mã học sinh — kiểm schema) — file: `src/backend/modules/identity/tests/integration/data_request_test.go` — **Done: TestUserAnonymize (403 tenant admin, 404 tenant khác, payload user.anonymized chỉ định danh) pass**

### Implementation cho User Story 5

- [x] T031 [US5] Command `AnonymizeUser`, `CreateDataRequest`; job `retention-anonymizer` (Clock, thời hạn theo DATA_PRIVACY §2) — file: `src/backend/modules/identity/internal/application/command/anonymize.go`, `src/backend/modules/identity/internal/presentation/job/retention_anonymizer.go` — **Done: command/anonymize.go (UserAnonymize: CreateDataRequest, List/Get, RunRetention); job RetentionAnonymizer**
- [x] T032 [US5] Endpoint internal `anonymizeUserData` ở module game và module learning — file: `src/backend/modules/game/internal/presentation/http/internal/platform_users.go`, `src/backend/modules/learning/internal/presentation/http/internal/platform_users.go` — **Done (facade trong tiến trình theo ADR-0074, không HTTP): game facade.AnonymizeUserData, learning facade/platform_students.go; facade test pass**

## Phase 8: Polish & Cross-Cutting

- [x] T033 [P] Kiểm ACCOUNT_PROVISIONING §6b khớp quyết định Keycloak (giữ `enabled`; main agent đã sửa) và sửa thứ tự saga, §7b (`done/blocked/nothing_to_do`), IDENTITY_MAPPING §5 (không xóa Keycloak user), EVENT_DESIGN (close 4403, `user.anonymized`), ADMIN_PORTAL §2.2 (`tenant.user.lock`), CONFIG_DEFAULTS (tham số retention), CHANGELOG — file: `docs/09-identity/ACCOUNT_PROVISIONING.md` — **Done: ACCOUNT_PROVISIONING §6b/§6b.1/§7b, IDENTITY_MAPPING §5, EVENT_DESIGN (4403, user.anonymized) đã khớp; CONFIG_DEFAULTS student_data_retention có sẵn**
- [x] T034 [P] Metric `eduworld_student_cleanup_total`, `eduworld_saga_*`, `eduworld_access_revoke_latency_seconds`; alert `SagaStuck`, `StudentCleanupLargeBatch` — file: `src/backend/modules/identity/internal/infrastructure/observability/lifecycle_metrics.go` — **Done: internal/infrastructure/lifecyclemetrics (eduworld_student_cleanup_total{tenant,result}, eduworld_student_cleanup_large_batch_total, eduworld_saga_runs_total, eduworld_saga_stuck); alert SagaStuck, StudentCleanupLargeBatch theo MONITORING; TestMetrics pass**
- [x] T035 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **Done: quickstart §5 (bằng chứng test tự động; kịch bản thủ công chờ nối binary, OQ F016-7)**

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 → US5 → Polish.
- Module nhận saga (T024, T032) triển khai trước bên gọi (R29, AGENTS §3 bước 3).
- Consumer kết thúc session/thu hồi phiên game đã có từ F014 (T039), F013 (T040) ở M-C1; T036 chỉ kiểm end-to-end (CODE_PLAN §4.1).

## Definition of Done

- [ ] Mọi tiêu chí F016-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh cho identity, game, learning
- [ ] Constitution Check trong plan.md đạt (không còn ⚠️); không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
