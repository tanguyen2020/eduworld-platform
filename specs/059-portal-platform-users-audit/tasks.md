# Tasks: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Thêm route lazy `/portal/platform-admin/{permissions,roles,user-roles,users,audit,audit/permissions,login-events,feature-flags,integrations}` vào chunk D — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx` — **2026-11-20:** màn gắn vào khóa menu của catalog trong `src/cms/frontend/apps/portal/src/workspaces/platform-admin/index.tsx` (path của catalog, ADR-0090): `access-control?tab=users|roles|permissions|user-roles`, `audit-logs?tab=system|permissions`, `student-activity`, `feature-toggles`, `integrations` (OQ F059-10)
- [x] T002 [P] Mục menu theo permission (`platform.role.read`, `platform.user_role.write`, `platform.user.manage`, `platform.audit.read`, `platform.cross_tenant.read`, `platform.feature_flag.write`, `platform.sis_connection.manage`) + feature flag ẩn màn chờ delta; khóa menu từ `specs/portal/menu-catalog.yaml` (registry sinh, F051; không khai báo menu tay) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx` — **2026-11-20:** menu/action theo permission từ registry sinh (`WorkspaceRoutes`, `ActionGate`, tab ẩn theo `actionStatus`); phần chờ delta khóa bằng cờ `PLATFORM_USERS_AVAILABLE`, `INTEGRATION_OVERVIEW_AVAILABLE` (OQ F059-1, F059-2); kiểm `__tests__/IntegrationsGuard.test.tsx`
- [x] T003 [P] File dịch vi/en (`platform.permissions.*`, `platform.roles.*`, `platform.userRoles.*`, `platform.users.*`, `platform.audit.*`, `platform.loginEvents.*` gồm nhãn `LoginEndReason`, `platform.featureFlags.*`, `platform.integrations.*`, `error.PERMISSION_NOT_ALLOWED_FOR_SCOPE`, `error.ROLE_NOT_FOUND`, `error.USERNAME_TAKEN`, `error.EMAIL_TAKEN`) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/i18n/vi.json`, `en.json` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/i18n/{vi,en}.json` (`platform.access|permissions|roles|userRoles|users|audit|loginEvents|featureFlags|integrations|scopeKind|scopeType`, nhãn `LoginEndReason`) + `error.USERNAME_TAKEN`, `error.EMAIL_TAKEN`, `error.ROLE_NOT_FOUND`, `authz.*` ở file dịch chung; `PERMISSION_NOT_ALLOWED_FOR_SCOPE` hiển thị bằng `platform.roles.scopeNotAllowed`; kiểm `limits.test.ts` › translations, `i18n/keys.test.ts`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract (owner F012): `listPlatformUsers`, `createPlatformUser`, schema `PlatformUserCreateRequest` — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **2026-11-20:** chưa áp — owner F012, chưa có backend; không tự thêm contract (OQ F059-1) — **2026-11-20:** owner F012 áp (`listPlatformUsers`, `createPlatformUser`, `PlatformUserCreateRequest`, `User.tenant_ids[]`), có backend; xem OQ F059-1
- [x] T005 Delta contract (owner F012): `UserRoleGrantRequest.training_completed_at`; bỏ `class` khỏi `TenantUserCreateRequest.initial_user_roles[].scope_type` và mô tả `scope_id` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **2026-11-20:** một phần: `training_completed_at` đã có (owner F012 áp); `class` còn trong `TenantUserCreateRequest.initial_user_roles[].scope_type` — Portal không cho chọn (OQ F059-4) — **2026-11-20:** `class` đã không còn trong enum (F052); hoàn tất
- [x] T006 Delta contract (owner F016): `unlockUser`, `deactivateUser` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** đã áp bởi owner F016 (`unlockUser`, `deactivateUser` có trong `operation-meta.gen.ts`) (OQ F059-8)
- [x] T007 Delta contract (owner F015): `listFeatureFlags`, `putFeatureFlag`, schema `FeatureFlag`/`FeatureFlagList`/`FeatureFlagUpdate`; sửa `listPlatformAuditLogs` (`x-internal`, `security`, lọc tiền tố action) — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** một phần: `listFeatureFlags`/`putFeatureFlag` đã áp bởi owner F015 (hình dạng của owner, OQ F059-7); `listPlatformAuditLogs` có `x-internal` nhưng chưa lọc tiền tố action (OQ F059-3) — **2026-11-20:** owner F015 áp `action_prefix[]` + `security` cho `listPlatformAuditLogs` (OQ F059-3)
- [x] T008 Delta contract (owner F011): `listSisConnections`, `listSisSyncRuns`, schema `SisConnectionPage`, `SyncRunPage` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint` — **2026-11-17:** `listSisConnections`, `listSisSyncRuns`, `SisConnectionPage`, `SyncRunPage` đã áp bởi owner F011 (tên trường theo owner) — **2026-11-20:** xác nhận: `listSisConnections`, `listSisSyncRuns` có trong contract
- [x] T009 Delta contract (owner identity/F012): `getIntegrationOverview` + `IntegrationOverview`; ghi nguồn giá trị (cấu hình server, không secret) vào ADMIN_PORTAL §10 — file: `specs/openapi/modules/identity.v1.yaml`, `docs/04-admin-portal/ADMIN_PORTAL.md` — kiểm: `make specs-lint`, `make docs-check` — **2026-11-20:** chưa áp — owner identity/F012 (OQ F059-2); ADMIN_PORTAL §10 không đổi — **2026-11-20:** owner F012 áp `getIntegrationOverview` + `IntegrationOverview` (ADMIN_PORTAL §10 ghi nguồn: cấu hình server); `route_groups[]` chưa làm (OQ F059-12)
- [x] T010 Sinh lại client TS — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff — **2026-11-20:** F059 không đổi contract; `pnpm gen:check` khớp spec
- [x] T011 [P] Hàm thuần `groupPermissions`, `grantableRoles`, `endReasonLabel` + unit test — file: `src/cms/frontend/apps/portal/src/shared/authz/groupPermissions.ts`, `src/cms/frontend/apps/portal/src/workspaces/platform-admin/user-roles/grantableRoles.ts`, `src/cms/frontend/apps/portal/src/workspaces/platform-admin/login-events/endReasonLabel.ts` — **2026-11-20:** `src/shared/authz/groupPermissions.ts`, `src/cms/frontend/apps/portal/src/workspaces/platform-admin/user-roles/grantableRoles.ts`, `endReasonKey` trong `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/loginEvents.ts`; unit test `src/shared/authz/groupPermissions.test.ts` (9 test)
- [x] T012 [P] MSW handler test-only (nhãn `MOCK`) cho mọi operation ở contracts §1 — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/msw/f059Handlers.ts` — **2026-11-20:** backend MOCK `src/cms/frontend/apps/portal/src/test/platform-access-backend.ts` (nhãn MOCK, cắm sau mock F057) thay MSW (OQ F059-9)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Role platform và Permission catalog (Priority: P1) 🎯 MVP

**Goal**: Xem catalog, quản lý role platform/dùng chung. **Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T013 [P] [US1] Test F059-AC-01: catalog nhóm `group_code`, lọc `scope_kind`, `catalog_version`, không nút sửa — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/PermissionCatalogPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/AccessControlRoles.test.tsx` › Permission catalog
- [x] T014 [P] [US1] Test F059-AC-02: `listPlatformRoles` lọc `role_scope_kind` + `tenant_ids[]` (không `X-Tenant-Id`), nhãn "Hệ thống"/"Dùng chung" (TC-TEN-004) — file: `…/platform/__tests__/PlatformRoleListPage.test.tsx` — **2026-11-20:** `AccessControlRoles.test.tsx` › list without X-Tenant-Id…
- [x] T015 [P] [US1] Test F059-AC-03, F059-AC-04: picker theo `scope_kind`, `createPlatformRole` + `Idempotency-Key`, `updatePlatformRole` + `If-Match` + `reason`; hiển thị `PERMISSION_NOT_ALLOWED_FOR_SCOPE`/`412`/`409`/`403`; `is_system` chỉ "Nhân bản" (TC-AUTH-076, TC-CMS-RBAC-01, TC-CMS-RBAC-05) — file: `…/platform/__tests__/PlatformRoleEditor.test.tsx` — **2026-11-20:** `AccessControlRoles.test.tsx` › create / 422 PERMISSION_NOT_ALLOWED_FOR_SCOPE / edit (412, 403) / clone
- [x] T016 [P] [US1] Test F059-AC-05: xóa có xác nhận + lý do, thông điệp `principal_cache_ttl` + số người (TC-CMS-RBAC-04) — file: `…/platform/__tests__/PlatformRoleDelete.test.tsx` — **2026-11-20:** `AccessControlRoles.test.tsx` › delete (409, If-Match + reason), role deleted meanwhile (404)
- [ ] T017 [US1] E2E role: tạo → sửa → nhân bản role hệ thống → xóa — file: `src/cms/frontend/apps/portal/e2e/platform/roles.spec.ts` — **2026-11-20:** E2E backend thật chờ môi trường compose (OQ F059-9); luồng tương đương ở mức integration trong `AccessControlRoles.test.tsx`

