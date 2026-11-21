# Contracts: F045 — Platform Administration backend: saga gán/clone, thùng rác mọi tenant, đọc liên tenant

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/identity.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`, `x-service: identity`). Route `/admin/*` chỉ gateway nội bộ (`x-internal: true`), `azp = eduworld-portal` + role platform.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listTenants` | `GET /api/v1/admin/tenants?q=&tenant_type=` | `identity.v1.yaml` | `platform.cross_tenant.read` | Có sẵn |
| `listTrash` | `GET /api/v1/admin/trash?resource_type=&tenant_ids=` | `identity.v1.yaml` (`TrashItem`) | `platform.trash.read` | Có sẵn |
| `restoreTrashItem` | `POST /api/v1/admin/trash/{resource_type}/{resource_id}:restore` | `identity.v1.yaml` | `platform.trash.restore` · `Idempotency-Key` | Cần sửa (thêm body `RestoreRequest {reason}`) |
| `listTenantAssignments` | `GET /api/v1/admin/tenant-assignments?resource_type=&resource_id=` | `identity.v1.yaml` (`TenantAssignment`) | `platform.tenant_data.assign` | Có sẵn |
| `assignToTenants` | `POST /api/v1/admin/tenant-assignments` | `identity.v1.yaml` (`TenantAssignmentRequest`) | `platform.tenant_data.assign` · `Idempotency-Key` | Có sẵn |
| `unassignFromTenants` | `DELETE /api/v1/admin/tenant-assignments` | `identity.v1.yaml` | `platform.tenant_data.assign` · `Idempotency-Key` | Có sẵn |
| `createTenantClone` | `POST /api/v1/admin/tenant-clones` → `202` | `identity.v1.yaml` (`TenantCloneRequest`) | `platform.tenant_data.clone` · `Idempotency-Key` | Có sẵn |
| `getTenantClone` | `GET /api/v1/admin/tenant-clones/{job_id}` | `identity.v1.yaml` (`TenantCloneJob`) | `platform.tenant_data.clone` | Có sẵn |
| `listTenantTrash` | `GET /api/v1/tenant/trash?resource_type=` | `identity.v1.yaml` | `tenant.trash.read` | Có sẵn |
| `restoreTenantTrashItem` | `POST /api/v1/tenant/trash/{resource_type}/{resource_id}:restore` | `identity.v1.yaml` | `tenant.trash.restore` · `Idempotency-Key` | Cần sửa (thêm body `RestoreRequest {reason}`) |
| `mergeUsers` | `POST /api/v1/admin/users:merge` → `202` | `identity.v1.yaml` (`UserMergeCommand`, `UserMergeJob`) | `platform.user.merge` [Confirmed] · `Idempotency-Key` · `X-Tenant-Id` đích | Cần thêm |
| `getUserMerge` | `GET /api/v1/admin/user-merges/{merge_id}` | `identity.v1.yaml` (`UserMergeJob`) | `platform.user.merge` | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listTrash` | identity → learning, game, competition, chat | `common/internal-common.v1.yaml` | `[identity]` | Có sẵn |
| `restoreTrashItem` | identity → learning, game, competition, chat | `common/internal-common.v1.yaml` | `[identity]` | Có sẵn |
| `executeTenantCloneStep` | identity → learning, game, competition, chat | `common/internal-common.v1.yaml` | `[identity]` | Có sẵn |
| `mergeUserData` | identity → learning, game, competition, chat (`UserMergeRequest`, `UserLifecycleResult`) | `common/internal-common.v1.yaml` | `[identity]` | Có sẵn |
| `assignTenantResources` | identity → module sở hữu tài nguyên | `common/internal-common.v1.yaml` | `[identity]` | Cần thêm |
| `unassignTenantResources` | identity → module sở hữu tài nguyên | `common/internal-common.v1.yaml` | `[identity]` | Cần thêm |
| `listTenantResourceAssignments` | identity → module sở hữu tài nguyên | `common/internal-common.v1.yaml` | `[identity]` | Cần thêm |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `tenant_data.assigned` | Phát (consumer: cache content/lms/game, AUD) | `specs/events/payloads/identity/tenant_data.assigned.v1.schema.json` | Cần thêm |
| `tenant_data.unassigned` | Phát | `specs/events/payloads/identity/tenant_data.unassigned.v1.schema.json` | Cần thêm |
| `tenant_data.cloned` | Phát (AUD, NTF System Admin) | `specs/events/payloads/identity/tenant_data.cloned.v1.schema.json` | Cần thêm |
| `audit.recorded` (`user.merged`) | Phát khi saga gộp xong (AUD) | Theo [EVENT_DESIGN §2](../../../docs/07-backend/EVENT_DESIGN.md#2-envelope-chuẩn) | Có sẵn (envelope) |
| `tenant.created` | Tiêu thụ (gán mặc định) | `specs/events/payloads/identity/tenant.created.v1.schema.json` | Có sẵn (áp 2026-11-17, owner F010) |
| `<entity>.restored` | Module sở hữu phát | Theo module sở hữu | Không thuộc F045 |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- `common/internal-common.v1.yaml`: thêm `POST /internal/v1/platform/tenant-assignments` (`assignTenantResources`, body `{resource_type, resource_id, tenant_ids[] maxItems 100, assigned_by}` → `{assignments[] {tenant_id, assignment_id, created: bool}}`), `POST /internal/v1/platform/tenant-assignments:unassign` (`unassignTenantResources`, body `{resource_type, resource_id, tenant_ids[], reason (maxLength 1000)}`), `GET /internal/v1/platform/tenant-assignments?resource_type=&resource_id=` (`listTenantResourceAssignments`); mọi operation `x-allowed-callers: [identity]`, `Idempotency-Key` cho POST; đồng bộ sang `<module>.internal.v1.yaml` bằng `scripts/openapi/sync-platform-ops.mjs`. Path OpenAPI chỉ là hình thức của spec; lời gọi chạy trong tiến trình qua facade sinh vào `src/backend/modules/<module>/api` (ADR-0074).
- `identity.v1.yaml`: `restoreTrashItem`, `restoreTenantTrashItem` thêm request body `RestoreRequest {reason: string, minLength 3, maxLength 1000}` (ADMIN_PORTAL §7: lý do bắt buộc khi restore) và response `409` khi vi phạm unique (đã có ở admin, kiểm tra tenant).
- `ResourceType` (identity + internal-common): thêm `class` (TC-TEN-015, AC-ORG-016 trong MULTITENANCY) và `party` (TC-TEN-015) nếu Thùng rác phủ các loại này [Confirmed]; thống nhất enum giữa `identity.v1.yaml` và `internal-common.v1.yaml`.
- `identity.v1.yaml` (spec gộp `eduworld-api.v1.yaml`, `x-service: identity`, `x-internal: true`): thêm `mergeUsers` `POST /admin/users:merge` (body `UserMergeCommand {from_user_id: uuid, into_user_id: uuid, reconciliation_issue_id: uuid, reason: string minLength 3 maxLength 1000}`, `additionalProperties: false`; response `202 UserMergeJob {merge_id, saga_id, status enum [running, succeeded, failed], steps[] maxItems 10 {service maxLength 64, status, error maxLength 1000}}`; lỗi `403`, `404`, `409 CONFLICT`, `409 IDEMPOTENCY_KEY_REUSED`, `422`, `428`) và `getUserMerge` `GET /admin/user-merges/{merge_id}` theo [IDENTITY_MAPPING §8](../../../docs/09-identity/IDENTITY_MAPPING.md#8-api-liên-quan).
- `TenantCloneJob`: thêm `errors[] {step_no, service, message (maxLength 1000)}` để trả lỗi từng bước (F045-AC-17).
- Event payload schema mới: `tenant_data.assigned.v1`, `tenant_data.unassigned.v1`, `tenant_data.cloned.v1` (trường theo [EVENT_DESIGN §4.1](../../../docs/07-backend/EVENT_DESIGN.md#41-identity-tenancy-organization--sis), mọi string có `maxLength`).
