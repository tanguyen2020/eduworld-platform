# Tasks: F045 — Platform Administration backend: saga gán/clone, thùng rác mọi tenant, đọc liên tenant

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo module `platformadmin` (domain/application/infrastructure/presentation) và đăng ký route, consumer, job trong composition root — file: `modules/identity/cmd/api/main.go`, `modules/identity/cmd/worker/main.go`
- [ ] T002 [P] Nối facade platform ops của learning, game, competition, chat vào module identity ở composition root (`module.go`, không URL/env, không token) và bảng `resource_type` → module — file: `modules/identity/module.go`, `modules/identity/internal/infrastructure/modules/registry.go`
- [ ] T003 [P] Sinh facade platform ops từ spec internal của 4 module (vào `modules/<module>/api`) — file: `modules/identity/internal/infrastructure/modules/gen.go` — kiểm: `make gen MOD=identity`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract internal: `assignTenantResources`, `unassignTenantResources`, `listTenantResourceAssignments` trong interface chung, đồng bộ sang `<module>.internal.v1.yaml` — file: `specs/openapi/modules/common/internal-common.v1.yaml`, `specs/openapi/modules/{learning,game,competition,chat}.internal.v1.yaml` — kiểm: `node scripts/openapi/sync-platform-ops.mjs`, `make specs-lint`
- [ ] T005 Delta contract public: `mergeUsers` (`POST /admin/users:merge`), `getUserMerge` (`GET /admin/user-merges/{merge_id}`), schema `UserMergeCommand`, `UserMergeJob` theo contracts §6; body `RestoreRequest {reason}` cho `restoreTrashItem`/`restoreTenantTrashItem`, `TenantCloneJob.errors[]`, thống nhất `ResourceType` (thêm `class`, `party` [Proposed]) — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity`
- [ ] T006 [P] Payload schema `tenant_data.assigned.v1`, `tenant_data.unassigned.v1`, `tenant_data.cloned.v1` — file: `specs/events/payloads/identity/tenant_data.assigned.v1.schema.json`, `specs/events/payloads/identity/tenant_data.unassigned.v1.schema.json`, `specs/events/payloads/identity/tenant_data.cloned.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T007 ent schema `platform.tenant_clone_jobs` (+ `saga_id`), `platform.saga_runs` (+ `next_run_at`, giá trị `tenant_assign`/`tenant_unassign`), `platform.saga_steps`, `platform.tenant_default_assignments` (+ PK) — file: `modules/identity/ent/schema/tenant_clone_job.go`, `modules/identity/ent/schema/saga_run.go`, `modules/identity/ent/schema/saga_step.go`, `modules/identity/ent/schema/tenant_default_assignment.go` — kiểm: `make migrate-diff MOD=identity name=f045_platform_admin`, `make migrate-lint MOD=identity`
- [ ] T008 [P] Cập nhật DATABASE_DESIGN §2.0 (cột/giá trị mới của saga, clone job, default assignment) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T009 Domain `ResourceRouting` (resource_type → module, hỗ trợ assign/clone/trash) và `ClonePlan` (allowlist, thứ tự bước) — file: `modules/identity/internal/domain/platformadmin/resource_routing.go`, `modules/identity/internal/domain/platformadmin/clone_plan.go`
- [ ] T010 Port `OwnerServiceGateway` + adapter `OwnerGateway` qua `pkg/modcall` (deadline bước saga, Principal gốc, `Idempotency-Key`, ánh xạ lỗi; module chưa nối → `503 UNAVAILABLE`) — file: `modules/identity/internal/application/port/owner_gateway.go`, `modules/identity/internal/infrastructure/modules/owner_gateway.go`
- [ ] T011 Middleware `/admin/*`: `azp = eduworld-portal`, MFA, role platform, audit `authz.denied` — file: `modules/identity/internal/presentation/http/public/admin_guard.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — System Admin đọc dữ liệu liên tenant có kiểm soát (Priority: P1) 🎯 MVP

**Goal**: Phạm vi liên tenant tường minh, có audit; người dùng tenant không vượt tenant.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T012 [P] [US1] API test F045-AC-01, F045-AC-02: `tenant_ids` rỗng = mọi tenant active, cột `tenant_id`, audit `cross_tenant.accessed`; `tenant_ids` của người dùng tenant bị bỏ qua — file: `modules/identity/tests/integration/platformadmin/cross_tenant_read_test.go` — phải FAIL trước khi code
- [ ] T013 [P] [US1] Test authorization F045-AC-03, F045-AC-04 cho mọi endpoint F045 (ma trận `system_admin`, `content_admin`, `safety_moderator`, `tenant_admin`, `school_admin`, thiếu MFA, sai `azp`) — file: `modules/identity/tests/integration/platformadmin/authz_matrix_test.go`
- [ ] T014 [P] [US1] API test F045-AC-05: `listTenants?q` không phân biệt hoa thường, cursor, `q` quá dài → 422 — file: `modules/identity/tests/integration/platformadmin/list_tenants_test.go`
- [ ] T015 [P] [US1] Test F045-AC-06: bypass chỉ qua `tenancy.Bypass` có audit + metric; linter chặn chỗ khác — file: `modules/identity/tests/integration/platformadmin/bypass_guard_test.go`

### Implementation cho User Story 1

- [ ] T016 [US1] Query `ListTenants` (tìm theo mã/tên, `tenant_type`) — file: `modules/identity/internal/application/query/platformadmin/list_tenants.go`
- [ ] T017 [US1] Helper đọc liên tenant (query handler gọi tường minh): `authz.CrossTenantScope` + audit `cross_tenant.accessed` cho query của module — file: `modules/identity/internal/application/query/platformadmin/cross_tenant_scope.go`
- [ ] T018 [US1] Handler `listTenants` — file: `modules/identity/internal/presentation/http/public/admin_platform_handler.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Gán dữ liệu platform cho tenant (Priority: P1)

