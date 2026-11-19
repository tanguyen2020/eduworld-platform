# Tasks: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo cây thư mục domain/application/infrastructure/presentation và rule depguard/go-arch-lint cho module learning — file: `src/backend/modules/learning/.golangci.yml`, `src/backend/modules/learning/.go-arch-lint.yml` — **xong 2026-11-19 — cây `domain/application/infrastructure/presentation` có sẵn; luật tầng + ranh giới module tập trung ở `src/backend/.golangci.arch.yml` (`tools/archgen`), không tạo file lint riêng theo module (Clarifications 2026-11-19); `archgen --check` 0 violations**
- [x] T002 [P] Khai báo stream/consumer `learning.projection_identity` (FilterSubjects `ew.evt.identity.{tenant,school,class,student,enrollment,province}.>`) — file: `src/backend/modules/learning/internal/infrastructure/messaging/streams.go` — **xong 2026-11-19 — `internal/infrastructure/messaging/streams.go` (10 subject chính xác của `EVT_IDENTITY`)**
- [x] T003 [P] Bộ fixture test (chỉ trong `tests/`, có comment `// TEST FIXTURE — không dùng trong runtime`) gồm entity `fixture_content` dùng `ContentItemMixin` — file: `src/backend/modules/learning/tests/integration/fixtures/fixture_content.go` — **xong 2026-11-19 — `tests/integration/fixtures` (ent `fixture_contents`/`fixture_content_versions`/`fixture_attempts`, `Handler`, `Install`), nhãn TEST FIXTURE**

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Thêm `applyTenantAssignments`, `revokeTenantAssignments`, `listResourceAssignments` vào block platform-ops rồi đồng bộ — file: `specs/openapi/modules/common/internal-common.v1.yaml`, `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `node scripts/openapi/sync-platform-ops.mjs`, `make specs-lint`, `make gen MOD=learning` — **xong 2026-11-19 — platform-ops + `sync-platform-ops.mjs`, `facadegen`/`allowedcallers --check` xanh; thêm `TrashItem.restorable`**
- [x] T005 [P] Thêm payload schema `resource.deleted.v1`, `resource.restored.v1` — file: `specs/events/payloads/learning/resource.deleted.v1.schema.json`, `specs/events/payloads/learning/resource.restored.v1.schema.json` — kiểm: `make specs-lint` — **xong 2026-11-19 — `resource.deleted.v1`/`resource.restored.v1`; `check-cross-features` 0 lỗi, `validate-schemas` 0 lỗi**
- [x] T006 [P] Ghi yêu cầu payload event identity cho F010/F011/F016 (issue liên kết feature); facade `listProjectionSnapshot` do F010 sở hữu (F010 T043), F020 chỉ gọi — file: `specs/020-learning-foundation/contracts/README.md` — **xong 2026-11-19 — contracts/README §"Trạng thái delta"; khoảng trống payload ghi ở Câu hỏi mở F020-1**
- [x] T007 ent schema mixin `ContentItemMixin`, `ContentVersionMixin`; `content_release`, `content_transition`, `tenant_resource_assignment`, `ref_*` (MaxLen theo data-model) — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make gen MOD=learning` — **xong 2026-11-19 — `ent/schema/{mixin_content,content}.go`; `ent/ddl_test.go` (golden) PASS; `entlint` 0**
- [x] T008 Migration Atlas + SQL viết tay: RLS loại T/TA/H (chỉ tenant / tenant + gán, không `deleted_at` — ADR-0089), view `<table>_live` + `GRANT SELECT` cho `learning_readonly`, `REVOKE DELETE, TRUNCATE` cho `learning_app`/`learning_platform`, hàm `content.forbid_published_update()`, `infra.current_tenant_id()` — file: `src/backend/modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=f020_foundation`, `make migrate-lint MOD=learning` — **xong 2026-11-19 — `migrations/20261122000001_f020_foundation.sql` (+ atlas.sum); áp bằng Atlas trong testcontainers; `tenancyguard` 0; `TestRLSAndSoftDelete`, `TestOwnerCheckAndPublishedImmutability` PASS**
- [x] T009 Domain `contentmodel` (OwnerScope, ContentMeta, Lifecycle, ContentType↔ResourceType) — file: `src/backend/modules/learning/internal/domain/contentmodel/` — **xong 2026-11-19 — `internal/domain/contentmodel` (+ `resourceassignment`, `trash`)**
- [x] T010 Port `ResourceHandler`/`ResourceRegistry` (Trash, Restore, Clone, AssignableCheck) — file: `src/backend/modules/learning/internal/application/port/resource.go` — **xong 2026-11-19 — `internal/application/port/resource.go` (`ResourceHandler`, `Registry`)**

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Projection dữ liệu identity (Priority: P1) 🎯 MVP

