# Contracts: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

F020 không thêm endpoint public. Endpoint public liên quan thuộc module identity (F045) và gọi xuống facade `learning` ở §2 (trong tiến trình):

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `assignToTenants` / `unassignFromTenants` / `listTenantAssignments` | `POST`/`DELETE`/`GET /api/v1/admin/tenant-assignments` | `specs/openapi/eduworld-api.v1.yaml` (`x-service: identity`) | `platform.tenant_data.assign` | Có sẵn (F045) |
| `listTrash` / `restoreTrashItem` | `GET /api/v1/admin/trash`, `POST /api/v1/admin/trash/{resource_type}/{resource_id}:restore` | như trên | system_admin | Có sẵn (F045) |
| `listTenantTrash` / `restoreTenantTrashItem` | `GET /api/v1/tenant/trash`, `POST …:restore` | như trên | `tenant.trash.*` [Proposed] | Có sẵn (F045) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listTrash` | identity → learning (facade `learning`) | `specs/openapi/modules/learning.internal.v1.yaml` | `identity` | Có sẵn |
| `restoreTrashItem` | identity → learning (facade `learning`) | như trên | `identity` | Có sẵn |
| `executeTenantCloneStep` | identity → learning (facade `learning`) | như trên | `identity` | Có sẵn |
| `applyTenantAssignments` | identity → learning (facade `learning`) | như trên (block platform-ops trong `common/internal-common.v1.yaml`) | `identity` | Cần thêm |
| `revokeTenantAssignments` | identity → learning (facade `learning`) | như trên | `identity` | Cần thêm |
| `listResourceAssignments` | identity → learning (facade `learning`, tham số `resource_type`, `resource_id`) | như trên | `identity` | Cần thêm |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `tenant.created`, `tenant.updated` | Tiêu thụ (identity) | `specs/events/payloads/identity/tenant.*.v1.schema.json` | Cần thêm (owner F010) |
| `school.synced` | Tiêu thụ | `specs/events/payloads/identity/school.synced.v1.schema.json` | Cần thêm (owner F011) |
| `class.synced` | Tiêu thụ | `specs/events/payloads/identity/class.synced.v1.schema.json` | Có sẵn |
| `province.updated` | Tiêu thụ | `specs/events/payloads/identity/province.updated.v1.schema.json` | Có sẵn |
| `student.synced`, `student.cleaned_up`, `student.restored`, `enrollment.changed` | Tiêu thụ | `specs/events/payloads/identity/<event>.v1.schema.json` | Cần thêm (owner F010/F011/F016) |
| `<resource_type>.deleted`, `<resource_type>.restored` | Phát | `specs/events/payloads/learning/resource.deleted.v1.schema.json`, `resource.restored.v1.schema.json` | Cần thêm |
| `audit.recorded` | Phát | `specs/events/envelope.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/common/enums.schema.json#/$defs/LearningContentType` | Registry loại nội dung | Có sẵn |
| `ResourceType` (`specs/openapi/modules/common/internal-common.v1.yaml`) | Thùng rác/gán/clone | Có sẵn |

## 6. Delta cần thực hiện

- Thêm vào block platform-ops của `specs/openapi/modules/common/internal-common.v1.yaml` (đồng bộ bằng `scripts/openapi/sync-platform-ops.mjs`) ba operation: `applyTenantAssignments` (body `TenantAssignmentApplyRequest {resource_type, resource_id, tenant_ids[] maxItems 500}` → `TenantAssignmentList`), `revokeTenantAssignments` (body cùng shape + `reason` ≤ 1000), `listResourceAssignments` (query `resource_type`, `resource_id`, cursor). `Idempotency-Key` (trừ thao tác đọc), Principal gốc (on-behalf) và trace truyền qua `context` (`pkg/modcall`); `x-allowed-callers: [identity]`. Lỗi: 422 `VALIDATION_FAILED`.
- Thêm payload schema `resource.deleted.v1` / `resource.restored.v1` (`resource_type`, `resource_id`, `tenant_id` nullable, `deleted_by`/`restored_by`, `reason` ≤ 1000) vào `specs/events/payloads/learning/`.
- Yêu cầu F010/F011/F016 bổ sung payload schema event identity ở bảng §4. Operation facade phân trang rebuild projection `listProjectionSnapshot` do **F010** sở hữu (F010-AC-27, task F010 T043, `x-allowed-callers` gồm `learning`); F020 chỉ là bên gọi.