**Goal**: Saga gán/gỡ gán qua module sở hữu, gán mặc định cho tenant mới.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T019 [P] [US2] Integration test F045-AC-07, F045-AC-08: định tuyến theo `resource_type`, một assignment/tenant, gọi lại không trùng, event + audit từng tenant; tài nguyên không phải platform → 422, không tồn tại → 404 — file: `modules/identity/tests/integration/platformadmin/assign_to_tenants_test.go` — phải FAIL trước khi code
- [ ] T020 [P] [US2] Facade test identity ↔ learning F045-AC-09: tenant được gán đọc chỉ đọc, tenant chưa gán → `RESOURCE_NOT_ASSIGNED` — file: `modules/identity/tests/contract/platformadmin/learning_assignment_contract_test.go`
- [ ] T021 [P] [US2] Integration test F045-AC-10, F045-AC-11: gỡ gán soft-delete, attempt cũ xem được; `apply_to_new_tenants` khi `tenant.created` — file: `modules/identity/tests/integration/platformadmin/unassign_default_test.go`
- [ ] T022 [P] [US2] Test F045-AC-12: replay cùng key cùng payload → kết quả cũ; khác payload → `409 IDEMPOTENCY_KEY_REUSED` — file: `modules/identity/tests/integration/platformadmin/assign_idempotency_test.go`

### Implementation cho User Story 2

- [ ] T023 [P] [US2] Domain saga gán (`tenant_assign`/`tenant_unassign`), bước theo từng tenant — file: `modules/identity/internal/domain/platformadmin/saga.go`
- [ ] T024 [US2] Command `AssignToTenants`, `UnassignFromTenants` (kiểm tenant `active`, outbox `tenant_data.assigned/unassigned`, audit) — file: `modules/identity/internal/application/command/platformadmin/assign_to_tenants.go`, `modules/identity/internal/application/command/platformadmin/unassign_from_tenants.go`
- [ ] T025 [US2] Command `ApplyDefaultAssignments` + consumer `tenant.created` — file: `modules/identity/internal/application/command/platformadmin/apply_default_assignments.go`, `modules/identity/internal/presentation/consumer/tenant_created_consumer.go`
- [ ] T026 [US2] Query `ListTenantAssignments` (gọi `listTenantResourceAssignments` của module sở hữu) + handler `assignToTenants`, `unassignFromTenants`, `listTenantAssignments` — file: `modules/identity/internal/application/query/platformadmin/list_tenant_assignments.go`, `modules/identity/internal/presentation/http/public/admin_platform_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Clone cấu hình/nội dung giữa tenant (Priority: P2)

**Goal**: Job clone bất đồng bộ qua saga, không dữ liệu học sinh.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T027 [P] [US3] Integration test F045-AC-13, F045-AC-14: `202` + `job_id`, gọi bước theo đồ thị, truyền `id_map`, bản sao `draft` có `cloned_from_*` (dùng module learning thật qua testcontainers) — file: `modules/identity/tests/integration/platformadmin/tenant_clone_test.go` — phải FAIL trước khi code
- [ ] T028 [P] [US3] Test F045-AC-15, F045-AC-16: loại ngoài allowlist, nguồn = đích, tenant `disabled`, `resource_ids` > 500 → 422; quét DB đích không có học sinh/enrollment/attempt/mastery mới — file: `modules/identity/tests/integration/platformadmin/tenant_clone_validation_test.go`
- [ ] T029 [P] [US3] Test F045-AC-17: lỗi tạm thời → retry cùng key, bước không hiệu lực hai lần, job `partially_failed`/`failed` kèm `errors[]` — file: `modules/identity/tests/integration/platformadmin/saga_retry_test.go`
- [ ] T030 [P] [US3] Test F045-AC-18: audit `tenant_data.cloned` ở tenant nguồn và đích, event `tenant_data.cloned` — file: `modules/identity/tests/integration/platformadmin/tenant_clone_audit_test.go`

### Implementation cho User Story 3

- [ ] T031 [P] [US3] Domain `TenantCloneJob` (trạng thái, `id_map`, thống kê) — file: `modules/identity/internal/domain/platformadmin/clone_job.go`
- [ ] T032 [US3] Command `CreateTenantClone` (validate, tạo job + saga trong một transaction, outbox) — file: `modules/identity/internal/application/command/platformadmin/create_tenant_clone.go`
- [ ] T033 [US3] Command `RunSagaStep` + job `platform-saga` (`FOR UPDATE SKIP LOCKED` theo `next_run_at`, `Idempotency-Key = <saga_id>:<step_no>`) — file: `modules/identity/internal/application/command/platformadmin/run_saga_step.go`, `modules/identity/internal/presentation/job/platform_saga_runner.go`
- [ ] T034 [US3] Query `GetTenantClone` + handler `createTenantClone`, `getTenantClone` — file: `modules/identity/internal/application/query/platformadmin/get_tenant_clone.go`, `modules/identity/internal/presentation/http/public/admin_platform_handler.go`
- [ ] T035 [US3] Bước clone của identity (`class_template`, `school_settings`, `tenant_settings`) — file: `modules/identity/internal/infrastructure/persistence/identity_clone_step.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Thùng rác và khôi phục (Priority: P2)