**Goal**: `ref.*` luôn phản ánh identity, idempotent, chống lệch tenant.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T011 [P] [US1] Test F020-AC-01 upsert theo `source_version`, bỏ bản cũ — file: `src/backend/modules/learning/tests/integration/projection_tenant_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestProjectionTenantSourceVersion` PASS (lag < 5 s)**
- [x] T012 [P] [US1] Test F020-AC-02 từng event → đúng bảng/cột tối thiểu (kiểm không có cột thừa) — file: `src/backend/modules/learning/tests/integration/projection_entities_test.go` — **xong 2026-11-19 — `TestProjectionEntities` PASS**
- [x] T013 [P] [US1] Test F020-AC-03 giao lại event xử lý 1 lần; F020-AC-04 lệch tenant → DLQ + metric — file: `src/backend/modules/learning/tests/integration/projection_dedupe_test.go` — **xong 2026-11-19 — `TestProjectionRedeliveryProcessedOnce`, `TestProjectionTenantMismatchDeadLettered` PASS**
- [x] T014 [P] [US1] Test F020-AC-05 use case ghi `ref.*` bị chặn (linter/arch test + role) — file: `src/backend/modules/learning/tests/unit/arch_ref_write_test.go` — **xong 2026-11-19 — `tests/unit/arch_ref_write_test.go` `TestOnlyProjectionConsumerWritesRef` PASS (FAIL trước khi có `ref_repo.go`)**

### Implementation cho User Story 1

- [x] T015 [US1] Command `UpsertProjection` (so `source_version`, kiểm tenant khớp) — file: `src/backend/modules/learning/internal/application/command/upsert_projection.go` — **xong 2026-11-19 — `command/upsert_projection.go`**
- [x] T016 [US1] Repository projection (ent, chỉ consumer dùng) — file: `src/backend/modules/learning/internal/infrastructure/persistence/ref_repo.go` — **xong 2026-11-19 — `infrastructure/persistence/write/ref_repo.go` (SQL tường minh, không ent: Clarifications)**
- [x] T017 [US1] Consumer `learning.projection_identity` (decode payload theo schema, NAK có delay, DLQ) — file: `src/backend/modules/learning/internal/presentation/consumer/identity_projection.go` — **xong 2026-11-19 — `presentation/consumer/identity_projection.go` + `IdentityProjectionOptions`**

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Mô hình nội dung chung và cô lập tenant (Priority: P1)

**Goal**: Đọc/ghi nội dung lai đúng phạm vi; metadata hợp lệ.

### Tests cho User Story 2

- [x] T018 [P] [US2] Unit test F020-AC-06 registry loại, F020-AC-07 ContentMeta thiếu field → lỗi từng field, F020-AC-08 owner_scope/tenant — file: `src/backend/modules/learning/internal/domain/contentmodel/content_meta_test.go` — **xong 2026-11-19 — `TestContentAndResourceTypes`, `TestContentMetaValidate`, `TestOwnerScopeTenantInvariant`, `TestLifecycle` PASS**
- [x] T019 [P] [US2] Integration F020-AC-09 đọc nội dung lai (tenant A/B/platform gán/không gán), F020-AC-10 thiếu scope + RLS 0 dòng — file: `src/backend/modules/learning/tests/integration/hybrid_scope_test.go` — **xong 2026-11-19 — `TestHybridReadScope`, `TestHybridMissingScopeAndRLS` PASS**
- [x] T020 [P] [US2] API test F020-AC-11 `tenant_ids` bị bỏ qua, header lệch → `TENANT_MISMATCH`; F020-AC-12 System Admin thiếu tenant đích → 422; F020-AC-13 CrossTenantScope + audit `cross_tenant.accessed` (`reason`, `tenant_ids`, `mode`) — file: `src/backend/modules/learning/tests/integration/scope_principal_test.go` — **xong 2026-11-19 — `TestScopeOfContentLists` (chuỗi `pkg/httpx` thật, route fixture), `TestForRead`/`TestForWrite` PASS**
- [x] T043 [P] [US2] Integration F020-AC-32: tạo nội dung platform và `content_releases` platform qua `learning_platform` + `PlatformScope` thành công; `learning_app` ghi dòng `tenant_id` NULL → RLS từ chối; tenant chỉ thấy dòng platform đã gán — file: `src/backend/modules/learning/tests/integration/platform_scope_write_test.go` — **xong 2026-11-19 — `TestPlatformScopeWrites` PASS**
- [x] T021 [P] [US2] Test F020-AC-14 chuỗi vượt MaxLen ở DB và Validate → 422 — file: `src/backend/modules/learning/tests/integration/maxlen_test.go` — **xong 2026-11-19 — `TestStringsAreBounded` PASS; facade `reason` 1001 ký tự → 422 trong `TestRevokeTenantAssignments`**