### Implementation cho User Story 1

- [x] T018 [P] [US1] Hook permission/role — file: `…/platform/api/permissions.ts`, `…/platform/api/roles.ts` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/roles.ts` (catalog + role)
- [x] T019 [P] [US1] `PermissionPicker` dùng chung — file: `src/cms/frontend/apps/portal/src/shared/authz/PermissionPicker.tsx` — **2026-11-20:** `src/shared/authz/PermissionPicker.tsx`
- [x] T020 [US1] Trang catalog, danh sách role, editor, hộp nhân bản — file: `…/platform/permissions/PermissionCatalogPage.tsx`, `…/platform/roles/PlatformRoleListPage.tsx`, `PlatformRoleEditor.tsx`, `CloneSystemRoleDialog.tsx` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/permissions/PermissionCatalogPage.tsx`, `roles/PlatformRoleListPage.tsx`, `roles/PlatformRoleEditor.tsx` (nhân bản = editor chế độ `clone`, thay `CloneSystemRoleDialog.tsx`)

## Phase 4: User Story 2 — UserRole platform và tenant đích (Priority: P1)

### Tests cho User Story 2

- [x] T021 [P] [US2] Test F059-AC-06: `listAllUserRoles` lọc user/role/`tenant_ids[]`, đủ cột (TC-TEN-004) — file: `…/platform/__tests__/UserRoleListPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/UserRoles.test.tsx` › columns, filters…
- [x] T022 [P] [US2] Test F059-AC-07: người platform → chỉ role `platform`, không header; người có tenant gốc → role `tenant`, `TargetTenantField` theo membership, `X-Tenant-Id` = tenant đích, scope `tenant/school/subject` (TC-TEN-011c) — file: `…/platform/__tests__/GrantUserRoleDialog.scope.test.tsx` — **2026-11-20:** `UserRoles.test.tsx` › platform staff… / staff with a home tenant…
- [x] T023 [P] [US2] Test F059-AC-08: `Idempotency-Key`, `reason`, `valid_to > valid_from`; hiển thị `422`/`ROLE_NOT_FOUND`/`409` (TC-CMS-AUTHZ-03, TC-AUTH-077) — file: `…/platform/__tests__/GrantUserRoleDialog.errors.test.tsx` — **2026-11-20:** `UserRoles.test.tsx` › Idempotency-Key giữ khi mất mạng/thử lại, 409, 422 ROLE_NOT_FOUND; `valid_to > valid_from` = `validRange` (unit)
- [x] T024 [P] [US2] Test F059-AC-09: thu hồi có lý do, header tenant của UserRole (TC-CMS-RBAC-03) — file: `…/platform/__tests__/RevokeUserRoleDialog.test.tsx` — **2026-11-20:** `UserRoles.test.tsx` › revoke…
- [x] T025 [P] [US2] Test F059-AC-10: `safety_moderator` bắt buộc ngày đào tạo — file: `…/platform/__tests__/GrantUserRoleDialog.safety.test.tsx` — **2026-11-20:** `UserRoles.test.tsx` › safety_moderator needs the training date…
- [ ] T026 [US2] E2E gán → người được gán thấy menu ≤ 60 s → thu hồi — file: `src/cms/frontend/apps/portal/e2e/platform/user-roles.spec.ts` — **2026-11-20:** E2E backend thật chờ môi trường compose (OQ F059-9)

