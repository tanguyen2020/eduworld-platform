# Tasks: F045 — Platform Administration backend: saga gán/clone, thùng rác mọi tenant, đọc liên tenant

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo module `platformadmin` (domain/application/infrastructure/presentation) và đăng ký route, consumer, job trong composition root — file: `src/backend/modules/identity/cmd/api/main.go`, `src/backend/modules/identity/cmd/worker/main.go` — ✅ 2026-11-19: Hiện thực trong module identity hiện có (không tạo module con): domain `internal/domain/platformadmin`, command `platform_admin.go`/`platform_sagas.go`, query `platform_admin.go`, HTTP `presentation/http/platform_admin.go` (`PlatformAdminOperations` + `RegisterPlatformAdmin`), consumer `default_assignments.go`, job `platform_saga.go`; binary chưa nối (OQ F010-2) — kiểm bằng httptest
- [x] T002 [P] Nối facade platform ops của learning, game, competition, chat vào module identity ở composition root (`module.go`, không URL/env, không token) và bảng `resource_type` → module — file: `src/backend/modules/identity/module.go`, `src/backend/modules/identity/internal/infrastructure/modules/registry.go` — ✅ 2026-11-19: Bảng định tuyến `resource_type` → module ở `domain/platformadmin/routing.go`; adapter `infrastructure/modules/owner_gateway.go` nhận facade learning (nil = chưa nối → 503); composition root chưa có (OQ F010-2)
- [x] T003 [P] Sinh facade platform ops từ spec internal của 4 module (vào `src/backend/modules/<module>/api`) — file: `src/backend/modules/identity/internal/infrastructure/modules/gen.go` — kiểm: `make gen MOD=identity` — ✅ 2026-11-19: Facade đã sinh sẵn (`modules/learning/api`, F020); identity chỉ import `learningapi`; `go run ./tools/archgen --check` → 0 violations

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract internal: `assignTenantResources`, `unassignTenantResources`, `listTenantResourceAssignments` trong interface chung, đồng bộ sang `<module>.internal.v1.yaml` — file: `specs/openapi/modules/common/internal-common.v1.yaml`, `specs/openapi/modules/{learning,game,competition,chat}.internal.v1.yaml` — kiểm: `node scripts/openapi/sync-platform-ops.mjs`, `make specs-lint` — ✅ 2026-11-19: Đã có từ F020 với tên `applyTenantAssignments`/`revokeTenantAssignments`/`listResourceAssignments` (Clarifications 2026-11-19)
- [x] T005 Delta contract public: `mergeUsers` (`POST /admin/users:merge`), `getUserMerge` (`GET /admin/user-merges/{merge_id}`), schema `UserMergeCommand`, `UserMergeJob` theo contracts §6; body `RestoreRequest {reason}` cho `restoreTrashItem`/`restoreTenantTrashItem`, `TenantCloneJob.errors[]`, thống nhất `ResourceType` (thêm `class`, `party` [Confirmed]) — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity` — ✅ 2026-11-19: `eduworld-api.v1.yaml` + `split-public.mjs`: `RestoreRequest`, `mergeUsers`, `getUserMerge`, `UserMergeCommand`, `UserMergeJob`, `ResourceType` + `user`/`student`, `maxItems: 100` cho `tenant_ids`, 503/404/422; `check-maxlength` 0 vi phạm, `split-public --check` khớp, portal `pnpm gen:api`
- [x] T006 [P] Payload schema `tenant_data.assigned.v1`, `tenant_data.unassigned.v1`, `tenant_data.cloned.v1` — file: `specs/events/payloads/identity/tenant_data.assigned.v1.schema.json`, `specs/events/payloads/identity/tenant_data.unassigned.v1.schema.json`, `specs/events/payloads/identity/tenant_data.cloned.v1.schema.json` — kiểm: `make specs-lint` — ✅ 2026-11-19: 3 schema payload trong `specs/events/payloads/identity/`; `validate-schemas.mjs` 0 lỗi, `check-cross-features.mjs` 0 lỗi
- [x] T007 ent schema `platform.tenant_clone_jobs` (+ `saga_id`), `platform.saga_runs` (+ `next_run_at`, giá trị `tenant_assign`/`tenant_unassign`), `platform.saga_steps`, `platform.tenant_default_assignments` (+ PK) — file: `src/backend/modules/identity/ent/schema/tenant_clone_job.go`, `src/backend/modules/identity/ent/schema/saga_run.go`, `src/backend/modules/identity/ent/schema/saga_step.go`, `src/backend/modules/identity/ent/schema/tenant_default_assignment.go` — kiểm: `make migrate-diff MOD=identity name=f045_platform_admin`, `make migrate-lint MOD=identity` — ✅ 2026-11-19: Migration viết tay `20261126000001_f045_platform_admin.sql` (`tenant_clone_jobs`, `tenant_default_assignments` + `_live`, `identity.user_merges` + RLS, CHECK `resolution`); bảng saga dùng của F016; `atlas.sum` sinh lại, `go test ./modules/identity/ent/` pass
- [x] T008 [P] Cập nhật DATABASE_DESIGN §2.0 (cột/giá trị mới của saga, clone job, default assignment) — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ 2026-11-19: DATABASE_DESIGN §2.0 (`tenant_default_assignments`, `tenant_clone_jobs`) và §2.16 (`user_merges`)
- [x] T009 Domain `ResourceRouting` (resource_type → module, hỗ trợ assign/clone/trash) và `ClonePlan` (allowlist, thứ tự bước) — file: `src/backend/modules/identity/internal/domain/platformadmin/resource_routing.go`, `src/backend/modules/identity/internal/domain/platformadmin/clone_plan.go` — ✅ 2026-11-19: `routing.go`, `clone.go` (+ `platformadmin_test.go`: TestRouting, TestPlanClone, TestSummarizeClone pass)
- [x] T010 Port `OwnerServiceGateway` + adapter `OwnerGateway` qua `src/backend/pkg/modcall` (deadline bước saga, Principal gốc, `Idempotency-Key`, ánh xạ lỗi; module chưa nối → `503 UNAVAILABLE`) — file: `src/backend/modules/identity/internal/application/port/owner_gateway.go`, `src/backend/modules/identity/internal/infrastructure/modules/owner_gateway.go` — ✅ 2026-11-19: `application/port/owner_gateway.go` + `infrastructure/modules/owner_gateway.go`; TestOwnerGatewayCallContext, TestOwnerGatewayUnavailable pass
- [x] T011 Middleware `/admin/*`: `azp = eduworld-portal`, MFA, role platform, audit `authz.denied` — file: `src/backend/modules/identity/internal/presentation/http/public/admin_guard.go` — ✅ 2026-11-19: Dùng guard sẵn có của `pkg/httpx` (`/admin/*` → `portal_staff` + `platform_mfa`) và `authz.Require` (audit `authz.denied`); không cần file middleware riêng

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — System Admin đọc dữ liệu liên tenant có kiểm soát (Priority: P1) 🎯 MVP

**Goal**: Phạm vi liên tenant tường minh, có audit; người dùng tenant không vượt tenant.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T012 [P] [US1] API test F045-AC-01, F045-AC-02: `tenant_ids` rỗng = mọi tenant active, cột `tenant_id`, audit `cross_tenant.accessed`; `tenant_ids` của người dùng tenant bị bỏ qua — file: `src/backend/modules/identity/tests/integration/platformadmin/cross_tenant_read_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash (`platform_admin_trash_integration_test.go`) pass
- [x] T013 [P] [US1] Test authorization F045-AC-03, F045-AC-04 cho mọi endpoint F045 (ma trận `system_admin`, `content_admin`, `safety_moderator`, `tenant_admin`, `school_admin`, thiếu MFA, sai `azp`) — file: `src/backend/modules/identity/tests/integration/platformadmin/authz_matrix_test.go` — ✅ 2026-11-19: Ma trận role trong TestPlatformAdminCrossTenantTrash/Assignments/Clone/Merge (content_admin, safety_moderator, tenant_admin, school_admin, thiếu MFA) + `authztest.Cover`
- [x] T014 [P] [US1] API test F045-AC-05: `listTenants?q` không phân biệt hoa thường, cursor, `q` quá dài → 422 — file: `src/backend/modules/identity/tests/integration/platformadmin/list_tenants_test.go` — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash (`listTenants?q=PGD-PA`, `q` 101 ký tự → 422)
- [x] T015 [P] [US1] Test F045-AC-06: bypass chỉ qua `tenancy.Bypass` có audit + metric; linter chặn chỗ khác — file: `src/backend/modules/identity/tests/integration/platformadmin/bypass_guard_test.go` — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash (đếm `eduworld_tenant_filter_bypass_total{identity,trash}` + audit); linter `ewlint bypassreason` sẵn có pass

### Implementation cho User Story 1

- [x] T016 [US1] Query `ListTenants` (tìm theo mã/tên, `tenant_type`) — file: `src/backend/modules/identity/internal/application/query/platformadmin/list_tenants.go` — ✅ 2026-11-19: `listTenants` đã có từ F010 (`query/tenant.go`), dùng lại
- [x] T017 [US1] Helper đọc liên tenant (query handler gọi tường minh): `authz.CrossTenantScope` + audit `cross_tenant.accessed` cho query của module — file: `src/backend/modules/identity/internal/application/query/platformadmin/cross_tenant_scope.go` — ✅ 2026-11-19: `query.PlatformAdmin.ListTrash`: `authz.CrossTenantScope` + `tenancy.RecordCrossTenantScope` (danh sách tenant tường minh)
- [x] T018 [US1] Handler `listTenants` — file: `src/backend/modules/identity/internal/presentation/http/public/admin_platform_handler.go` — ✅ 2026-11-19: Handler `listTenants` có từ F010; route F045 ở `platform_admin.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Gán dữ liệu platform cho tenant (Priority: P1)

**Goal**: Saga gán/gỡ gán qua module sở hữu, gán mặc định cho tenant mới.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T019 [P] [US2] Integration test F045-AC-07, F045-AC-08: định tuyến theo `resource_type`, một assignment/tenant, gọi lại không trùng, event + audit từng tenant; tài nguyên không phải platform → 422, không tồn tại → 404 — file: `src/backend/modules/identity/tests/integration/platformadmin/assign_to_tenants_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: TestPlatformAdminAssignments pass
- [x] T020 [P] [US2] Facade test identity ↔ learning F045-AC-09: tenant được gán đọc chỉ đọc, tenant chưa gán → `RESOURCE_NOT_ASSIGNED` — file: `src/backend/modules/identity/tests/contract/platformadmin/learning_assignment_contract_test.go` — ✅ 2026-11-19: TestOwnerGatewayCallContext (bên gọi, fake facade learning); bên nhận (chỉ đọc, `RESOURCE_NOT_ASSIGNED`) đã kiểm ở F020 — Clarifications 2026-11-19
- [x] T021 [P] [US2] Integration test F045-AC-10, F045-AC-11: gỡ gán soft-delete, attempt cũ xem được; `apply_to_new_tenants` khi `tenant.created` — file: `src/backend/modules/identity/tests/integration/platformadmin/unassign_default_test.go` — ✅ 2026-11-19: TestPlatformAdminAssignments (gỡ gán soft-delete, consumer `tenant.created` gán mặc định, gửi lại không trùng)
- [x] T022 [P] [US2] Test F045-AC-12: replay cùng key cùng payload → kết quả cũ; khác payload → `409 IDEMPOTENCY_KEY_REUSED` — file: `src/backend/modules/identity/tests/integration/platformadmin/assign_idempotency_test.go` — ✅ 2026-11-19: TestPlatformAdminAssignments (replay `Idempotent-Replayed`, `409 IDEMPOTENCY_KEY_REUSED`)

### Implementation cho User Story 2

- [x] T023 [P] [US2] Domain saga gán (`tenant_assign`/`tenant_unassign`), bước theo từng tenant — file: `src/backend/modules/identity/internal/domain/platformadmin/saga.go` — ✅ 2026-11-19: Không cần saga gán (một lời gọi facade nguyên tử) — Clarifications 2026-11-19
- [x] T024 [US2] Command `AssignToTenants`, `UnassignFromTenants` (kiểm tenant `active`, outbox `tenant_data.assigned/unassigned`, audit) — file: `src/backend/modules/identity/internal/application/command/platformadmin/assign_to_tenants.go`, `src/backend/modules/identity/internal/application/command/platformadmin/unassign_from_tenants.go` — ✅ 2026-11-19: `command.AssignToTenants`, `command.UnassignFromTenants` (event + audit từng tenant)
- [x] T025 [US2] Command `ApplyDefaultAssignments` + consumer `tenant.created` — file: `src/backend/modules/identity/internal/application/command/platformadmin/apply_default_assignments.go`, `src/backend/modules/identity/internal/presentation/consumer/tenant_created_consumer.go` — ✅ 2026-11-19: `command.ApplyDefaultAssignments` + `consumer.DefaultAssignments`
- [x] T026 [US2] Query `ListTenantAssignments` (gọi `listTenantResourceAssignments` của module sở hữu) + handler `assignToTenants`, `unassignFromTenants`, `listTenantAssignments` — file: `src/backend/modules/identity/internal/application/query/platformadmin/list_tenant_assignments.go`, `src/backend/modules/identity/internal/presentation/http/public/admin_platform_handler.go` — ✅ 2026-11-19: `query.PlatformAdmin.ListTenantAssignments` + handler

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Clone cấu hình/nội dung giữa tenant (Priority: P2)

**Goal**: Job clone bất đồng bộ qua saga, không dữ liệu học sinh.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T027 [P] [US3] Integration test F045-AC-13, F045-AC-14: `202` + `job_id`, gọi bước theo đồ thị, truyền `id_map`, bản sao `draft` có `cloned_from_*` (dùng module learning thật qua testcontainers) — file: `src/backend/modules/identity/tests/integration/platformadmin/tenant_clone_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: TestPlatformAdminClone pass (TEST DOUBLE facade learning qua gateway thật — module learning thật không import được)
- [x] T028 [P] [US3] Test F045-AC-15, F045-AC-16: loại ngoài allowlist, nguồn = đích, tenant `disabled`, `resource_ids` > 500 → 422; quét DB đích không có học sinh/enrollment/attempt/mastery mới — file: `src/backend/modules/identity/tests/integration/platformadmin/tenant_clone_validation_test.go` — ✅ 2026-11-19: TestPlatformAdminClone (cùng tenant, `quest`, `student`, tenant `disabled`/không tồn tại, 501 id → 422, không tạo job; số học sinh tenant đích không đổi)
- [x] T029 [P] [US3] Test F045-AC-17: lỗi tạm thời → retry cùng key, bước không hiệu lực hai lần, job `partially_failed`/`failed` kèm `errors[]` — file: `src/backend/modules/identity/tests/integration/platformadmin/saga_retry_test.go` — ✅ 2026-11-19: TestPlatformAdminClone (`failAfter`: bước có hiệu lực rồi lỗi → retry cùng key, không clone hai lần; hết số lần thử → `failed` + `errors[]`)
- [x] T030 [P] [US3] Test F045-AC-18: audit `tenant_data.cloned` ở tenant nguồn và đích, event `tenant_data.cloned` — file: `src/backend/modules/identity/tests/integration/platformadmin/tenant_clone_audit_test.go` — ✅ 2026-11-19: TestPlatformAdminClone (audit `tenant_data.cloned` ×2, event ×1, đóng job một lần)

### Implementation cho User Story 3

- [x] T031 [P] [US3] Domain `TenantCloneJob` (trạng thái, `id_map`, thống kê) — file: `src/backend/modules/identity/internal/domain/platformadmin/clone_job.go` — ✅ 2026-11-19: `domain/platformadmin/clone.go` (`SummarizeClone`, trạng thái job)
- [x] T032 [US3] Command `CreateTenantClone` (validate, tạo job + saga trong một transaction, outbox) — file: `src/backend/modules/identity/internal/application/command/platformadmin/create_tenant_clone.go` — ✅ 2026-11-19: `command.CreateTenantClone` (job + `saga.Engine.Enqueue` cùng transaction)
- [x] T033 [US3] Command `RunSagaStep` + job `platform-saga` (`FOR UPDATE SKIP LOCKED` theo `next_run_at`, `Idempotency-Key = <saga_id>:<step_no>`) — file: `src/backend/modules/identity/internal/application/command/platformadmin/run_saga_step.go`, `src/backend/modules/identity/internal/presentation/job/platform_saga_runner.go` — ✅ 2026-11-19: `CloneDefinition` + `command.PlatformSagas.RunOnce` (ResumeDue + đóng job), job `PlatformSaga`; khóa `saga_runs` dùng của engine F016, `FOR UPDATE SKIP LOCKED` khi đóng job
- [x] T034 [US3] Query `GetTenantClone` + handler `createTenantClone`, `getTenantClone` — file: `src/backend/modules/identity/internal/application/query/platformadmin/get_tenant_clone.go`, `src/backend/modules/identity/internal/presentation/http/public/admin_platform_handler.go` — ✅ 2026-11-19: `query.PlatformAdmin.GetTenantClone` + handler
- [x] T035 [US3] Bước clone của identity (`class_template`, `school_settings`, `tenant_settings`) — file: `src/backend/modules/identity/internal/infrastructure/persistence/identity_clone_step.go` — ✅ 2026-11-19: Loại identity chưa có bảng → bước clone trả 503 (OQ F045-4)

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Thùng rác và khôi phục (Priority: P2)

**Goal**: Thùng rác System Admin và Tenant Admin, khôi phục có lý do.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T036 [P] [US4] Integration test F045-AC-19, F045-AC-20: list từ module sở hữu, đủ trường `deleted_*`, `restorable`; restore có `reason` → xuất hiện lại, `<entity>.restored`, audit — file: `src/backend/modules/identity/tests/integration/platformadmin/trash_admin_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash
- [x] T037 [P] [US4] Test F045-AC-21: khôi phục vi phạm unique → `409 CONFLICT`, dữ liệu không đổi — file: `src/backend/modules/identity/tests/integration/platformadmin/trash_conflict_test.go` — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash (`dup` → 409, dữ liệu không đổi)
- [x] T038 [P] [US4] Test tenant isolation + authorization F045-AC-22 cho `listTenantTrash`, `restoreTenantTrashItem`: tenant khác → 404, thiếu permission → 403 — file: `src/backend/modules/identity/tests/integration/platformadmin/trash_tenant_test.go` — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash (`/tenant/trash*`: tenant khác 404, thiếu permission 403)
- [x] T039 [P] [US4] Test F045-AC-23: không có route xóa vĩnh viễn (quét OpenAPI + router); API đọc thường không trả bản ghi đã xóa — file: `src/backend/modules/identity/tests/integration/platformadmin/no_hard_delete_test.go` — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash (DELETE → 404, bản ghi xóa không hiện ở đọc thường)

### Implementation cho User Story 4

- [x] T040 [US4] Trash provider của identity (đọc qua `tenancy.Bypass(ctx, "trash")`, khôi phục kiểm partial unique) — file: `src/backend/modules/identity/internal/infrastructure/persistence/identity_trash_provider.go` — ✅ 2026-11-19: `read.PlatformAdminModel.DeletedAccounts` dưới `tenancy.Bypass(trash)` (loại `user`/`student`)
- [x] T041 [US4] Query `ListTrash` (định tuyến theo `resource_type`, phạm vi System Admin/tenant) + command `RestoreTrashItem` (saga `trash_restore`, audit có lý do) — file: `src/backend/modules/identity/internal/application/query/platformadmin/list_trash.go`, `src/backend/modules/identity/internal/application/command/platformadmin/restore_trash_item.go` — ✅ 2026-11-19: `query.PlatformAdmin.ListTrash/ListTenantTrash` + `command.RestoreTrashItem` (đồng bộ, audit có lý do; `student` qua port `StudentRestorer` của F016)
- [x] T042 [US4] Handler `listTrash`, `restoreTrashItem`, `listTenantTrash`, `restoreTenantTrashItem` — file: `src/backend/modules/identity/internal/presentation/http/public/admin_platform_handler.go`, `src/backend/modules/identity/internal/presentation/http/public/tenant_trash_handler.go` — ✅ 2026-11-19: Handler trong `presentation/http/platform_admin.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Gọi module khác an toàn khi điều phối (Priority: P2)

**Goal**: Facade qua `src/backend/pkg/modcall`, ngữ cảnh truyền qua `context`, lỗi có kiểm soát.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T043 [P] [US5] Facade test F045-AC-24: fake facade nhận đủ span, deadline, `TenantScope`, Principal gốc, `Idempotency-Key` từ `context`; fixture module ngoài `x-allowed-callers` import facade platform ops → `make arch-check` fail — file: `src/backend/modules/identity/tests/contract/platformadmin/facade_context_contract_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: TestOwnerGatewayCallContext (span/deadline/Principal/scope/Idempotency-Key, `x-allowed-callers`); kiểm import chéo bằng `archgen --check` (OQ F020-2)
- [x] T044 [P] [US5] Integration test F045-AC-25: facade module sở hữu trả lỗi hạ tầng/vượt deadline/chưa nối → `503 UNAVAILABLE`, không dùng cache — file: `src/backend/modules/identity/tests/integration/platformadmin/owner_unavailable_test.go` — ✅ 2026-11-19: TestOwnerGatewayUnavailable + TestPlatformAdminCrossTenantTrash (`quest` → 503, learning down → 503)
- [x] T045 [P] [US5] API test F045-AC-26: `reason`, `q` vượt `maxLength`, `tenant_ids` > 100 → `422 VALIDATION_FAILED` — file: `src/backend/modules/identity/tests/integration/platformadmin/input_limits_test.go` — ✅ 2026-11-19: TestPlatformAdminCrossTenantTrash/Assignments/Merge (`q`, `reason`, `tenant_ids` > 100 → 422)

### Implementation cho User Story 5

- [x] T046 [US5] Hoàn thiện `OwnerGateway`: truyền ngữ cảnh bắt buộc qua `context` (`src/backend/pkg/modcall`), ánh xạ lỗi phụ thuộc → `UNAVAILABLE`, metric gọi facade của `src/backend/pkg/modcall` — file: `src/backend/modules/identity/internal/infrastructure/modules/owner_gateway.go` — ✅ 2026-11-19: `owner_gateway.go`: 5xx/deadline/chưa nối → `503 UNAVAILABLE`; metric `eduworld_internal_client_*` của `pkg/modcall`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Hợp nhất hai tài khoản của cùng một người (Priority: P3)

**Goal**: System Admin duyệt và chạy saga `user_merge`.

**Independent Test**: Như spec.md US6.

### Tests cho User Story 6

- [x] T047 [P] [US6] Integration test F045-AC-27, F045-AC-33: `mergeUsers` → `202` + `merge_id`; `external_identities`/`sis_identity_links` chuyển sang user đích, `identity.user_merges` một dòng, user nguồn soft-delete `merged`, issue đóng; đăng nhập định danh cũ vào user đích; Thùng rác `restorable = false` — file: `src/backend/modules/identity/tests/integration/platformadmin/user_merge_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: TestPlatformAdminMerge pass
- [x] T048 [P] [US6] Facade test F045-AC-28 identity ↔ learning, game (`mergeUserData` với `Idempotency-Key = merge_id`, kết quả `done`/`blocked`/`nothing_to_do`) — file: `src/backend/modules/identity/tests/contract/merge_user_data_test.go` — ✅ 2026-11-19: TestOwnerGatewayCallContext (`mergeUserData` learning) + TestPlatformAdminMerge (4 module, key = merge_id); bên nhận chưa có ở module nào (OQ F045-3); 2026-11-20: bên nhận learning có (F030 T056, `TestMergeUserDataLinksStudent`)
- [x] T049 [P] [US6] Test F045-AC-29: module trả `blocked`/vượt deadline → saga `failed`, bước `done` không chạy lại, alert `SagaStuck`, `getUserMerge` trả lỗi từng bước — file: `src/backend/modules/identity/tests/integration/platformadmin/user_merge_saga_test.go` — ✅ 2026-11-19: TestPlatformAdminMerge (game `blocked` → `failed`, chạy lại không lặp bước `done`); cảnh báo `SagaStuck` dùng metric engine F016
- [x] T050 [P] [US6] Test authorization + tenant isolation F045-AC-30: `school_admin`, `content_admin`, thiếu MFA → 403; user không thuộc tenant đích → 404 — file: `src/backend/modules/identity/tests/integration/platformadmin/user_merge_authz_test.go` — ✅ 2026-11-19: TestPlatformAdminMerge (School Admin, content_admin, thiếu MFA → 403; user tenant khác → 404)
- [x] T051 [P] [US6] API test F045-AC-31, F045-AC-32: from = into, khác `user_type`, đã gộp, `anonymized`, issue sai trạng thái → `409 CONFLICT`; `reason` < 3 / > 1000 → 422; replay cùng key → cùng `merge_id`, khác payload → `409 IDEMPOTENCY_KEY_REUSED`; audit `user.merged` không PII — file: `src/backend/modules/identity/tests/integration/platformadmin/user_merge_validation_test.go` — ✅ 2026-11-19: TestPlatformAdminMerge (from = into, issue chưa đề nghị, gộp lại → 409; reason 2/1001 → 422; replay/`IDEMPOTENCY_KEY_REUSED`; audit không chứa username)

### Implementation cho User Story 6

- [x] T052 [P] [US6] Domain `UserMerge` (invariant from ≠ into, cùng `user_type`, nguồn chưa gộp/ẩn danh) + saga `user_merge` (bước identity, bước theo module) — file: `src/backend/modules/identity/internal/domain/platformadmin/user_merge.go` — ✅ 2026-11-19: `domain/platformadmin/merge.go` + `MergeDefinition`
- [x] T053 [US6] Command `MergeUsers` (kiểm issue `merge_requested`, tạo `user_merges` + saga + outbox `audit.recorded` trong một transaction), bước identity `TransferIdentities` — file: `src/backend/modules/identity/internal/application/command/merge_users.go` — ✅ 2026-11-19: `command.MergeUsers` + bước identity `TransferIdentities` (`write/platform_admin_repo.go`)
- [x] T054 [US6] Query `GetUserMerge` + handler `mergeUsers`, `getUserMerge` (middleware `/admin/*`, `platform.user.merge`) — file: `src/backend/modules/identity/internal/presentation/http/public/admin_user_merge_handler.go` — ✅ 2026-11-19: `query.PlatformAdmin.GetUserMerge` + handler `mergeUsers`/`getUserMerge`; permission `platform.user.merge` (catalog + ADMIN_PORTAL §2.2)

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: Polish & Cross-Cutting

- [x] T055 [P] Metric saga và span theo plan.md §Quan sát; kiểm log không chứa `delete_reason`/tiêu đề ở INFO — file: `src/backend/modules/identity/internal/infrastructure/observability/platform_metrics.go` — ✅ 2026-11-19: Span `observability.UseCase` cho mọi use case; metric bypass và `eduworld_internal_client_*` sẵn có; log job chỉ id/loại (không lý do/tiêu đề). Metric riêng `eduworld_platform_saga_*` dùng `saga.Metrics` của engine (chưa nối) [Confirmed]
- [x] T056 [P] Cập nhật ADMIN_PORTAL §3.7–§3.8 nếu thêm `reason` khi khôi phục và danh sách `resource_type`, ADMIN_PORTAL §2.2 (`platform.user.merge`), DATABASE_DESIGN §2.16 (`identity.user_merges.saga_id`, `reconciliation_issue_id`), SIS_INTEGRATION (issue `merge_requested`); CHANGELOG; trạng thái F045 trong CODE_PLAN — file: `docs/04-admin-portal/ADMIN_PORTAL.md`, `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — ✅ 2026-11-19: ADMIN_PORTAL §2.2 (`platform.user.merge`), §3.8 (lý do khôi phục, loại `user`/`student`); DATABASE_DESIGN §2.0/§2.16; CHANGELOG; CODE_PLAN
- [x] T057 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/045-platform-administration/quickstart.md` — ✅ 2026-11-19: quickstart.md §5 bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). T059–T062 đã làm 2026-11-20; T058 chưa làm.

- [ ] T058 **OQ chốt 2026-11-20** (OQ F057-1) `previewTenantClone` + `TenantClonePreview` (contract public) và operation facade `previewTenantDataClone` (spec internal learning, `x-allowed-callers: [identity]`), hiện thực phía identity + learning; test authorization, tenant, phụ thuộc platform giữ tham chiếu — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/modules/learning.internal.v1.yaml`, `src/backend/modules/identity/internal/application/query/platform_admin.go`, `src/backend/modules/learning/internal/presentation/facade/` — kiểm: `make specs-lint`, `make test-contract`, integration — cỡ M — ⏸ 2026-11-20: chưa làm — lớn hơn cỡ M (phép xác định phụ thuộc cho mọi ResourceHandler của learning + facade mới + hợp nhất ở identity + màn F057), module learning đang được F031 sửa; chờ điều phối tách việc (ghi ở spec Clarifications)
- [x] T059 **OQ chốt 2026-11-20** (OQ F030) `activity` trong `ResourceType` (API public + `internal-common`) và bảng định tuyến `activity → learning` — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/modules/internal-common.v1.yaml`, `src/backend/modules/identity/internal/domain/platformadmin/routing.go` — kiểm: unit enum đủ module, `make specs-breaking` — cỡ S — ✅ 2026-11-20: `activity` trong `ResourceType` (spec gộp + `internal-common`, facade sinh lại), `activity → learning` ở `routing.go`; test `TestRoutingActivity`, `TestRoutingCoversResourceTypeEnum` (đỏ trước khi sửa) PASS. `make specs-breaking` không chạy được: oasdiff v1.11.7 không đọc được spec gộp hiện có trên main (`exclusiveMinimum` dạng số, lỗi sẵn có) — thay đổi chỉ thêm giá trị enum
- [x] T060 **OQ chốt 2026-11-20** (OQ F045-4) Clone `tenant_settings`/`school_settings` từ bảng F015; `class_template` → `422` khi tạo job — file: `src/backend/modules/identity/internal/infrastructure/persistence/identity_clone_step.go` — kiểm: integration — cỡ S — ✅ 2026-11-20: `class_template` bỏ khỏi allowlist clone → `422` khi tạo job; `tenant_settings` (`resource_ids = [source]`) và `school_settings` (một trường nguồn → mọi trường của tenant đích, bỏ `allowed_hours` rộng hơn giờ tenant đích) chạy trong bước identity của saga (`platform_clone_settings.go`, cách ánh xạ [Confirmed] ghi ở spec), phát `tenant.updated`/`platform_settings.changed`/`school.updated`, audit cài đặt; test `TestPlatformAdminCloneSettings`, `TestPlanCloneSettings` PASS
- [x] T061 **OQ chốt 2026-11-20** (OQ F045-6) Bước identity của saga gộp: chuyển enrollment active, soft-delete `org.students` nguồn (`merged`), gỡ membership nguồn — file: `src/backend/modules/identity/internal/application/command/platform_admin.go` — kiểm: integration saga gộp — cỡ M — ✅ 2026-11-20: bước identity của saga gộp (`mergeStudentRecord` cùng transaction): chuyển enrollment active của nguồn sang đích khi đích chưa có (có thì kết thúc enrollment của nguồn [Confirmed]), soft-delete `org.students` nguồn (`delete_reason = merged`), phát `enrollment.changed` và `tenant_membership.changed` (`removed`, `source = merge`); test `TestPlatformAdminMerge` (mở rộng) PASS
- [x] T062 **OQ chốt 2026-11-20** (OQ F045-9) `unassignFromTenants.remove_default` (contract + soft-delete quy tắc mặc định + audit) — file: `specs/openapi/modules/identity.v1.yaml`, `src/backend/modules/identity/internal/application/command/platform_admin.go` — kiểm: `make specs-breaking`, integration — cỡ S — ✅ 2026-11-20: `unassignFromTenants.remove_default` (contract, mặc định `false`) → soft-delete quy tắc trong `platform.tenant_default_assignments` (`delete_reason = unassigned`) + audit `tenant_data.default_assignment_removed`; tenant tạo sau không được gán; test `TestPlatformAdminAssignments` (mở rộng) PASS

## Dependencies & Execution Order

- Setup → Foundational (T004–T011) → US1 → US2 → US3, US4 (song song) → US5 → US6 (cần engine saga của US3 và callee `mergeUserData` ở learning/game) → Polish.
- T004 cần thống nhất với F020 (learning hiện thực `assignTenantResources`) trước khi chạy T019–T020; facade test chạy với hiện thực facade thật của module learning.
- T026, T034, T042 cùng sửa `admin_platform_handler.go`: làm tuần tự.

## Definition of Done

- [x] Mọi tiêu chí F045-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