### Implementation cho User Story 2

- [x] T022 [US2] `HybridScopeQuery` (SQL tường minh nhận scope, nhánh assignment) + helper ent predicate — file: `src/backend/modules/learning/internal/infrastructure/persistence/hybrid_scope.go` — **xong 2026-11-19 — `infrastructure/persistence/read/hybrid_scope.go` (`HybridCondition`); sửa `entx` filter H (`SELECT 1`), `TestHybridPredicateSQL`**
- [x] T023 [US2] Helper chọn scope (handler gọi tường minh): `ScopeFor` / `CrossTenantScope` / `PlatformScope(p)` (ghi dòng platform qua `learning_platform`) + kiểm tenant đích khi ghi — file: `src/backend/modules/learning/internal/application/scope/target_tenant.go` — **xong 2026-11-19 — `internal/application/scope/target_tenant.go` (`ForRead`, `ForWrite`)**

## Phase 5: User Story 3 — Gán nội dung platform cho tenant (Priority: P1)

**Goal**: identity gán/gỡ gán qua facade `learning`; tenant chỉ đọc.

### Tests cho User Story 3

- [x] T024 [P] [US3] Integration F020-AC-15 gán + idempotency; F020-AC-16 tài nguyên không hợp lệ → 422 không ghi gì — file: `src/backend/modules/learning/tests/integration/assignment_apply_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestApplyTenantAssignments`, `TestApplyTenantAssignmentsInvalid` PASS**
- [x] T025 [P] [US3] Integration F020-AC-17 gỡ gán → mất quyền đọc, lịch sử giữ; F020-AC-20 audit + xóa cache — file: `src/backend/modules/learning/tests/integration/assignment_revoke_test.go` — **xong 2026-11-19 — `TestRevokeTenantAssignments` PASS (Valkey testcontainers)**
- [x] T026 [P] [US3] Test F020-AC-18 ghi nội dung được gán → 403, tham chiếu chưa gán → `RESOURCE_NOT_ASSIGNED` — file: `src/backend/modules/learning/tests/integration/assignment_readonly_test.go` — **xong 2026-11-19 — `TestAssignedContentReadOnly` PASS**
- [x] T027 [P] [US3] Facade test + kiến trúc F020-AC-19 (fixture module `game` import facade platform-ops → `make arch-check` fail; không route HTTP) + tenant isolation/authorization mọi operation facade mới — file: `src/backend/modules/learning/tests/contract/platform_ops_auth_test.go` — **xong 2026-11-19 — `tests/contract/platform_ops_auth_test.go` (`TestPlatformOpsOnlyForIdentity`, `TestPlatformOpsAuthorizePrincipal`) PASS; giới hạn arch-check theo package ghi ở Câu hỏi mở F020-2**

### Implementation cho User Story 3

- [x] T028 [P] [US3] Aggregate `resourceassignment.Assignment` — file: `src/backend/modules/learning/internal/domain/resourceassignment/assignment.go` — **xong 2026-11-19 — `internal/domain/resourceassignment/assignment.go` (+ test)**
- [x] T029 [US3] Command `ApplyTenantAssignments`, `RevokeTenantAssignments`; query `ListResourceAssignments` — file: `src/backend/modules/learning/internal/application/command/tenant_assignments.go` — **xong 2026-11-19 — `command/tenant_assignments.go`, `query/assignments.go` (`ListResourceAssignments`, `CheckReference`), `command/content_access.go`**
- [x] T030 [US3] Repository + invalidation Valkey `ew:learning:qb:pool:<tenant_id>:*` — file: `src/backend/modules/learning/internal/infrastructure/persistence/assignment_repo.go` — **xong 2026-11-19 — `persistence/write/assignment_repo.go`, `persistence/read/assignments.go`, `infrastructure/cache/invalidator.go`**
- [x] T031 [US3] Hiện thực facade `applyTenantAssignments`, `revokeTenantAssignments`, `listResourceAssignments` — file: `src/backend/modules/learning/internal/presentation/facade/platform_assignments.go` — **xong 2026-11-19 — `presentation/facade/platform_assignments.go`**

## Phase 6: User Story 4 — Soft-delete và Thùng rác (Priority: P2)

### Tests cho User Story 4