**Goal**: Thùng rác System Admin và Tenant Admin, khôi phục có lý do.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T036 [P] [US4] Integration test F045-AC-19, F045-AC-20: list từ module sở hữu, đủ trường `deleted_*`, `restorable`; restore có `reason` → xuất hiện lại, `<entity>.restored`, audit — file: `modules/identity/tests/integration/platformadmin/trash_admin_test.go` — phải FAIL trước khi code
- [ ] T037 [P] [US4] Test F045-AC-21: khôi phục vi phạm unique → `409 CONFLICT`, dữ liệu không đổi — file: `modules/identity/tests/integration/platformadmin/trash_conflict_test.go`
- [ ] T038 [P] [US4] Test tenant isolation + authorization F045-AC-22 cho `listTenantTrash`, `restoreTenantTrashItem`: tenant khác → 404, thiếu permission → 403 — file: `modules/identity/tests/integration/platformadmin/trash_tenant_test.go`
- [ ] T039 [P] [US4] Test F045-AC-23: không có route xóa vĩnh viễn (quét OpenAPI + router); API đọc thường không trả bản ghi đã xóa — file: `modules/identity/tests/integration/platformadmin/no_hard_delete_test.go`

### Implementation cho User Story 4

- [ ] T040 [US4] Trash provider của identity (đọc qua `tenancy.Bypass(ctx, "trash")`, khôi phục kiểm partial unique) — file: `modules/identity/internal/infrastructure/persistence/identity_trash_provider.go`
- [ ] T041 [US4] Query `ListTrash` (định tuyến theo `resource_type`, phạm vi System Admin/tenant) + command `RestoreTrashItem` (saga `trash_restore`, audit có lý do) — file: `modules/identity/internal/application/query/platformadmin/list_trash.go`, `modules/identity/internal/application/command/platformadmin/restore_trash_item.go`
- [ ] T042 [US4] Handler `listTrash`, `restoreTrashItem`, `listTenantTrash`, `restoreTenantTrashItem` — file: `modules/identity/internal/presentation/http/public/admin_platform_handler.go`, `modules/identity/internal/presentation/http/public/tenant_trash_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Gọi module khác an toàn khi điều phối (Priority: P2)

**Goal**: Facade qua `pkg/modcall`, ngữ cảnh truyền qua `context`, lỗi có kiểm soát.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T043 [P] [US5] Facade test F045-AC-24: fake facade nhận đủ span, deadline, `TenantScope`, Principal gốc, `Idempotency-Key` từ `context`; fixture module ngoài `x-allowed-callers` import facade platform ops → `make arch-check` fail — file: `modules/identity/tests/contract/platformadmin/facade_context_contract_test.go` — phải FAIL trước khi code
- [ ] T044 [P] [US5] Integration test F045-AC-25: facade module sở hữu trả lỗi hạ tầng/vượt deadline/chưa nối → `503 UNAVAILABLE`, không dùng cache — file: `modules/identity/tests/integration/platformadmin/owner_unavailable_test.go`
- [ ] T045 [P] [US5] API test F045-AC-26: `reason`, `q` vượt `maxLength`, `tenant_ids` > 100 → `422 VALIDATION_FAILED` — file: `modules/identity/tests/integration/platformadmin/input_limits_test.go`

### Implementation cho User Story 5

- [ ] T046 [US5] Hoàn thiện `OwnerGateway`: truyền ngữ cảnh bắt buộc qua `context` (`pkg/modcall`), ánh xạ lỗi phụ thuộc → `UNAVAILABLE`, metric gọi facade của `pkg/modcall` — file: `modules/identity/internal/infrastructure/modules/owner_gateway.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Hợp nhất hai tài khoản của cùng một người (Priority: P3)

