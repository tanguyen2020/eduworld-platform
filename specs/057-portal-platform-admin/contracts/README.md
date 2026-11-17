# Contracts: F057 — Workspace D — Platform Administration: tenant, gán/clone, thùng rác, danh mục, cấu hình hệ thống

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml`. Route `/admin/*` chỉ gateway nội bộ, `azp = eduworld-portal` + MFA + role platform.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listTenants` | `GET /api/v1/admin/tenants` | eduworld-api (identity) | `platform.cross_tenant.read` | Có sẵn |
| `createTenant` | `POST /api/v1/admin/tenants` | eduworld-api (identity) | `platform.tenant.manage` | Cần thêm |
| `updateTenant` | `PATCH /api/v1/admin/tenants/{tenant_id}` | eduworld-api (identity) | `platform.tenant.manage` | Cần thêm |
| `assignToTenants` · `listTenantAssignments` · `unassignFromTenants` | `POST`/`GET`/`DELETE /api/v1/admin/tenant-assignments` | eduworld-api | `platform.tenant_data.assign` | Có sẵn |
| `createTenantClone` · `getTenantClone` | `POST /api/v1/admin/tenant-clones` · `GET …/{job_id}` | eduworld-api | `platform.tenant_data.clone` | Có sẵn |
| `previewTenantClone` | `POST /api/v1/admin/tenant-clones:preview` | eduworld-api | `platform.tenant_data.clone` | Cần thêm |
| `listTrash` · `restoreTrashItem` | `GET /api/v1/admin/trash` · `POST …/{resource_type}/{resource_id}:restore` | eduworld-api | `platform.trash.read` / `.restore` | Có sẵn |
| `listCountries` · `listProvinces` · `listCatalogSchools` | `GET /api/v1/catalog/{countries,provinces,schools}` | eduworld-api (identity) | đã đăng nhập; `status=all`, `tenant_ids[]` chỉ System Admin | Có sẵn |
| `createCountry` · `updateCountry` | `POST /api/v1/admin/catalog/countries` · `PATCH …/{code}` | eduworld-api | `platform.catalog.write` | Có sẵn |
| `createProvince` · `updateProvince` | `POST /api/v1/admin/catalog/provinces` · `PATCH …/{province_id}` | eduworld-api | `platform.catalog.write` | Có sẵn |
| `createProvinceSuccession` | `POST /api/v1/admin/catalog/province-successions` | eduworld-api | `platform.catalog.write` | Có sẵn |
| `importSchools` · `getSchoolImport` | `POST /api/v1/admin/schools:import` · `GET /api/v1/admin/school-imports/{import_id}` | eduworld-api | `platform.catalog.write` | Có sẵn |
| `getStudentCleanupSettings` · `putStudentCleanupSettings` | `GET`/`PUT /api/v1/admin/settings/student-cleanup` | eduworld-api | `platform.settings.write` | Có sẵn |
| `listStudentCleanupRuns` · `approveStudentCleanupRun` | `GET /api/v1/admin/student-cleanup/runs` · `POST …/{run_id}:approve` | eduworld-api | `platform.student_cleanup.approve` | Có sẵn |
| `cancelStudentCleanupRun` | `POST /api/v1/admin/student-cleanup/runs/{run_id}:cancel` | eduworld-api | `platform.student_cleanup.approve` | Cần thêm |
| `getChatRetentionSettings` · `putChatRetentionSettings` | `GET`/`PUT /api/v1/admin/settings/chat-retention` | eduworld-api | `platform.settings.write` | Có sẵn |
| `getMyPermissions` | `GET /api/v1/me/permissions` | eduworld-api | đã đăng nhập | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (frontend) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Portal không phát/tiêu thụ; `tenant_data.assigned`… do backend phát (F045) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không | — |

## 6. Delta cần thực hiện

Phase 2 task T004–T006.

- **Cần thêm** (owner F010/F045) `createTenant` `POST /admin/tenants` body `TenantCreate` {`code` maxLength 64 pattern mã, `name` maxLength 255, `tenant_type` `TenantType`, `default_locale` `Locale`, `reason` maxLength 500} → 201 `TenantSummary`, 409 `CONFLICT`, 422; `updateTenant` `PATCH /admin/tenants/{tenant_id}` (JSON Merge Patch: `name`, `status` `active`/`disabled`, `reason` bắt buộc; `If-Match`). Cả hai `Idempotency-Key` (POST), `x-internal: true` ([ADMIN_PORTAL §10.1](../../../docs/04-admin-portal/ADMIN_PORTAL.md), CODE_PLAN §3).
- **Cần thêm** (owner F045) `previewTenantClone` `POST /admin/tenant-clones:preview` body `TenantCloneRequest` → `TenantClonePreview` {`items[]` {`resource_type`, `source_id`, `action` enum `clone`/`keep_platform_ref`, `title` maxLength 255} maxItems 2000, `blocked[]` {`resource_type`, `reason_code` maxLength 64}} ([ADMIN_PORTAL §3.7](../../../docs/04-admin-portal/ADMIN_PORTAL.md) bước 3).
- **Cần thêm** (owner F016) `cancelStudentCleanupRun` `POST /admin/student-cleanup/runs/{run_id}:cancel` body `ReasonRequest`, `Idempotency-Key`; trạng thái `StudentCleanupRun.status` thêm `cancelled` ([ADMIN_PORTAL §3.10](../../../docs/04-admin-portal/ADMIN_PORTAL.md) "Xác nhận hoặc Hủy").
