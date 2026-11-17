# Implementation Plan: F052 — Workspace C — quản trị trường: trường/lớp/học sinh, SIS, người dùng và phân quyền, cài đặt, audit

**Branch**: `052-school-admin-workspace` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm các màn quản trị vào chunk workspace `school_lms` (`src/cms/frontend/apps/portal/src/workspaces/school-lms/`): Trường/Lớp/Học sinh, Học sinh chưa có lớp, Hoạt động đăng nhập, Nhập CSV SIS + Xung đột đồng bộ, Người dùng/Role/UserRole/Nhật ký phân quyền, Cài đặt tenant, Cài đặt trường, Học sinh sắp bị dọn, Nhật ký. Mỗi màn = page (ProTable/ProForm) + hook TanStack Query gọi operationId của module identity qua `apiClient` (F050) với `withTenantScope` (F051). Không business logic ở client: quyền, scope, trùng định danh, giới hạn số ngày đều do server; client chỉ ẩn/hiện và hiển thị lỗi theo `code`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript (strict) · React 18 · Vite |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, ProForm, StepsForm, QueryFilter), TanStack Query, React Hook Form + Zod (schema sinh từ OpenAPI — F050), `react-i18next`, `src/cms/frontend/packages/ops-ui` (`ResponsiveTable`, Tag trạng thái) |
| Storage | Không có (client); dữ liệu ở `eduworld_identity` (F010, F011, F015, F016) |
| Testing | Vitest + Testing Library + MSW (`// MOCK (test-only)`); Playwright E2E với backend local; axe |
| Target Platform | Trình duyệt desktop/tablet/mobile |
| Project Type | Web app (workspace chunk) |
| Performance Goals | Danh sách phân trang cursor, trang đầu < 1,5 s trên dữ liệu seed [Proposed] |
| Constraints | AGENTS R1, R14, R16, R20, R21, R23, R32 · ADR-0027, ADR-0031, ADR-0032, ADR-0060, ADR-0066, ADR-0069, ADR-0072, ADR-0081 |
| Scale/Scope | ~14 màn; tham số `csv_import_max`, `inactive_student_cleanup_days`, `principal_cache_ttl` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta contract (`listTenantClassStudents`, `listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue`, `getSchoolSettings`, `updateSchoolSettings`) ở Phase 2 (T004–T007) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ gọi API public module identity; không gọi API của module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Header tenant phiên; System Admin chọn một tenant; E2E cô lập `ad.truong.b` (T014, T023, T040); không có xóa vĩnh viễn |
| IV Server-authoritative | ✅ | Không có điểm/phần thưởng; mọi ràng buộc (scope, trùng, giới hạn ngày) server quyết |
| V Xác thực, phân quyền | ✅ | Bảng permission dưới đây; ẩn hành động + xử lý 403; không gán role platform |
| VI An toàn trẻ em | ✅ | Không IP/thiết bị thô; không chat; tên học sinh chỉ trong tenant phiên; không log PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; validation từ OpenAPI |
| VIII UI tách logic, token | ✅ | Page/hook tách; theme token |
| IX Test có bằng chứng | ✅ | Mỗi F052-AC-* có task test trước code |

## Project Structure

### Documentation (feature này)

```text
specs/052-school-admin-workspace/
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
src/cms/frontend/apps/portal/src/workspaces/school-lms/
├── index.tsx                      # đăng ký route; menu/action lấy từ registry sinh từ catalog (F051, ADR-0090)
├── routes.tsx                     # ánh xạ khóa `menu.school_lms.*` của catalog → trang
├── schools/{SchoolListPage.tsx,SchoolDetailPage.tsx,SchoolEditForm.tsx,ApproveSchoolDialog.tsx,useSchools.ts}
├── classes/{ClassListPage.tsx,ClassStudentsPage.tsx,useClasses.ts}
├── students/{UnassignedStudentsPage.tsx,StudentLoginActivityPanel.tsx,useLoginEvents.ts}
├── sis/{SisImportPage.tsx,SisImportProgress.tsx,ReconciliationIssuesPage.tsx,ResolveIssueDialog.tsx,useSisImport.ts,sha256.ts}
├── users/{UserListPage.tsx,CreateStaffForm.tsx,UserRolesPanel.tsx,GrantUserRoleDialog.tsx,useUsers.ts,useUserRoles.ts}
├── roles/{RoleListPage.tsx,RoleEditor.tsx,PermissionPicker.tsx,useRoles.ts}
├── audit/{AuditLogPage.tsx,PermissionAuditPage.tsx,useAuditLogs.ts}
├── settings/{TenantSettingsPage.tsx,SchoolSettingsPage.tsx,useTenantSettings.ts,useSchoolSettings.ts}
└── student-cleanup/{CleanupPreviewPage.tsx,ExcludeDialog.tsx,useCleanup.ts}
src/cms/frontend/apps/portal/src/i18n/locales/{vi,en}/portal.school_lms.json
src/cms/frontend/apps/portal/tests/unit/school-lms/**/*.test.ts(x)
src/cms/frontend/apps/portal/tests/e2e/school-lms/*.spec.ts
```

**Structure Decision**: Một thư mục con theo module của workspace C ([ADMIN_PORTAL §10](../../docs/04-admin-portal/ADMIN_PORTAL.md) sitemap); hook API đặt cạnh màn, dùng `apiClient` + `withTenantScope` của F050/F051.

## Phase 0: Research