### Implementation cho User Story 2

- [x] T027 [P] [US2] Hook UserRole — file: `…/platform/api/userRoles.ts` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/userRoles.ts`
- [x] T028 [US2] Trang UserRole, hộp gán/thu hồi — file: `…/platform/user-roles/UserRoleListPage.tsx`, `GrantUserRoleDialog.tsx`, `RevokeUserRoleDialog.tsx` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/user-roles/UserRoleListPage.tsx`, `GrantUserRoleDialog.tsx` (thu hồi = `ReasonDialog` dùng chung, thay `RevokeUserRoleDialog.tsx`)

## Phase 5: User Story 3 — Người dùng mọi tenant (Priority: P2)

### Tests cho User Story 3

- [x] T029 [P] [US3] Test F059-AC-11: `listPlatformUsers` lọc `q`/`user_type`/`status`/`tenant_ids[]`/`platform_only`; nhãn SIS; học sinh không nút gán role (TC-CMS-RBAC-02) — file: `…/platform/__tests__/PlatformUserListPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/Users.test.tsx` › tenant accounts… (theo phạm vi contract có — OQ F059-1) — **2026-11-20 (portal):** `listPlatformUsers` thật: không chọn tenant = chỉ nhân sự platform, bộ lọc tenant → `tenant_ids[]` + cột "Thuộc tenant", `q`/loại/`status` (OQ F059-1) — `Users.test.tsx` › platform staff without tenant filter…, status filter
- [x] T030 [P] [US3] Test F059-AC-12: tạo nhân sự platform (`createPlatformUser`) / trường (`createTenantUser` + `X-Tenant-Id`), email bắt buộc, `409` + gợi ý (TC-CMS-LOGIN-04, TC-AUTH-081) — file: `…/platform/__tests__/CreateUserDialog.test.tsx` — **2026-11-20:** `Users.test.tsx` › Tạo nhân sự (nhân sự platform khóa tới khi có `createPlatformUser`) — **2026-11-20 (portal):** "Nhân sự platform" bật: `createPlatformUser` không `X-Tenant-Id`, role platform ban đầu (không `safety_moderator`), lý do, `Idempotency-Key`, `409 USERNAME_TAKEN` + gợi ý — `Users.test.tsx` › platform staff: createPlatformUser…
- [x] T031 [P] [US3] Test F059-AC-13: khóa có lý do + `Idempotency-Key`; nút mở khóa/vô hiệu hóa chỉ khi contract có — file: `…/platform/__tests__/LockUserDialog.test.tsx` — **2026-11-20:** `Users.test.tsx` › lock / unlock / deactivate
- [x] T032 [P] [US3] Test F059-AC-14: chi tiết UserRole theo tenant; DOM không có token/IP — file: `…/platform/__tests__/UserDetailDrawer.test.tsx` — **2026-11-20:** `Users.test.tsx` › detail drawer without secrets — **2026-11-20 (portal):** `Users.test.tsx` › detail drawer (tách test riêng)

