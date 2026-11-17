# Implementation Plan: F045 — Platform Administration backend: saga gán/clone, thùng rác mọi tenant, đọc liên tenant

**Branch**: `045-platform-administration` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `platformadmin` trong module `identity` phục vụ các route System Admin `/api/v1/admin/{tenants, trash, tenant-assignments, tenant-clones, users:merge, user-merges}` và route Tenant Admin `/api/v1/tenant/trash*`. Module identity **không** đọc DB module khác: nó định tuyến theo `resource_type` tới module sở hữu qua facade (trong tiến trình, `src/backend/pkg/modcall`) sinh từ `specs/openapi/modules/<module>.internal.v1.yaml` (interface chung `common/internal-common.v1.yaml`: `listTrash`, `restoreTrashItem`, `executeTenantCloneStep`, và các operation gán tài nguyên cần thêm). Thao tác nhiều bước (clone, gán nhiều tenant, khôi phục) chạy bằng saga lưu ở `platform.saga_runs`/`saga_steps`, consumer/job `platform-saga` (profile `worker-jobs`) thực thi tuần tự từng bước idempotent. Đọc liên tenant dùng `authz.CrossTenantScope` + audit `cross_tenant.accessed`; Thùng rác dùng `tenancy.Bypass` (R27).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server public + facade sinh từ spec internal), `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/authz`, `src/backend/pkg/tenancy`, `src/backend/pkg/modcall`, `src/backend/pkg/outbox`, `src/backend/pkg/idempotency`, `src/backend/pkg/audit` |
| Storage | PostgreSQL `eduworld_identity` (schema `platform`, `infra`); không Valkey cho dữ liệu saga (ADR-0004) |
| Testing | go test + testcontainers (PostgreSQL, NATS); facade test với module learning (`make test-contract`: identity dùng fake facade, learning test hiện thực facade theo ví dụ spec internal); `make arch-check` |
| Target Platform | Linux container (module `identity` api + worker) |
| Project Type | backend module |
| Performance Goals | Deadline mỗi bước saga theo `module_call_deadline` (bước saga); thử lại do engine saga (không retry/circuit breaker trong tiến trình, ADR-0074) ([API_DESIGN §12.3](../../docs/07-backend/API_DESIGN.md#123-resilience-mặc-định-proposed)) |
| Constraints | AGENTS R7, R8, R10, R13, R20, R21, R22, R27, R28, R29 · ADR-0027, ADR-0028, ADR-0029, ADR-0040, ADR-0065, ADR-0066, ADR-0074 |
| Scale/Scope | `resource_ids` ≤ 500/job; `tenant_ids` ≤ 100/request (spec) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready; delta contract (internal gán tài nguyên, `ResourceType`, `RestoreRequest` public, payload event) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Identity chỉ điều phối; mọi đọc/ghi dữ liệu module khác qua facade (`src/backend/pkg/modcall`); saga là command handler + outbox |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng `platform.*` dùng role `identity_platform` có audit; không hard-delete; test tenant isolation cho `/tenant/trash*` |
| IV Server-authoritative | ✅ | Không liên quan điểm/kết quả; mọi ghi có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | `/admin/*` gateway nội bộ + `azp = eduworld-portal` + MFA + role platform; permission `platform.cross_tenant.read`, `platform.trash.*`, `platform.tenant_data.assign/clone`, `platform.user.merge` [Proposed], `tenant.trash.*` |
| VI An toàn trẻ em | ✅ | Không clone dữ liệu học sinh (R22); `TrashItem` không chứa PII ngoài tiêu đề tài nguyên; log không chứa nội dung |
| VII Một nguồn sự thật, cấu hình | ✅ | Bảng định tuyến `resource_type` → module một nơi trong code; facade nối ở composition root (`module.go`), không URL/env cho gọi giữa module; chuỗi có `MaxLen` |
| VIII UI tách logic, token | ✅ | Không có UI (F057 làm giao diện) |
| IX Test có bằng chứng | ✅ | Mỗi `F045-AC-*` có task test viết trước; test authorization ma trận role; facade test với module learning |

## Project Structure

### Documentation (feature này)

```text
specs/045-platform-administration/
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
src/backend/modules/identity/
├── ent/schema/
│   ├── tenant_clone_job.go              # platform.tenant_clone_jobs (có sẵn trong DATABASE_DESIGN)
│   ├── saga_run.go, saga_step.go        # platform.saga_runs, platform.saga_steps
│   └── tenant_default_assignment.go     # platform.tenant_default_assignments
├── internal/domain/platformadmin/
│   ├── resource_routing.go              # ResourceType → module sở hữu, khả năng assign/clone/trash
│   ├── clone_plan.go                    # đồ thị phụ thuộc, thứ tự bước, danh sách loại được clone
│   ├── clone_job.go                     # aggregate TenantCloneJob (trạng thái, id_map)
│   └── saga.go                          # SagaRun/SagaStep, chuyển trạng thái, retry
├── internal/application/
│   ├── command/platformadmin/           # assign_to_tenants.go, unassign_from_tenants.go, create_tenant_clone.go,
│   │                                    # run_saga_step.go, restore_trash_item.go, apply_default_assignments.go
│   ├── query/platformadmin/             # list_tenants.go, list_trash.go, list_tenant_assignments.go, get_tenant_clone.go
│   └── port/                            # OwnerServiceGateway (list/restore/clone/assign), SagaRepository, CloneJobRepository, AuditWriter
├── internal/infrastructure/
│   ├── facade/                          # learning.go, game.go, competition.go, chat.go (adapter facade + src/backend/pkg/modcall)
│   │   └── owner_gateway.go             # hiện thực OwnerServiceGateway theo resource_routing
│   ├── persistence/                     # ent repo saga, clone job, default assignment; trash provider của identity
│   └── messaging/                       # outbox tenant_data.assigned/unassigned/cloned, audit.recorded
├── internal/presentation/
│   ├── http/public/                     # admin_platform_handler.go (listTenants, listTrash, restoreTrashItem, listTenantAssignments,
│   │                                    # assignToTenants, unassignFromTenants, createTenantClone, getTenantClone),
│   │                                    # tenant_trash_handler.go (listTenantTrash, restoreTenantTrashItem)
│   ├── consumer/                        # tenant_created_consumer.go (gán mặc định)
│   └── job/                             # platform_saga_runner.go
└── tests/{unit,integration,contract}/platformadmin/
```

**Structure Decision**: module identity là bên điều phối platform theo ADR-0074 (BACKEND_DESIGN §3.1: `StartTenantClone`, `TrashList`, worker "saga platform"); dữ liệu nghiệp vụ ở lại module sở hữu (ADR-0037).

## Phase 0: Research

Không có. Saga, idempotency, resilience, phạm vi liên tenant đã chốt ở ADR-0029, ADR-0040, ADR-0074, MULTITENANCY §4.1, §10.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `ResourceRouting`, `ClonePlan`, `TenantCloneJob`, `SagaRun`/`SagaStep` | Invariant: một tenant đích; nguồn ≠ đích; loại clone trong allowlist; không loại dữ liệu học sinh |
| application | Command `AssignToTenants`, `UnassignFromTenants`, `ApplyDefaultAssignments`, `CreateTenantClone`, `RunSagaStep`, `RestoreTrashItem` · Query `ListTenants`, `ListTrash`, `ListTenantAssignments`, `GetTenantClone` | Permission: `platform.cross_tenant.read` (đọc liên tenant, `listTenants`), `platform.tenant_data.assign`, `platform.tenant_data.clone`, `platform.trash.read`, `platform.trash.restore`, `tenant.trash.read`, `tenant.trash.restore` |
| infrastructure | Adapter facade 4 module + `OwnerGateway`; repo ent; trash provider identity (`class_template`, `school_settings`, `tenant_settings`); outbox | `src/backend/pkg/modcall` (deadline, `TenantScope`, Principal gốc, `Idempotency-Key`; không token, không retry trong tiến trình) |
| presentation | Handler `listTenants`, `listTrash`, `restoreTrashItem`, `listTenantAssignments`, `assignToTenants`, `unassignFromTenants`, `createTenantClone`, `getTenantClone`, `listTenantTrash`, `restoreTenantTrashItem` · consumer `tenant.created` · job `platform-saga` | Middleware `/admin/*`: `azp`, MFA (`acr`/`amr`), role platform |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F015 (audit sink, platform settings), F020 (callee learning: `listTrash`, `restoreTrashItem`, `executeTenantCloneStep`, ghi `infra.tenant_resource_assignments`), F010 (tenant, `tenant.created`), F012 (Principal, permission catalog).
- Facade (R29): `learning.internal.v1.yaml`, `game.internal.v1.yaml`, `competition.internal.v1.yaml`, `chat.internal.v1.yaml` (operation platform ops). Loại thuộc module chưa đăng ký facade trong binary → `503 UNAVAILABLE`.
- Event phát: `tenant_data.assigned`, `tenant_data.unassigned`, `tenant_data.cloned`, `audit.recorded`. Event tiêu thụ: `tenant.created` (gán mặc định). `<entity>.restored` do module sở hữu phát.

### Bảo mật, tenant, idempotency, audit

- Đọc liên tenant: `authz.CrossTenantScope(p, tenant_ids, reason)` → audit `cross_tenant.accessed`; người dùng tenant: `authz.ScopeFor(p)`, `tenant_ids` bị bỏ qua.
- Ghi của System Admin: `X-Tenant-Id` = tenant đích khi bản ghi thuộc tenant (`restoreTrashItem`); gán/clone mang tenant trong body (`tenant_ids`, `target_tenant_id`), luôn danh sách tường minh.
- `Idempotency-Key` bắt buộc cho `assignToTenants`, `unassignFromTenants`, `createTenantClone`, `restoreTrashItem`, `restoreTenantTrashItem`; mỗi bước saga gọi facade module đích với key `<saga_id>:<step_no>`.
- Audit action: `cross_tenant.accessed`, `tenant_data.assigned`, `tenant_data.unassigned`, `tenant_data.cloned` (ghi ở tenant nguồn và đích), `trash.viewed` [Proposed], `<resource_type>.restored`, `authz.denied`.

### Quan sát

- Log không chứa `delete_reason`/tiêu đề ở INFO; chỉ `saga_id`, `job_id`, `resource_type`, số bản ghi.
- Metric: `eduworld_platform_saga_steps_total{saga_type,service,status}`, `eduworld_platform_saga_duration_seconds{saga_type}`, `eduworld_tenant_filter_bypass_total{module,reason}`, metric gọi facade của `src/backend/pkg/modcall` (`caller="identity"`, `callee`, `operation`, `outcome`; tên theo API_DESIGN §12.3).
- Span: `AssignToTenants`, `CreateTenantClone`, `RunSagaStep(service, action)`, `ListTrash(resource_type)`.

Ước lượng: 1–2 sprint (backend 1–2 dev) [Proposed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
