# Contracts: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

F020 không thêm endpoint public. Endpoint public liên quan thuộc module identity (F045) và gọi xuống facade `learning` ở §2 (trong tiến trình):

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `assignToTenants` / `unassignFromTenants` / `listTenantAssignments` | `POST`/`DELETE`/`GET /api/v1/admin/tenant-assignments` | `specs/openapi/eduworld-api.v1.yaml` (`x-service: identity`) | `platform.tenant_data.assign` | Có sẵn (F045) |
| `listTrash` / `restoreTrashItem` | `GET /api/v1/admin/trash`, `POST /api/v1/admin/trash/{resource_type}/{resource_id}:restore` | như trên | system_admin | Có sẵn (F045) |
| `listTenantTrash` / `restoreTenantTrashItem` | `GET /api/v1/tenant/trash`, `POST …:restore` | như trên | `tenant.trash.*` [Confirmed] | Có sẵn (F045) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listTrash` | identity → learning (facade `learning`) | `specs/openapi/modules/learning.internal.v1.yaml` | `identity` | Có sẵn |
| `restoreTrashItem` | identity → learning (facade `learning`) | như trên | `identity` | Có sẵn |
| `executeTenantCloneStep` | identity → learning (facade `learning`) | như trên | `identity` | Có sẵn |
| `applyTenantAssignments` | identity → learning (facade `learning`) | như trên (block platform-ops trong `common/internal-common.v1.yaml`) | `identity` | Có sẵn (F020, 2026-11-19) |
| `revokeTenantAssignments` | identity → learning (facade `learning`) | như trên | `identity` | Có sẵn (F020, 2026-11-19) |
| `listResourceAssignments` | identity → learning (facade `learning`, tham số `resource_type`, `resource_id`) | như trên | `identity` | Có sẵn (F020, 2026-11-19) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `tenant.created`, `tenant.updated` | Tiêu thụ (identity) | `specs/events/payloads/identity/tenant.*.v1.schema.json` | Có sẵn (áp 2026-11-17, owner F010) |
| `school.synced` | Tiêu thụ | `specs/events/payloads/identity/school.synced.v1.schema.json` | Có sẵn (áp 2026-11-17, owner F011) |
| `province.updated` | Tiêu thụ | `specs/events/payloads/identity/province.updated.v1.schema.json` | Có sẵn |
| `student.synced`, `student.cleaned_up`, `student.restored`, `student.placement_changed` | Tiêu thụ | `specs/events/payloads/identity/<event>.v1.schema.json` | Có sẵn (áp 2026-11-17, owner F010/F011/F016) |
| `<resource_type>.deleted`, `<resource_type>.restored` | Phát | `specs/events/payloads/learning/resource.deleted.v1.schema.json`, `resource.restored.v1.schema.json` | Có sẵn (F020, 2026-11-19) |
| `audit.recorded` | Phát | `specs/events/envelope.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/common/enums.schema.json#/$defs/LearningContentType` | Registry loại nội dung | Có sẵn |
| `ResourceType` (`specs/openapi/modules/common/internal-common.v1.yaml`) | Thùng rác/gán/clone | Có sẵn |

## 6. Delta cần thực hiện

- Thêm vào block platform-ops của `specs/openapi/modules/common/internal-common.v1.yaml` (đồng bộ bằng `scripts/openapi/sync-platform-ops.mjs`) ba operation: `applyTenantAssignments` (body `TenantAssignmentApplyRequest {resource_type, resource_id, tenant_ids[] maxItems 500}` → `TenantAssignmentList`), `revokeTenantAssignments` (body cùng shape + `reason` ≤ 1000), `listResourceAssignments` (query `resource_type`, `resource_id`, cursor). `Idempotency-Key` (trừ thao tác đọc), Principal gốc (on-behalf) và trace truyền qua `context` (`src/backend/pkg/modcall`); `x-allowed-callers: [identity]`. Lỗi: 422 `VALIDATION_FAILED`.
- Thêm payload schema `resource.deleted.v1` / `resource.restored.v1` (`resource_type`, `resource_id`, `tenant_id` nullable, `deleted_by`/`restored_by`, `reason` ≤ 1000) vào `specs/events/payloads/learning/`.
- Yêu cầu F010/F011/F016 bổ sung payload schema event identity ở bảng §4. Operation facade phân trang rebuild projection `listProjectionSnapshot` do **F010** sở hữu (F010-AC-27, task F010 T043, `x-allowed-callers` gồm `learning`); F020 chỉ là bên gọi.

### Trạng thái delta (2026-11-19, T004–T006)

- T004 xong: ba operation `applyTenantAssignments`, `revokeTenantAssignments`, `listResourceAssignments` nằm trong block platform-ops của `common/internal-common.v1.yaml`, đồng bộ sang `learning`/`game`/`competition`/`chat` `*.internal.v1.yaml` bằng `sync-platform-ops.mjs`; facade sinh lại (`tools/facadegen`, `tools/allowedcallers`). Schema mới: `TenantAssignmentApplyRequest`, `TenantAssignmentRevokeRequest` (`reason` 3–1000), `TenantAssignment` (`status` `active`/`revoked`, `revoked_at`), `TenantAssignmentList`. Module `game`/`competition`/`chat` chỉ có kiểu sinh, chưa hiện thực.
- T005 xong: `resource.deleted.v1`, `resource.restored.v1` (schema chung cho mẫu `<resource_type>.deleted/restored`; `check-cross-features.mjs` chấp nhận tên file theo mẫu chung này).
- T006: payload event identity ở §4 đã có (F010/F011/F016, áp 2026-11-17). Khoảng trống ghi nhận cho owner identity [Confirmed]: `class.synced` không mang `academic_year_id`/`is_virtual`/`source_version` (projection dùng `occurred_at` của envelope làm version, `academic_year_id` để NULL); `school.synced` không mang `school_level`/`is_virtual`. Rebuild projection qua `listProjectionSnapshot` (F010 T043) chưa nối ở F020 (thư mục `presentation/job` để trống theo plan).