### Implementation cho User Story 3

- [x] T033 [P] [US3] Hook người dùng — file: `…/platform/api/users.ts` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/users.ts` — **2026-11-20 (portal):** `usePlatformUsers`, `useCreatePlatformUser`; cờ `PLATFORM_USERS_AVAILABLE` đã gỡ
- [x] T034 [US3] Trang người dùng, ngăn chi tiết, hộp tạo/khóa — file: `…/platform/users/PlatformUserListPage.tsx`, `UserDetailDrawer.tsx`, `CreateUserDialog.tsx`, `LockUserDialog.tsx` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/users/PlatformUserPage.tsx`, `UserDetailDrawer.tsx`, `CreateUserDialog.tsx` (khóa = `ReasonDialog` trong ngăn chi tiết) — **2026-11-20 (portal):** `PlatformUserPage.tsx` theo `listPlatformUsers` (bỏ xem theo một tenant bằng `listTenantUsers`), `CreateUserDialog.tsx` hai loại theo action; gán role từ dòng chỉ cho tenant trong `User.tenant_ids` (OQ F059-5) — `Users.test.tsx` › grant from a row

## Phase 6: User Story 4 — Audit toàn hệ thống (Priority: P1)

### Tests cho User Story 4

- [x] T035 [P] [US4] Test F059-AC-15: `from`/`to` bắt buộc (mặc định 24 giờ), lọc đủ tham số, ngăn chi tiết `changes` escape, `trace_id` (TC-CMS-TEN-02) — file: `…/platform/__tests__/PlatformAuditLogPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/AuditLogs.test.tsx` › Nhật ký toàn hệ thống
- [x] T036 [P] [US4] Test F059-AC-16: nhật ký phân quyền chỉ `role.*`/`user_role.*` — file: `…/platform/__tests__/PermissionAuditPage.test.tsx` — **2026-11-20:** `AuditLogs.test.tsx` › Nhật ký phân quyền (OQ F059-3) — **2026-11-20 (portal):** một truy vấn `action_prefix=role&action_prefix=user_role`, "Hành động" tùy chọn (AND) — `AuditLogs.test.tsx` › Nhật ký phân quyền (OQ F059-3)
- [x] T037 [P] [US4] Test F059-AC-17: `422` khoảng thời gian giữ bộ lọc; không nút sửa/xóa — file: `…/platform/__tests__/PlatformAuditLogPage.errors.test.tsx` — **2026-11-20:** `AuditLogs.test.tsx` › 422 over the maximum range

