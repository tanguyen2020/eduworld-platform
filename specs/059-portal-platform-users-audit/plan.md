# Implementation Plan: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Branch**: `059-portal-platform-users-audit` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm 9 màn vào chunk workspace D (`src/cms/frontend/apps/portal/src/workspaces/platform-admin/`, do F057 tạo): Permission catalog (chỉ xem), Role platform/dùng chung (+ xem role tùy biến của tenant), UserRole (platform + tenant đích), Người dùng mọi tenant, Nhật ký toàn hệ thống, Nhật ký phân quyền, Đăng nhập game liên tenant, Feature flag, Tích hợp (Keycloak/SIS/APISIX chỉ xem). Gọi `/api/v1/admin/*` qua gateway nội bộ với token `eduworld-portal` của role platform, dùng lại `TenantFilter`, `TargetTenantField`, `useCrossTenantScope`, `VpnRequiredBanner` của F057. Không có logic nghiệp vụ ở client: picker permission/role chỉ lọc hiển thị, server quyết định (`422`/`403`). Delta contract (owner F011, F012, F015, F016, identity) đưa vào Phase 2; màn phụ thuộc delta ẩn sau feature flag tới khi contract có.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, ProForm, ModalForm, Drawer, Descriptions, Transfer/Tree cho picker), TanStack Query, openapi-typescript, React Hook Form + Zod, i18next |
| Storage | Không (RAM/TanStack Query); bộ lọc tenant theo F057 (`sessionStorage`) |
| Testing | Vitest + Testing Library + MSW (test-only, nhãn `MOCK`), Playwright E2E với backend compose qua gateway nội bộ local |
| Target Platform | Trình duyệt desktop/tablet ≥ 1024 px cho form; mobile chỉ xem danh sách |
| Project Type | web app (workspace D) |
| Performance Goals | Danh sách phân trang cursor; tìm kiếm debounce 300 ms; picker permission ≤ 1000 mục (`PermissionList.maxItems`) render ảo hóa |
| Constraints | AGENTS R1, R3, R10, R14, R20, R28, R32; ADR-0027, ADR-0031, ADR-0041, ADR-0047, ADR-0065, ADR-0066, ADR-0069, ADR-0072, ADR-0081 |
| Scale/Scope | Hàng trăm tenant, hàng trăm nghìn người dùng, audit nhiều triệu dòng (chỉ truy vấn theo khoảng thời gian) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready; delta `listPlatformUsers`, `createPlatformUser`, `training_completed_at`, `listFeatureFlags`/`putFeatureFlag`, `unlockUser`/`deactivateUser`, `listSisConnections`/`listSisSyncRuns`, `getIntegrationOverview`, sửa `listPlatformAuditLogs` là task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Frontend gọi API identity/game qua APISIX nội bộ; không đọc DB, không logic nghiệp vụ |
| III Cô lập tenant, an toàn dữ liệu | ✅ | List gửi `tenant_ids[]` không `X-Tenant-Id`; ghi tenant gửi `X-Tenant-Id` tenant đích; xóa role là soft-delete; audit chỉ đọc |
| IV Server-authoritative | ✅ | Kiểm tra scope/permission ở server; Portal hiển thị `code` lỗi |
| V Xác thực, phân quyền | ✅ | Chỉ `system_admin`; permission từng màn; 403 kèm `trace_id` (không step-up — ADR-0094); test ma trận 403 |
| VI An toàn trẻ em | ✅ | Không hiển thị IP, token; login events chỉ phục vụ System Admin; không log lý do/PII ở client |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn từ schema OpenAPI; URL gateway từ env; tham số theo tên CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Hook `api/*.ts` gọi API; component theo theme `admin-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F059-AC có test viết trước; E2E authz + tenant |

## Project Structure

### Documentation (feature này)

```text
specs/059-portal-platform-users-audit/
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
src/cms/frontend/apps/portal/src/shared/authz/
├── groupPermissions.ts            # nhóm PermissionList theo group_code, lọc scope_kind (hàm thuần)
└── PermissionPicker.tsx           # picker chỉ đọc catalog; workspace C (F052) có thể dùng lại
src/cms/frontend/apps/portal/src/workspaces/platform-admin/
├── routes.tsx                     # thêm route F059 (file của F057)
├── (không có menu.ts)             # mục menu = khóa `menu.platform_admin.*` của catalog, lọc theo permission (F059-AC-24, ADR-0090)
├── api/{permissions,roles,userRoles,users,audit,loginEvents,featureFlags,integrations,sisConnections}.ts
├── permissions/PermissionCatalogPage.tsx
├── roles/{PlatformRoleListPage,PlatformRoleEditor,CloneSystemRoleDialog}.tsx
├── user-roles/{UserRoleListPage,GrantUserRoleDialog,RevokeUserRoleDialog}.tsx
├── users/{PlatformUserListPage,UserDetailDrawer,CreateUserDialog,LockUserDialog}.tsx
├── audit/{PlatformAuditLogPage,PermissionAuditPage,AuditDetailDrawer}.tsx
├── login-events/LoginEventsPage.tsx
├── feature-flags/{FeatureFlagListPage,FeatureFlagEditor}.tsx
├── integrations/{IntegrationsPage,KeycloakPanel,ApisixPanel,SisConnectionsTab}.tsx
├── i18n/{vi,en}.json              # thêm khóa platform.users.*, platform.roles.*, platform.audit.*…
└── __tests__/
src/cms/frontend/apps/portal/e2e/platform/{roles,user-roles,users,audit,login-events,feature-flags,integrations,authz-f059}.spec.ts
```

**Structure Decision**: Màn đặt trong chunk D của F057 (ADR-0027 mục 7). `PermissionPicker`/`groupPermissions` đặt ở `src/shared/authz` vì workspace C (F052) và D cùng dùng catalog; không đặt ở `ops-ui` vì phụ thuộc kiểu sinh từ OpenAPI.

## Phase 0: Research

Không có. Quy tắc role/UserRole ở [ADMIN_PORTAL §2, §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md), audit ở §7, login events ở §3.10, liên tenant ở §2.5; API ở [API_DESIGN §9.16](../../docs/07-backend/API_DESIGN.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `groupPermissions.ts`, `grantableRoles.ts` (lọc role theo `primary_tenant_id` của người được gán: null → role `platform`; có → role `tenant`), `endReasonLabel.ts` | Hàm thuần, chỉ hiển thị; server vẫn kiểm |
| application | Hook `api/*.ts` (TanStack Query); mutation sinh `Idempotency-Key` một lần/thao tác; giữ ETag cho `If-Match` | Polling không cần |
| infrastructure | `apiClient` (F050), base URL `VITE_EW_INTERNAL_API_BASE_URL` (F057), `useCrossTenantScope` (F057) | Không hardcode URL (R3) |
| presentation | Page/dialog gọi: `listAllPermissions`, `listPlatformRoles`, `getPlatformRole`, `createPlatformRole`, `updatePlatformRole`, `deletePlatformRole`, `listAllUserRoles`, `grantPlatformUserRole`, `revokePlatformUserRole`, `createTenantUser`, `lockUser`, `listPlatformAuditLogs`, `listLoginEventsCrossTenant`, `listTenants`, `listSisConnectionSchools`; sau delta: `listPlatformUsers`, `createPlatformUser`, `unlockUser`, `deactivateUser`, `listFeatureFlags`, `putFeatureFlag`, `getIntegrationOverview`, `listSisConnections`, `listSisSyncRuns` | |

### Quy ước request

| Thao tác | Header / tham số |
|---|---|
| List liên tenant (role, UserRole, người dùng, audit, login events, SIS connection) | Không `X-Tenant-Id`; `tenant_ids[]` theo `TenantFilter` |
| Role platform/dùng chung, UserRole `platform`, tạo nhân sự platform | Không `X-Tenant-Id` |
| UserRole tenant, tạo nhân sự trường, thu hồi UserRole tenant, khóa người có tenant gốc | `X-Tenant-Id` = tenant đích / tenant của bản ghi |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F012 (role/UserRole/permission/user API), F015 (audit, feature flag), F057 (chunk D, `TenantFilter`, lỗi VPN), F014 (login events), F016 (lock/unlock/deactivate), F011/F017 (SIS connection), F050, F051.
- Typed client: client TS sinh từ `specs/openapi/eduworld-api.v1.yaml` (`pnpm gen:api`).

### Bảo mật, tenant, idempotency, audit

- Permission: `platform.role.read`, `platform.role.write`, `platform.user_role.write`, `platform.user.manage`, `platform.audit.read`, `platform.cross_tenant.read`, `platform.feature_flag.write`, `platform.sis_connection.manage`.
- Idempotency-Key: `createPlatformRole`, `grantPlatformUserRole`, `createPlatformUser`, `createTenantUser`, `lockUser`, `unlockUser`, `deactivateUser`; `If-Match`: `updatePlatformRole`, `deletePlatformRole`, `putFeatureFlag`.
- Audit ghi ở backend (`role.*`, `user_role.*`, `user.*`, `cross_tenant.accessed`, `platform_settings.changed`); Portal chỉ đọc.

### Quan sát

- Log client: `code`, `trace_id`, route; không username/email/lý do nhập tay; đếm lỗi `403` theo màn (metric web RUM nếu F050 có).


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.platform_admin.access_control`, `menu.platform_admin.audit_logs`, `menu.platform_admin.student_activity`, `menu.platform_admin.feature_toggles`, `menu.platform_admin.integrations` |
| Action dùng | `action.platform_admin.access_control.view_roles`, `action.platform_admin.access_control.edit_roles`, `action.platform_admin.access_control.view_user_roles`, `action.platform_admin.access_control.assign_roles`, `action.platform_admin.access_control.lock_user`, `action.platform_admin.audit_logs.view`, `action.platform_admin.student_activity.view`, `action.platform_admin.feature_toggles.view`, `action.platform_admin.feature_toggles.update`, `action.platform_admin.integrations.view`, `action.platform_admin.tenants.view`, `action.platform_admin.tenants.sis_connections`, `action.school_lms.access_control.edit_users`, `action.platform_admin.access_control.view_users`, `action.platform_admin.access_control.edit_users` |
| Không tắt được | `menu.platform_admin.access_control`, `menu.platform_admin.audit_logs`, `menu.platform_admin.feature_toggles` không tắt được (ADR-0090 mục 2) |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `listPlatformUsers`, `createPlatformUser`, `unlockUser` (Cần thêm trong OpenAPI) → gắn `x-portal-action` khi thêm: `action.platform_admin.access_control.view_users` / `edit_users` (`unlockUser` → `edit_users` hoặc `lock_user`); hai action đã có trong catalog, UI-only tới khi có operation
- `getIntegrationOverview`, `listSisConnections`, `listSisSyncRuns` (Cần thêm) → đề xuất `action.platform_admin.integrations.view` / `action.platform_admin.tenants.sis_connections`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
