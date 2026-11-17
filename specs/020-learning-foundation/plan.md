# Implementation Plan: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Branch**: `020-learning-foundation` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Dựng nền dữ liệu của module `learning` (DB `eduworld_learning`): consumer projection `ref.*` từ event identity; package domain `contentmodel` (OwnerScope, ContentMeta, state machine workflow, quy tắc bất biến) và mixin ent `ContentItemMixin`/`ContentVersionMixin` dùng lại ở F021–F027; bảng `content.content_releases`, `content.content_transitions`, `infra.tenant_resource_assignments`; repository đọc nội dung lai (riêng tenant + platform được gán) + RLS loại H; registry `ResourceHandler` cho Thùng rác, gán, clone; các operation facade platform-ops cho saga của module identity. Mọi ghi qua command handler + outbox (ADR-0038, ADR-0042).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent + Atlas, oapi-codegen, nats.go (JetStream), `pkg/{entx,tenancy,authz,outbox,httpx,modcall,problem,observability}` |
| Storage | PostgreSQL `eduworld_learning` (schema `ref`, `content`, `infra`); Valkey chỉ cache (`ew:learning:*`) |
| Testing | `go test` + testcontainers (PostgreSQL, NATS, Valkey); facade test (identity gọi với fake facade; learning kiểm hiện thực facade theo ví dụ spec internal) |
| Target Platform | Linux container (module `learning` trong `core-api` + `worker-events`) |
| Project Type | backend module |
| Performance Goals | Projection ≤ 5 s p95 (TC-SVC-008); đọc nội dung lai dùng index assignment ([MULTITENANCY §5.4](../../docs/06-architecture/MULTITENANCY.md#54-hiệu-năng)) |
| Constraints | AGENTS R8, R13, R21, R26–R29; ADR-0005, ADR-0028, ADR-0029, ADR-0037, ADR-0039, ADR-0041 |
| Scale/Scope | `idempotency_record_ttl` (CONFIG_DEFAULTS) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta spec internal (facade) + event payload ở Phase 2 tasks (T004–T007) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ ghi DB `eduworld_learning`; identity → learning qua facade `learning` (`modules/learning/api`, `x-allowed-callers: [identity]`); projection qua event |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Mọi bảng mới có `tenant_id` (NULL chỉ với platform loại H) + `deleted_at`; RLS; chỉ soft-delete; test isolation |
| IV Server-authoritative | ✅ | Không có điểm/kết quả; nền cho bất biến version (trigger mẫu) |
| V Xác thực, phân quyền | ✅ | Facade platform-ops: `x-allowed-callers: [identity]` cưỡng chế bằng `make arch-check`; public đọc dùng permission catalog `pkg/authz` |
| VI An toàn trẻ em | ✅ | Projection chỉ cột tối thiểu, không PII trong log |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; MaxLen theo DATABASE_DESIGN §1.4 |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi F020-AC có task test; fixture chỉ trong `tests/` |

## Project Structure

### Documentation (feature này)

```text
specs/020-learning-foundation/
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
modules/learning/
├── ent/schema/
│   ├── mixin_content.go            # ContentItemMixin (HybridContent + owner_scope + current_version_id + latest_version_no + cloned_from_*), ContentVersionMixin
│   ├── ref_tenant.go ref_school.go ref_class.go ref_student.go ref_enrollment.go ref_province.go
│   ├── content_release.go content_transition.go
│   └── tenant_resource_assignment.go
├── migrations/                      # Atlas + SQL viết tay: RLS loại T/TA/H (không deleted_at, ADR-0089), view *_live, REVOKE DELETE, trigger mẫu content.forbid_published_update
├── internal/domain/contentmodel/    # owner_scope.go, content_meta.go, lifecycle.go (state machine), content_type.go (LearningContentType↔ResourceType)
├── internal/domain/resourceassignment/  # Assignment aggregate, quy tắc chỉ platform
├── internal/domain/trash/           # TrashEntry, RestorePolicy
├── internal/application/
│   ├── port/                        # ResourceHandler, ResourceRegistry, AssignmentRepository, ProjectionWriter, Clock
│   ├── command/                     # ApplyTenantAssignments, RevokeTenantAssignments, SoftDeleteResource, RestoreResource, ExecuteCloneStep, UpsertProjection
│   └── query/                       # ListTrash, ListResourceAssignments
├── internal/infrastructure/
│   ├── persistence/                 # hybrid_scope.go (SQL đọc nội dung lai §4.2), assignment_repo.go, ref_repo.go
│   └── messaging/                   # JetStream consumer adapter (filter subject identity)
├── internal/presentation/
│   ├── facade/platform_ops.go          # hiện thực modules/learning/api: listTrash, restoreTrashItem, executeTenantCloneStep, applyTenantAssignments, revokeTenantAssignments, listResourceAssignments
│   ├── consumer/identity_projection.go  # consumer learning.projection_identity
│   └── job/                         # (để trống — rebuild projection chờ operation facade F010)
└── tests/{unit,integration,contract}/  # integration/fixtures/fixture_content.go (chỉ test)
```

**Structure Decision**: Theo [BACKEND_DESIGN §3](../../docs/07-backend/BACKEND_DESIGN.md#3-cấu-trúc-repository-và-module-clean-architecture), đường dẫn `modules/learning/` (D1). Mixin content đặt trong module (không phải `pkg/`) vì là quy tắc nghiệp vụ nội dung, `pkg/entx` chỉ cung cấp `PlatformBase`.

## Phase 0: Research

Không có. ADR-0029, ADR-0028, MULTITENANCY §4.2 và DATABASE_DESIGN §3.0 đã đủ.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `contentmodel.OwnerScope`, `ContentMeta.Validate(kind)`, `Lifecycle.Transition(from, action)`, `resourceassignment.Assignment`, `trash.RestorePolicy` | Thuần Go, không I/O |
| application | Command `ApplyTenantAssignments`, `RevokeTenantAssignments`, `SoftDeleteResource`, `RestoreResource`, `ExecuteCloneStep`, `UpsertProjection` · Query `ListTrash`, `ListResourceAssignments` | Facade: chỉ module `identity` được import (`x-allowed-callers`), use case kiểm Principal gốc (on-behalf) trong `context`; public delete dùng permission của loại (đăng ký ở F021/F022/F024) |
| infrastructure | ent repository, `HybridScopeQuery`, outbox writer, JetStream consumer, Valkey invalidation `ew:learning:qb:pool:<tenant_id>:*` | SQL tường minh có tham số scope + `deleted_at IS NULL` |
| presentation | Hiện thực facade (6 operationId) của `modules/learning/api`, consumer `learning.projection_identity` | Interface + DTO sinh từ `specs/openapi/modules/learning.internal.v1.yaml` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F003 (pkg dữ liệu), F010 (event tenant/school/class/student/enrollment). Bên gọi facade: F045.
- Event tiêu thụ: `tenant.created`, `tenant.updated`, `school.synced`, `class.synced`, `student.synced`, `student.cleaned_up`, `student.restored`, `enrollment.changed`, `province.updated` (stream `EVT_IDENTITY`).
- Event phát: `audit.recorded`, `<resource_type>.deleted`, `<resource_type>.restored`.

### Bảo mật, tenant, idempotency, audit

- Facade: chỉ module `identity` gọi (`x-allowed-callers`, `make arch-check`), Principal gốc (on-behalf) qua `context` (`pkg/modcall`) bắt buộc cho audit; `Idempotency-Key` bắt buộc cho assign/revoke/restore/clone (`infra.idempotency_records`).
- Thùng rác và clone đọc liên tenant qua `tenancy.Bypass(ctx, reason)` với reason `trash` / `tenant_clone`; mọi lần bypass phát audit + metric.
- Audit action: `tenant_data.assigned`, `tenant_data.unassigned`, `tenant_data.cloned`, `content.deleted`, `content.restored`, `cross_tenant.accessed`.

### Quan sát

- Metric: `eduworld_projection_lag_seconds{table}`, `eduworld_consumer_rejected_total{consumer,reason}`, `eduworld_tenant_filter_bypass_total{module,reason}`, `eduworld_tenant_assignments_total{op}`.
- Span: `learning.consumer.projection`, `learning.internal.assign`, `learning.internal.trash`; log `slog` không chứa tên học sinh.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