### Implementation cho User Story 4

- [x] T038 [US4] Hook + trang audit, nhật ký phân quyền, ngăn chi tiết — file: `…/platform/api/audit.ts`, `…/platform/audit/PlatformAuditLogPage.tsx`, `PermissionAuditPage.tsx`, `AuditDetailDrawer.tsx` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/audit.ts`, `audit/AuditLogsPage.tsx` (hai tab), `audit/AuditDetailDrawer.tsx` — **2026-11-20 (portal):** `api/audit.ts` `PERMISSION_AUDIT_PREFIXES` → `action_prefix[]`

## Phase 7: User Story 5 — Đăng nhập game liên tenant (Priority: P2)

### Tests cho User Story 5

- [x] T039 [P] [US5] Test F059-AC-18: `listLoginEventsCrossTenant` lọc `tenant_ids[]`/`student_id`/7 ngày, nhãn `end_reason` (TC-TEN-018) — file: `…/platform/__tests__/LoginEventsPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/LoginEventsFlags.test.tsx` › Đăng nhập game liên tenant
- [x] T040 [P] [US5] Test F059-AC-19: không có IP trong DOM, phân trang cursor — file: `…/platform/__tests__/LoginEventsPage.privacy.test.tsx` — **2026-11-20:** `LoginEventsFlags.test.tsx` (không IP trong DOM, cursor "Tải thêm")

### Implementation cho User Story 5

- [x] T041 [US5] Hook + trang đăng nhập game — file: `…/platform/api/loginEvents.ts`, `…/platform/login-events/LoginEventsPage.tsx` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/loginEvents.ts`, `login-events/LoginEventsPage.tsx`

## Phase 8: User Story 6 — Feature flag (Priority: P3)

### Tests cho User Story 6

- [x] T042 [P] [US6] Test F059-AC-20, F059-AC-21: danh sách, sửa mặc định + ghi đè tenant, `If-Match` + `reason`, `412` tải lại; không tạo/xóa flag — file: `…/platform/__tests__/FeatureFlagEditor.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `LoginEventsFlags.test.tsx` › Feature flag

### Implementation cho User Story 6

- [x] T043 [US6] Hook + trang feature flag — file: `…/platform/api/featureFlags.ts`, `…/platform/feature-flags/FeatureFlagListPage.tsx`, `FeatureFlagEditor.tsx` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/featureFlags.ts`, `feature-flags/FeatureFlagListPage.tsx`, `FeatureFlagEditor.tsx`

