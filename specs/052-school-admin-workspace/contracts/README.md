# Contracts: F052 — Workspace C — quản trị trường: trường/lớp/học sinh, SIS, người dùng và phân quyền, cài đặt, audit

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Nguồn contract: [`specs/openapi/eduworld-api.v1.yaml`](../../openapi/eduworld-api.v1.yaml) (`x-service: identity`). Mọi endpoint dưới đây qua gateway public, `azp = eduworld-portal`, `user_type = staff`, `X-Tenant-Id` bắt buộc.

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listTenantSchools` | `GET /api/v1/tenant/schools` | `eduworld-api.v1.yaml` | `tenant.school.read` | Có sẵn |
| `getTenantSchool` | `GET /api/v1/tenant/schools/{school_id}` | như trên | `tenant.school.read` | Có sẵn |
| `updateTenantSchool` | `PATCH /api/v1/tenant/schools/{school_id}` (`If-Match`) | như trên | `tenant.school.write` | Có sẵn |
| `approveTenantSchool` | `POST /api/v1/tenant/schools/{school_id}:approve` (`If-Match`) | như trên | `tenant.school.write` | Có sẵn |
| `listProvinces` | `GET /api/v1/catalog/provinces?status=active` | như trên | đã đăng nhập | Có sẵn |
| `listTenantSchoolClasses` | `GET /api/v1/tenant/schools/{school_id}/classes` | như trên | `tenant.class.read` | Có sẵn |
| `listTenantClassStudents` | `GET /api/v1/tenant/schools/{school_id}/classes/{class_id}/students` | như trên | `tenant.class.read` | Cần thêm |
| `listUnassignedStudents` | `GET /api/v1/tenant/students/unassigned` | như trên | `tenant.class.read` | Có sẵn |
| `listTenantStudentLoginEvents` | `GET /api/v1/tenant/students/{student_id}/login-events` | như trên | `lms.student_activity.read` | Có sẵn |
| `createSisImport` | `POST /api/v1/schools/{school_id}/sis-imports` (`Idempotency-Key`) | như trên | `tenant.sis.import` | Có sẵn |
| `getSisImport` | `GET /api/v1/schools/{school_id}/sis-imports/{import_id}` | như trên | `tenant.sis.read` | Có sẵn |
| `listSchoolReconciliationIssues` | `GET /api/v1/schools/{school_id}/reconciliation-issues` | như trên | `tenant.sis.read` | Cần thêm |
| `resolveSchoolReconciliationIssue` | `POST /api/v1/schools/{school_id}/reconciliation-issues/{issue_id}:resolve` (`Idempotency-Key`) | như trên | `tenant.sis.import` | Cần thêm |
| `listTenantUsers` | `GET /api/v1/tenant/users` | như trên | `tenant.user.read` | Có sẵn |
| `createTenantUser` | `POST /api/v1/tenant/users` (`Idempotency-Key`) | như trên | `tenant.user.write` | Có sẵn (Cần sửa `scope_type`) |
| `updateTenantUser` | `PATCH /api/v1/tenant/users/{user_id}` (`If-Match`) | như trên | `tenant.user.write` | Có sẵn |
| `listTenantPermissions` | `GET /api/v1/tenant/permissions` | như trên | `tenant.role.read` | Có sẵn |
| `listTenantRoles` / `getTenantRole` | `GET /api/v1/tenant/roles`, `/{role_id}` | như trên | `tenant.role.read` | Có sẵn |
| `createTenantRole` | `POST /api/v1/tenant/roles` (`Idempotency-Key`) | như trên | `tenant.role.write` | Có sẵn |
| `updateTenantRole` | `PATCH /api/v1/tenant/roles/{role_id}` (`If-Match`) | như trên | `tenant.role.write` | Có sẵn |
| `deleteTenantRole` | `DELETE /api/v1/tenant/roles/{role_id}` (`If-Match`, `reason`) | như trên | `tenant.role.write` | Có sẵn |
| `listTenantUserRoles` | `GET /api/v1/tenant/user-roles` | như trên | `tenant.user.read` | Có sẵn |
| `grantTenantUserRole` | `POST /api/v1/tenant/user-roles` (`Idempotency-Key`) | như trên | `tenant.user_role.write` | Có sẵn (Cần sửa `scope_type`) |
| `revokeTenantUserRole` | `DELETE /api/v1/tenant/user-roles/{user_role_id}` (`reason`) | như trên | `tenant.user_role.write` | Có sẵn |
| `listSchoolUserRoles` / `grantSchoolUserRole` | `GET`/`POST /api/v1/schools/{school_id}/user-roles` | như trên | `tenant.user.read` / `tenant.user_role.write` (scope trường) | Có sẵn |
| `listTenantAuditLogs` | `GET /api/v1/tenant/audit-logs` | như trên | `tenant.audit.read` | Có sẵn |
| `getTenantSettings` / `updateTenantSettings` | `GET`/`PATCH /api/v1/tenant/settings` (`If-Match`, Merge Patch) | như trên | `tenant.settings.read` / `tenant.settings.write` | Có sẵn |
| `getSchoolSettings` / `updateSchoolSettings` | `GET`/`PATCH /api/v1/schools/{school_id}/settings` (`If-Match`) | như trên | `school.settings.read` / `school.settings.write` (action `action.school_lms.settings.view` / `action.school_lms.settings.edit_school`) | Có sẵn (`SchoolSettings` gồm `curfew_window`, `platform_curfew`, `effective_curfew` — ADR-0078) |
| `previewTenantStudentCleanup` | `GET /api/v1/tenant/student-cleanup/preview` | như trên | `tenant.student_cleanup.read` | Có sẵn |
| `createStudentCleanupExclusions` / `deleteStudentCleanupExclusions` | `POST`/`DELETE /api/v1/tenant/student-cleanup/exclusions` | như trên | `tenant.student_cleanup.exclude` | Có sẵn |

Mã lỗi xử lý: `PERMISSION_DENIED`, `NOT_FOUND`, `CONFLICT`, `VALIDATION_FAILED` (sub-code `PERMISSION_NOT_ALLOWED_FOR_SCOPE`), `VERSION_MISMATCH`, `PRECONDITION_REQUIRED`, `USERNAME_TAKEN`, `EMAIL_TAKEN`, `ROLE_NOT_FOUND`, `RETENTION_BELOW_MINIMUM`, `IDEMPOTENCY_KEY_REUSED`, `IDEMPOTENCY_IN_PROGRESS`, `TENANT_MISMATCH`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không áp dụng | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không áp dụng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không áp dụng (client); `role.updated`, `user_role.changed` do module identity phát khi portal gọi API | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| [`specs/sis/eduworld-sis-contract.v1.yaml`](../../sis/eduworld-sis-contract.v1.yaml) | Cột CSV mẫu và hướng dẫn tải file | Có sẵn |

## 6. Delta cần thực hiện

Owner backend: F010 (lớp/học sinh), F011 (reconciliation), F015 (school settings). Mỗi endpoint mới cần `maxLength` cho mọi string (R28), test tenant isolation + authorization ở backend.

- **Cần thêm** `listTenantClassStudents` — query `q` (maxLength 100), `Limit`, `Cursor`; item `{ student_id, display_name (≤128), student_code (≤64), username (≤128), status, last_login_at?, days_since_last_login?, source: sis|csv }`; không có ngày sinh/liên hệ phụ huynh. Task T004.
- **Cần thêm** `listSchoolReconciliationIssues` (query `issue_type`, `status` ∈ {open, resolved, ignored}, `import_id`, `Limit`, `Cursor`) và `resolveSchoolReconciliationIssue` (body `{ resolution: resolved|ignored, reason (3–500) }`, `Idempotency-Key`, audit `sis.reconciliation_issue.resolved`); schema `ReconciliationIssue { issue_id, school_id, issue_type (≤64), entity_refs (object), suggested_action (≤255), status, resolved_by?, resolved_at?, created_at }`. Task T005.
- **Đã thêm** (2026-11-17) `getSchoolSettings` / `updateSchoolSettings` — schema `SchoolSettings { allowed_hours (object), class_leaderboard_enabled, school_leaderboard_enabled, student_club_creation, curfew_window, platform_curfew (readOnly), effective_curfew (readOnly), reason (writeOnly) }`, `CurfewWindow` (HH:MM ICT), `ETag`/`If-Match`, JSON Merge Patch; khung trường không chứa trọn `platform_curfew` → `422 VALIDATION_FAILED`. Chat policy trường để F122 bổ sung. Task T006.
- **Cần sửa** `TenantUserCreateRequest.initial_user_roles[].scope_type` và `UserRoleScopeType` bỏ `class` (ADR-0072, mâu thuẫn D18 của CODE_PLAN §3). Task T007.
- **Mâu thuẫn ghi nhận**: [SIS_INTEGRATION §9–§10](../../../docs/09-identity/SIS_INTEGRATION.md) đặt `/api/v1/sis/imports`, `/api/v1/sis/reconciliation-issues` cho `school_admin` nhưng chỉ ở gateway nội bộ; contract dùng route public `/schools/{school_id}/sis-imports`. Task T050 cập nhật SIS_INTEGRATION theo contract.