- [x] T032 [P] [US4] Integration F020-AC-21 soft-delete + outbox cùng tx; F020-AC-22 hook ent + role DELETE lỗi quyền; F020-AC-23 không hiện ở API đọc — file: `src/backend/modules/learning/tests/integration/soft_delete_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestSoftDeleteContent`, `TestHardDeleteImpossible` PASS**
- [x] T033 [P] [US4] Contract/integration F020-AC-24 `listTrash` (bypass `trash` + audit `cross_tenant.accessed` + `eduworld_tenant_filter_bypass_total`), F020-AC-25 restore + idempotency, F020-AC-26 xung đột unique → 409 — file: `src/backend/modules/learning/tests/integration/trash_test.go` — **xong 2026-11-19 — `TestTrashListAndRestore` PASS**
- [x] T042 [P] [US4] Integration F020-AC-31: policy RLS không chứa `deleted_at` (truy `pg_policies`); soft-delete item bằng `learning_app` dưới RLS thành công; item không còn qua interceptor và view `_live`; `learning_readonly` đọc bảng gốc bị từ chối — file: `src/backend/modules/learning/tests/integration/rls_softdelete_test.go` — **xong 2026-11-19 — `TestRLSAndSoftDelete` PASS**

### Implementation cho User Story 4

- [x] T034 [US4] Command `SoftDeleteResource`, `RestoreResource`; query `ListTrash` qua `ResourceRegistry` — file: `src/backend/modules/learning/internal/application/command/trash.go` — **xong 2026-11-19 — `command/trash.go` (`SoftDeleteResource`, `RestoreResource`), `query/trash.go` (`ListTrash`)**
- [x] T035 [US4] Hiện thực facade `listTrash`, `restoreTrashItem` — file: `src/backend/modules/learning/internal/presentation/facade/platform_trash.go` — **xong 2026-11-19 — `presentation/facade/platform_trash.go`**

## Phase 7: User Story 5 — Khung bước clone (Priority: P3)

### Tests cho User Story 5

- [x] T036 [P] [US5] Integration F020-AC-27 clone fixture (ID mới, draft, `cloned_from_*`, `id_map`, idempotent), F020-AC-28 không sao dữ liệu học sinh, F020-AC-29 loại chưa đăng ký → failed, F020-AC-30 audit hai tenant — file: `src/backend/modules/learning/tests/integration/clone_step_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestCloneStep` PASS**

### Implementation cho User Story 5

- [x] T037 [US5] Command `ExecuteCloneStep` (bypass `tenant_clone`, tx theo phần tử, lưu kết quả theo `(job_id, resource_type)`) — file: `src/backend/modules/learning/internal/application/command/clone_step.go` — **xong 2026-11-19 — `command/clone_step.go`**
- [x] T038 [US5] Hiện thực facade `executeTenantCloneStep` — file: `src/backend/modules/learning/internal/presentation/facade/platform_clone.go` — **xong 2026-11-19 — `presentation/facade/platform_clone.go`**

## Phase 8: Polish & Cross-Cutting

- [x] T039 [P] Cập nhật DATABASE_DESIGN: DDL `content.content_transitions`, cột `content_language`, `curriculum_version_id` của mixin version, `delete_reason` của assignment; CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md` — **xong 2026-11-19 — DATABASE_DESIGN §2.5, §2.17 (quy tắc 5), §3.0, §3.3; CHANGELOG**
- [x] T040 [P] Metric/span theo plan (projection lag, bypass, rejected) — file: `src/backend/modules/learning/internal/infrastructure/observability/metrics.go` — **xong 2026-11-19 — `infrastructure/observability/metrics.go` (`eduworld_consumer_rejected_total`, `eduworld_projection_lag_seconds`, `eduworld_tenant_assignments_total`; bypass metric của `pkg/tenancy`), span `observability.UseCase`**
- [x] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/020-learning-foundation/quickstart.md` — **xong 2026-11-19 — quickstart §5 (bằng chứng tự động; bước chạy tay chờ F021/F045 và composition root F020-3)**

## Dependencies & Execution Order

- Setup → Foundational (T004–T010) → US1 → US2 → US3 (cần US2) → US4 → US5 → Polish.
- US1 chặn mọi ghi nội dung (FK `ref.tenants`). US4/US5 cần ít nhất một `ResourceHandler` (fixture trong test; loại thật đăng ký ở F021/F022/F024).

## Definition of Done

- [x] Mọi tiêu chí F020-AC-* có test pass (ghi mã test/TC) — **32/32 AC có test PASS (bảng AC → test trong quickstart §5)**
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — **chạy bằng lệnh tương đương (không có `make`): gen-check, lint, test, test-integration xanh — quickstart §5**
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — **fixture chỉ trong `tests/`**
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — **DATABASE_DESIGN, CHANGELOG; CONFIG_DEFAULTS không đổi**
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — **Done (2026-11-19)**