## Phase 9: User Story 7 — Tích hợp chỉ xem (Priority: P3)

### Tests cho User Story 7

- [x] T044 [P] [US7] Test F059-AC-22: panel Keycloak/APISIX từ `getIntegrationOverview`, tab SIS `listSisConnections`/`listSisConnectionSchools`/`listSisSyncRuns` — file: `…/platform/__tests__/IntegrationsPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/IntegrationsGuard.test.tsx` › Tích hợp chỉ xem (panel Keycloak/APISIX chờ OQ F059-2) — **2026-11-20 (portal):** tab Keycloak · APISIX từ `getIntegrationOverview` (issuer, realm, client, MFA, URL gateway, "Chưa cấu hình", nhóm route "Chưa có dữ liệu" — OQ F059-12, SIS theo trạng thái); trường ngoài schema `client_secret` không vào DOM — `IntegrationsGuard.test.tsx` › Tích hợp
- [x] T045 [P] [US7] Test F059-AC-23: không ô nhập, không render trường ngoài schema; E2E quét secret trong response/DOM — file: `src/cms/frontend/apps/portal/e2e/platform/integrations.spec.ts` — **2026-11-20:** mức integration: `IntegrationsGuard.test.tsx` (trường ngoài schema `secret_ref`/`api_token` không vào DOM, không ô nhập); quét response ở E2E chờ compose (OQ F059-9)

### Implementation cho User Story 7