Không có — ADR và tài liệu sở hữu đủ; delta contract đã xác định ([contracts/README.md](contracts/README.md)).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md) (chỉ trạng thái client)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `sha256.ts`, `canGrant.ts` (lọc role người gán được cấp theo `listTenantRoles` + permission của mình — chỉ hiển thị), map `issue_type` → khóa i18n | Hàm thuần |
| application (client) | Hook query/mutation theo bảng dưới | Mutation ghi gửi `Idempotency-Key` sinh một lần cho mỗi ý định (giữ qua retry) |
| infrastructure | `apiClient` (F050), `withTenantScope` (F051), upload `PUT upload_url` bằng `fetch` không kèm `Authorization` | |
| presentation | Page + dialog trong cây trên | ProTable cursor; ProForm validation từ OpenAPI |

**Màn → operationId → permission (ẩn/hiện; server vẫn kiểm):**

| Màn | Đọc | Ghi | Permission đọc / ghi |
|---|---|---|---|
| Trường | `listTenantSchools`, `getTenantSchool`, `listProvinces` | `updateTenantSchool`, `approveTenantSchool` | `tenant.school.read` / `tenant.school.write` |
| Lớp, học sinh lớp | `listTenantSchoolClasses`, `listTenantClassStudents` (Cần thêm) | — | `tenant.class.read` |
| Học sinh chưa có lớp | `listUnassignedStudents` | — | `tenant.class.read` |
| Hoạt động đăng nhập | `listTenantStudentLoginEvents` | — | `lms.student_activity.read` |
| Nhập CSV SIS | `getSisImport` | `createSisImport` | `tenant.sis.read` / `tenant.sis.import` |
| Xung đột đồng bộ | `listSchoolReconciliationIssues` (Cần thêm), `listUnassignedStudents` | `resolveSchoolReconciliationIssue` (Cần thêm) | `tenant.sis.read` / `tenant.sis.import` |
| Người dùng | `listTenantUsers` | `createTenantUser`, `updateTenantUser` | `tenant.user.read` / `tenant.user.write` |
| Role | `listTenantRoles`, `getTenantRole`, `listTenantPermissions` | `createTenantRole`, `updateTenantRole`, `deleteTenantRole` | `tenant.role.read` / `tenant.role.write` |
| UserRole | `listTenantUserRoles`, `listSchoolUserRoles` | `grantTenantUserRole`, `grantSchoolUserRole`, `revokeTenantUserRole` | `tenant.user.read` / `tenant.user_role.write` |
| Nhật ký, nhật ký phân quyền | `listTenantAuditLogs` | — | `tenant.audit.read` |
| Cài đặt tenant | `getTenantSettings` | `updateTenantSettings` | `tenant.settings.read` / `tenant.settings.write` (+ `tenant.student_cleanup.configure`, `tenant.chat_retention.configure` cho trường tương ứng) |
| Cài đặt trường (gồm khung giới nghiêm trường, ADR-0078) | `getSchoolSettings` | `updateSchoolSettings` | `school.settings.read` / `school.settings.write` |
| Học sinh sắp bị dọn | `previewTenantStudentCleanup` | `createStudentCleanupExclusions`, `deleteStudentCleanupExclusions` | `tenant.student_cleanup.read` / `tenant.student_cleanup.exclude` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F051 (shell, `withTenantScope`, `TenantTargetField`, `LargeScreenRequired`, registry menu sinh từ `specs/portal/menu-catalog.yaml`, `<ActionGate>`), F050 (`apiClient`, `ResponsiveTable`), F010 (trường/lớp), F011 (SIS import, reconciliation), F012 (role/UserRole), F015 (audit, settings), F016 (cleanup).
- Không tiêu thụ event; không gọi `/internal/*`.

### Bảo mật, tenant, idempotency, audit

- Người dùng tenant: `X-Tenant-Id` = tenant phiên; System Admin: `requireSingleTenant` (F052-AC-24).
- Idempotency-Key cho `createSisImport`, `resolveSchoolReconciliationIssue`, `createTenantUser`, `createTenantRole`, `grantTenantUserRole`, `grantSchoolUserRole`, `createStudentCleanupExclusions`; `If-Match` cho mọi PATCH/DELETE có `IfMatch`.
- Audit ghi phía server (`role.*`, `user_role.*`, `school.*`, `tenant_settings.*`, cleanup); portal hiển thị qua `listTenantAuditLogs`.
- Không render IP, `ip_hash`, `device_id` dù API trả nhầm (lọc whitelist field khi map DTO → view).

### Quan sát

- Không log tên, email, username học sinh ra console; lỗi hiển thị `trace_id`.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.school_lms.schools`, `menu.school_lms.classes`, `menu.school_lms.sis_sync`, `menu.school_lms.student_activity`, `menu.school_lms.access_control`, `menu.school_lms.settings`, `menu.school_lms.audit_logs` |
| Action dùng | `action.school_lms.schools.view`, `action.school_lms.schools.edit`, `action.school_lms.classes.view`, `action.school_lms.sis_sync.view`, `action.school_lms.sis_sync.import`, `action.school_lms.student_activity.view`, `action.school_lms.access_control.view_users`, `action.school_lms.access_control.edit_users`, `action.school_lms.access_control.view_roles`, `action.school_lms.access_control.edit_roles`, `action.school_lms.access_control.view_user_roles`, `action.school_lms.access_control.assign_roles`, `action.school_lms.settings.view`, `action.school_lms.settings.edit`, `action.school_lms.settings.cleanup_view`, `action.school_lms.settings.cleanup_exclude`, `action.school_lms.audit_logs.view` |
| Không tắt được | `menu.school_lms.access_control` không tắt được (ADR-0090 mục 2) |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `listTenantClassStudents`, `listSchoolReconciliationIssues` (Cần thêm) → gắn `action.school_lms.classes.view` / `action.school_lms.sis_sync.view`
- `getSchoolSettings`, `updateSchoolSettings` (F015, đã có trong spec gộp) → gắn `action.school_lms.settings.view` / `action.school_lms.settings.edit_school`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
