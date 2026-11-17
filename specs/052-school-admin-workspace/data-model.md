# Data Model: F052 — Workspace C — quản trị trường: trường/lớp/học sinh, SIS, người dùng và phân quyền, cài đặt, audit

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F052 là client (`src/cms/frontend/apps/portal`), không tạo/sửa bảng hay migration. Bảng `identity.*`, `sis.*`, `audit.*` thuộc module identity (F010, F011, F012, F015, F016); endpoint "Cần thêm" ở [contracts](contracts/README.md) §6 đọc các bảng đã có trong DATABASE_DESIGN (`reconciliation_issues`, enrollment, school settings). Dưới đây là **model phía client**.

### Cache truy vấn TanStack Query (mọi key có `scopeKey` = tenant phiên, F051)

| Query key | operationId |
|---|---|
| `['school-lms','schools', scopeKey, filters]` | `listTenantSchools` |
| `['school-lms','school', scopeKey, schoolId]` | `getTenantSchool` |
| `['school-lms','classes', scopeKey, schoolId, academicYear, grade]` | `listTenantSchoolClasses` |
| `['school-lms','class-students', scopeKey, schoolId, classId]` | `listTenantClassStudents` |
| `['school-lms','unassigned', scopeKey, schoolId]` | `listUnassignedStudents` |
| `['school-lms','login-events', scopeKey, studentId, from, to]` | `listTenantStudentLoginEvents` |
| `['school-lms','sis-import', scopeKey, schoolId, importId]` | `getSisImport` (poll khi chưa kết thúc) |
| `['school-lms','recon-issues', scopeKey, schoolId, filters]` | `listSchoolReconciliationIssues` |
| `['school-lms','users', scopeKey, filters]` | `listTenantUsers` |
| `['school-lms','roles', scopeKey]`, `['school-lms','role', scopeKey, roleId]` | `listTenantRoles`, `getTenantRole` |
| `['school-lms','permissions', scopeKey, locale]` | `listTenantPermissions` |
| `['school-lms','user-roles', scopeKey, filters]` | `listTenantUserRoles` / `listSchoolUserRoles` |
| `['school-lms','audit', scopeKey, filters]` | `listTenantAuditLogs` |
| `['school-lms','tenant-settings', scopeKey]` | `getTenantSettings` (giữ `ETag` cho `If-Match`) |
| `['school-lms','school-settings', scopeKey, schoolId]` | `getSchoolSettings` |
| `['school-lms','cleanup-preview', scopeKey]` | `previewTenantStudentCleanup` |

Mutation thành công → `invalidateQueries` theo tiền tố tương ứng.

### Trạng thái cục bộ

| Model | Trường | Ghi chú |
|---|---|---|
| `PendingWrite` (bộ nhớ) | `intentId`, `idempotencyKey`, `operationId` | Một key cho một ý định; retry dùng lại (F052-AC-11) |
| `SisUploadState` (bộ nhớ) | `file`, `sha256`, `importId`, `uploadUrl`, `uploadExpiresAt`, `phase: hashing\|creating\|uploading\|tracking\|done\|failed` | Không lưu file ra storage |
| Bộ lọc danh sách | `localStorage` `ew.portal.<userSub>.school_lms.filters.<screen>` | Tiện ích per-viewer, try/catch |

## 2. Vòng đời / trạng thái

Trạng thái `SisImport.status` (contract): `awaiting_upload → queued → validating → applying → completed | completed_with_issues | failed | expired`. Portal poll `getSisImport` khi chưa ở trạng thái cuối (chu kỳ tăng dần, dừng khi rời màn).

## 3. Quy tắc kiểm tra dữ liệu

- Validation form sinh từ OpenAPI (`maxLength`, `pattern`, `required`, `minItems`/`maxItems`); quy tắc nghiệp vụ (trùng định danh, scope gán, giới hạn số ngày) do server.
- View model lọc field: login event chỉ map `login_at`, `logout_at`, `platform`, `device_label`, `client_version`, `end_reason`.

## 4. Event / outbox

Không áp dụng — client không phát/tiêu thụ event (R25).