**Goal**: System Admin duyệt và chạy saga `user_merge`.

**Independent Test**: Như spec.md US6.

### Tests cho User Story 6

- [ ] T047 [P] [US6] Integration test F045-AC-27, F045-AC-33: `mergeUsers` → `202` + `merge_id`; `external_identities`/`sis_identity_links` chuyển sang user đích, `identity.user_merges` một dòng, user nguồn soft-delete `merged`, issue đóng; đăng nhập định danh cũ vào user đích; Thùng rác `restorable = false` — file: `modules/identity/tests/integration/platformadmin/user_merge_test.go` — phải FAIL trước khi code
- [ ] T048 [P] [US6] Facade test F045-AC-28 identity ↔ learning, game (`mergeUserData` với `Idempotency-Key = merge_id`, kết quả `done`/`blocked`/`nothing_to_do`) — file: `modules/identity/tests/contract/merge_user_data_test.go`
- [ ] T049 [P] [US6] Test F045-AC-29: module trả `blocked`/vượt deadline → saga `failed`, bước `done` không chạy lại, alert `SagaStuck`, `getUserMerge` trả lỗi từng bước — file: `modules/identity/tests/integration/platformadmin/user_merge_saga_test.go`
- [ ] T050 [P] [US6] Test authorization + tenant isolation F045-AC-30: `school_admin`, `content_admin`, thiếu MFA → 403; user không thuộc tenant đích → 404 — file: `modules/identity/tests/integration/platformadmin/user_merge_authz_test.go`
- [ ] T051 [P] [US6] API test F045-AC-31, F045-AC-32: from = into, khác `user_type`, đã gộp, `anonymized`, issue sai trạng thái → `409 CONFLICT`; `reason` < 3 / > 1000 → 422; replay cùng key → cùng `merge_id`, khác payload → `409 IDEMPOTENCY_KEY_REUSED`; audit `user.merged` không PII — file: `modules/identity/tests/integration/platformadmin/user_merge_validation_test.go`

### Implementation cho User Story 6

- [ ] T052 [P] [US6] Domain `UserMerge` (invariant from ≠ into, cùng `user_type`, nguồn chưa gộp/ẩn danh) + saga `user_merge` (bước identity, bước theo module) — file: `modules/identity/internal/domain/platformadmin/user_merge.go`
- [ ] T053 [US6] Command `MergeUsers` (kiểm issue `merge_requested`, tạo `user_merges` + saga + outbox `audit.recorded` trong một transaction), bước identity `TransferIdentities` — file: `modules/identity/internal/application/command/merge_users.go`
- [ ] T054 [US6] Query `GetUserMerge` + handler `mergeUsers`, `getUserMerge` (middleware `/admin/*`, `platform.user.merge`) — file: `modules/identity/internal/presentation/http/public/admin_user_merge_handler.go`

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: Polish & Cross-Cutting

- [ ] T055 [P] Metric saga và span theo plan.md §Quan sát; kiểm log không chứa `delete_reason`/tiêu đề ở INFO — file: `modules/identity/internal/infrastructure/observability/platform_metrics.go`
- [ ] T056 [P] Cập nhật ADMIN_PORTAL §3.7–§3.8 nếu thêm `reason` khi khôi phục và danh sách `resource_type`, ADMIN_PORTAL §2.2 (`platform.user.merge`), DATABASE_DESIGN §2.16 (`identity.user_merges.saga_id`, `reconciliation_issue_id`), SIS_INTEGRATION (issue `merge_requested`); CHANGELOG; trạng thái F045 trong CODE_PLAN — file: `docs/04-admin-portal/ADMIN_PORTAL.md`, `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T057 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/045-platform-administration/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T004–T011) → US1 → US2 → US3, US4 (song song) → US5 → US6 (cần engine saga của US3 và callee `mergeUserData` ở learning/game) → Polish.
- T004 cần thống nhất với F020 (learning hiện thực `assignTenantResources`) trước khi chạy T019–T020; facade test chạy với hiện thực facade thật của module learning.
- T026, T034, T042 cùng sửa `admin_platform_handler.go`: làm tuần tự.

## Definition of Done

- [ ] Mọi tiêu chí F045-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