- [x] T046 [US7] Hook + trang Tích hợp — file: `…/platform/api/integrations.ts`, `…/platform/api/sisConnections.ts`, `…/platform/integrations/IntegrationsPage.tsx`, `KeycloakPanel.tsx`, `ApisixPanel.tsx`, `SisConnectionsTab.tsx` — **2026-11-20:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/integrations.ts`, `integrations/IntegrationsPage.tsx`, `integrations/SisConnectionsTab.tsx` (panel Keycloak/APISIX chờ OQ F059-2) — **2026-11-20 (portal):** `useIntegrationOverview`, `integrations/IntegrationOverviewTab.tsx`; cờ `INTEGRATION_OVERVIEW_AVAILABLE` đã gỡ

## Phase 10: User Story 8 — Bảo vệ màn và đầu vào (Priority: P1)

### Tests cho User Story 8

- [x] T047 [P] [US8] E2E F059-AC-24: `ad.truong.a`, `cm.noidung`, `sm.antoan` mở URL màn F059 → 403, không tải chunk D; menu ẩn theo permission (TC-CMS-AUTHZ-05, TC-CMS-WS-01, TC-CMS-WS-03) — file: `src/cms/frontend/apps/portal/e2e/platform/authz-f059.spec.ts` — phải FAIL trước khi code — **2026-11-20:** mức integration: `IntegrationsGuard.test.tsx` › Bảo vệ màn F059 (403, chunk D không tải, không gọi `/admin/*`; menu ẩn khi thiếu permission)
- [x] T048 [P] [US8] Test F059-AC-25: `MFA_REQUIRED` → step-up, lỗi mạng → banner VPN, `PERMISSION_DENIED` + `trace_id` (TC-CMS-WS-02) — file: `…/platform/__tests__/f059Errors.test.tsx` — **2026-11-20:** `IntegrationsGuard.test.tsx` › Lỗi quyền, MFA; VPN dùng chung shell (F057 `workspaceGuard.test.tsx`)
- [x] T049 [P] [US8] Test F059-AC-26: mọi POST ghi có `Idempotency-Key` giữ khi retry; PATCH/PUT/DELETE có `If-Match` — file: `…/platform/__tests__/f059Idempotency.test.ts` — **2026-11-20:** kiểm trong các test màn: `Idempotency-Key` (tạo role, gán UserRole — giữ khi mất mạng/thử lại, tạo người dùng, khóa), `If-Match` (sửa/xóa role, feature flag)
- [x] T050 [P] [US8] Test F059-AC-27: ô nhập theo `maxLength`/`pattern` + bộ đếm (TC-TEN-024) — file: `…/platform/__tests__/f059InputLimits.test.tsx` — **2026-11-20:** `limits.test.ts` (mọi giới hạn F059 = OpenAPI) + `maxlength`/bộ đếm trong test màn

### Implementation cho User Story 8

- [x] T051 [US8] Gắn guard permission từng route, nối `MfaStepUp`/`VpnRequiredBanner` (F057) vào mọi page F059 — file: `…/platform/routes.tsx` — **2026-11-20:** guard theo catalog (`MenuGate`/`ActionGate`), MFA/VPN của shell áp cho mọi màn workspace D

## Phase N: Polish & Cross-Cutting

- [x] T052 [P] Kiểm không import component/Theme game (TC-UI-010), lint hardcode style (TC-UI-003) — kiểm: `make web-lint` — **2026-11-20:** `pnpm lint` xanh (không import Theme game, stylelint/ESLint chặn hardcode)
- [ ] T053 [P] axe + visual regression màn F059 sáng/tối (TC-UI-004, TC-UI-006) — file: `src/cms/frontend/apps/portal/e2e/platform/a11y-visual-f059.spec.ts` — **2026-11-20:** axe + visual sáng/tối chờ môi trường Playwright/compose (OQ F059-9)
- [x] T054 [P] Báo mâu thuẫn permission audit (`platform.audit.read` vs `platform.audit.read_all`) để F012 chốt; cập nhật ADMIN_PORTAL §2.2 khi chốt — file: `docs/04-admin-portal/ADMIN_PORTAL.md` — **2026-11-20:** ghi OQ F059-11 để F012 chốt; ADMIN_PORTAL §2.2 không sửa khi chưa chốt
- [x] T055 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md` — **2026-11-20:** `CHANGELOG.md`
- [ ] T056 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng — **2026-11-20:** chạy ở mức integration (bằng chứng trong quickstart.md); backend thật chờ compose (OQ F059-9)
- [x] T057 [P] Test F059-AC-28: tắt `menu.platform_admin.student_activity` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/platform-admin/users-audit-feature-toggle.spec.ts` — **2026-11-20:** mức integration: `IntegrationsGuard.test.tsx` › Menu/action tắt
- [x] T058 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F059-AC-28) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx` — **2026-11-20:** không đăng ký menu tay; nút bọc `<ActionGate actionKey>` theo bảng khóa plan.md; `node scripts/specs/check-portal-catalog.mjs` 0 lỗi

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T059 **OQ chốt 2026-11-20** (OQ F059-12) Bảng `route_groups[]` ở tab "Keycloak · APISIX" (F059-AC-22), sau task F012 — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/integrations/` — kiểm: `make web-test` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational → US8 + US1 (MVP) → US2 → US4 → US3 (cần T004, T006) → US5 → US6 (cần T007) → US7 (cần T008, T009) → Polish.
- F057 phải Done (chunk D, `TenantFilter`, `MfaStepUp`, `VpnRequiredBanner`); backend F012/F015 Done cho US1, US2, US4.

## Definition of Done

- [x] Mọi tiêu chí F059-AC-* có test pass (ghi mã test/TC) — **2026-11-20:** mức integration (portal đầy đủ + MOCK theo contract), bảng trong quickstart.md §4; AC-11/12 (danh sách mọi tenant, nhân sự platform) và AC-22 (Keycloak/APISIX) trong phạm vi contract hiện có — OQ F059-1, F059-2
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật qua gateway nội bộ — **2026-11-20:** lint + Vitest xanh; Playwright backend thật chờ compose (OQ F059-9)
- [x] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17) — MOCK chỉ trong `src/test/`, có nhãn
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — không đổi tài liệu sở hữu/CONFIG_DEFAULTS (chỉ đọc `principal_cache_ttl`, `audit_query_max_range_days`); CHANGELOG
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
