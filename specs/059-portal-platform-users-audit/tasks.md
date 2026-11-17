# Tasks: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Thêm route lazy `/portal/platform-admin/{permissions,roles,user-roles,users,audit,audit/permissions,login-events,feature-flags,integrations}` vào chunk D — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx`
- [ ] T002 [P] Mục menu theo permission (`platform.role.read`, `platform.user_role.write`, `platform.user.manage`, `platform.audit.read`, `platform.cross_tenant.read`, `platform.feature_flag.write`, `platform.sis_connection.manage`) + feature flag ẩn màn chờ delta; khóa menu từ `specs/portal/menu-catalog.yaml` (registry sinh, F051; không khai báo menu tay) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx`
- [ ] T003 [P] File dịch vi/en (`platform.permissions.*`, `platform.roles.*`, `platform.userRoles.*`, `platform.users.*`, `platform.audit.*`, `platform.loginEvents.*` gồm nhãn `LoginEndReason`, `platform.featureFlags.*`, `platform.integrations.*`, `error.PERMISSION_NOT_ALLOWED_FOR_SCOPE`, `error.ROLE_NOT_FOUND`, `error.USERNAME_TAKEN`, `error.EMAIL_TAKEN`) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/i18n/vi.json`, `en.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract (owner F012): `listPlatformUsers`, `createPlatformUser`, schema `PlatformUserCreateRequest` — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 Delta contract (owner F012): `UserRoleGrantRequest.training_completed_at`; bỏ `class` khỏi `TenantUserCreateRequest.initial_user_roles[].scope_type` và mô tả `scope_id` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T006 Delta contract (owner F016): `unlockUser`, `deactivateUser` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`
- [ ] T007 Delta contract (owner F015): `listFeatureFlags`, `putFeatureFlag`, schema `FeatureFlag`/`FeatureFlagList`/`FeatureFlagUpdate`; sửa `listPlatformAuditLogs` (`x-internal`, `security`, lọc tiền tố action) — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`
- [ ] T008 Delta contract (owner F011): `listSisConnections`, `listSisSyncRuns`, schema `SisConnectionPage`, `SyncRunPage` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint` — **2026-11-17:** `listSisConnections`, `listSisSyncRuns`, `SisConnectionPage`, `SyncRunPage` đã áp bởi owner F011 (tên trường theo owner)
- [ ] T009 Delta contract (owner identity/F012): `getIntegrationOverview` + `IntegrationOverview`; ghi nguồn giá trị (cấu hình server, không secret) vào ADMIN_PORTAL §10 — file: `specs/openapi/modules/identity.v1.yaml`, `docs/04-admin-portal/ADMIN_PORTAL.md` — kiểm: `make specs-lint`, `make docs-check`
- [ ] T010 Sinh lại client TS — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff
- [ ] T011 [P] Hàm thuần `groupPermissions`, `grantableRoles`, `endReasonLabel` + unit test — file: `src/cms/frontend/apps/portal/src/shared/authz/groupPermissions.ts`, `src/cms/frontend/apps/portal/src/workspaces/platform-admin/user-roles/grantableRoles.ts`, `src/cms/frontend/apps/portal/src/workspaces/platform-admin/login-events/endReasonLabel.ts`
- [ ] T012 [P] MSW handler test-only (nhãn `MOCK`) cho mọi operation ở contracts §1 — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/msw/f059Handlers.ts`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Role platform và Permission catalog (Priority: P1) 🎯 MVP

**Goal**: Xem catalog, quản lý role platform/dùng chung. **Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T013 [P] [US1] Test F059-AC-01: catalog nhóm `group_code`, lọc `scope_kind`, `catalog_version`, không nút sửa — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/PermissionCatalogPage.test.tsx` — phải FAIL trước khi code
- [ ] T014 [P] [US1] Test F059-AC-02: `listPlatformRoles` lọc `role_scope_kind` + `tenant_ids[]` (không `X-Tenant-Id`), nhãn "Hệ thống"/"Dùng chung" (TC-TEN-004) — file: `…/platform/__tests__/PlatformRoleListPage.test.tsx`
- [ ] T015 [P] [US1] Test F059-AC-03, F059-AC-04: picker theo `scope_kind`, `createPlatformRole` + `Idempotency-Key`, `updatePlatformRole` + `If-Match` + `reason`; hiển thị `PERMISSION_NOT_ALLOWED_FOR_SCOPE`/`412`/`409`/`403`; `is_system` chỉ "Nhân bản" (TC-AUTH-076, TC-CMS-RBAC-01, TC-CMS-RBAC-05) — file: `…/platform/__tests__/PlatformRoleEditor.test.tsx`
- [ ] T016 [P] [US1] Test F059-AC-05: xóa có xác nhận + lý do, thông điệp `principal_cache_ttl` + số người (TC-CMS-RBAC-04) — file: `…/platform/__tests__/PlatformRoleDelete.test.tsx`
- [ ] T017 [US1] E2E role: tạo → sửa → nhân bản role hệ thống → xóa — file: `src/cms/frontend/apps/portal/e2e/platform/roles.spec.ts`

### Implementation cho User Story 1

- [ ] T018 [P] [US1] Hook permission/role — file: `…/platform/api/permissions.ts`, `…/platform/api/roles.ts`
- [ ] T019 [P] [US1] `PermissionPicker` dùng chung — file: `src/cms/frontend/apps/portal/src/shared/authz/PermissionPicker.tsx`
- [ ] T020 [US1] Trang catalog, danh sách role, editor, hộp nhân bản — file: `…/platform/permissions/PermissionCatalogPage.tsx`, `…/platform/roles/PlatformRoleListPage.tsx`, `PlatformRoleEditor.tsx`, `CloneSystemRoleDialog.tsx`

## Phase 4: User Story 2 — UserRole platform và tenant đích (Priority: P1)

### Tests cho User Story 2

- [ ] T021 [P] [US2] Test F059-AC-06: `listAllUserRoles` lọc user/role/`tenant_ids[]`, đủ cột (TC-TEN-004) — file: `…/platform/__tests__/UserRoleListPage.test.tsx` — phải FAIL trước khi code
- [ ] T022 [P] [US2] Test F059-AC-07: người platform → chỉ role `platform`, không header; người có tenant gốc → role `tenant`, `TargetTenantField` theo membership, `X-Tenant-Id` = tenant đích, scope `tenant/school/subject` (TC-TEN-011c) — file: `…/platform/__tests__/GrantUserRoleDialog.scope.test.tsx`
- [ ] T023 [P] [US2] Test F059-AC-08: `Idempotency-Key`, `reason`, `valid_to > valid_from`; hiển thị `422`/`ROLE_NOT_FOUND`/`409` (TC-CMS-AUTHZ-03, TC-AUTH-077) — file: `…/platform/__tests__/GrantUserRoleDialog.errors.test.tsx`
- [ ] T024 [P] [US2] Test F059-AC-09: thu hồi có lý do, header tenant của UserRole (TC-CMS-RBAC-03) — file: `…/platform/__tests__/RevokeUserRoleDialog.test.tsx`
- [ ] T025 [P] [US2] Test F059-AC-10: `safety_moderator` bắt buộc ngày đào tạo — file: `…/platform/__tests__/GrantUserRoleDialog.safety.test.tsx`
- [ ] T026 [US2] E2E gán → người được gán thấy menu ≤ 60 s → thu hồi — file: `src/cms/frontend/apps/portal/e2e/platform/user-roles.spec.ts`

### Implementation cho User Story 2

- [ ] T027 [P] [US2] Hook UserRole — file: `…/platform/api/userRoles.ts`
- [ ] T028 [US2] Trang UserRole, hộp gán/thu hồi — file: `…/platform/user-roles/UserRoleListPage.tsx`, `GrantUserRoleDialog.tsx`, `RevokeUserRoleDialog.tsx`

## Phase 5: User Story 3 — Người dùng mọi tenant (Priority: P2)

### Tests cho User Story 3

- [ ] T029 [P] [US3] Test F059-AC-11: `listPlatformUsers` lọc `q`/`user_type`/`status`/`tenant_ids[]`/`platform_only`; nhãn SIS; học sinh không nút gán role (TC-CMS-RBAC-02) — file: `…/platform/__tests__/PlatformUserListPage.test.tsx` — phải FAIL trước khi code
- [ ] T030 [P] [US3] Test F059-AC-12: tạo nhân sự platform (`createPlatformUser`) / trường (`createTenantUser` + `X-Tenant-Id`), email bắt buộc, `409` + gợi ý (TC-CMS-LOGIN-04, TC-AUTH-081) — file: `…/platform/__tests__/CreateUserDialog.test.tsx`
- [ ] T031 [P] [US3] Test F059-AC-13: khóa có lý do + `Idempotency-Key`; nút mở khóa/vô hiệu hóa chỉ khi contract có — file: `…/platform/__tests__/LockUserDialog.test.tsx`
- [ ] T032 [P] [US3] Test F059-AC-14: chi tiết UserRole theo tenant; DOM không có token/IP — file: `…/platform/__tests__/UserDetailDrawer.test.tsx`

### Implementation cho User Story 3

- [ ] T033 [P] [US3] Hook người dùng — file: `…/platform/api/users.ts`
- [ ] T034 [US3] Trang người dùng, ngăn chi tiết, hộp tạo/khóa — file: `…/platform/users/PlatformUserListPage.tsx`, `UserDetailDrawer.tsx`, `CreateUserDialog.tsx`, `LockUserDialog.tsx`

## Phase 6: User Story 4 — Audit toàn hệ thống (Priority: P1)

### Tests cho User Story 4

- [ ] T035 [P] [US4] Test F059-AC-15: `from`/`to` bắt buộc (mặc định 24 giờ), lọc đủ tham số, ngăn chi tiết `changes` escape, `trace_id` (TC-CMS-TEN-02) — file: `…/platform/__tests__/PlatformAuditLogPage.test.tsx` — phải FAIL trước khi code
- [ ] T036 [P] [US4] Test F059-AC-16: nhật ký phân quyền chỉ `role.*`/`user_role.*` — file: `…/platform/__tests__/PermissionAuditPage.test.tsx`
- [ ] T037 [P] [US4] Test F059-AC-17: `422` khoảng thời gian giữ bộ lọc; không nút sửa/xóa — file: `…/platform/__tests__/PlatformAuditLogPage.errors.test.tsx`

### Implementation cho User Story 4

- [ ] T038 [US4] Hook + trang audit, nhật ký phân quyền, ngăn chi tiết — file: `…/platform/api/audit.ts`, `…/platform/audit/PlatformAuditLogPage.tsx`, `PermissionAuditPage.tsx`, `AuditDetailDrawer.tsx`

## Phase 7: User Story 5 — Đăng nhập game liên tenant (Priority: P2)

### Tests cho User Story 5

- [ ] T039 [P] [US5] Test F059-AC-18: `listLoginEventsCrossTenant` lọc `tenant_ids[]`/`student_id`/7 ngày, nhãn `end_reason` (TC-TEN-018) — file: `…/platform/__tests__/LoginEventsPage.test.tsx` — phải FAIL trước khi code
- [ ] T040 [P] [US5] Test F059-AC-19: không có IP trong DOM, phân trang cursor — file: `…/platform/__tests__/LoginEventsPage.privacy.test.tsx`

### Implementation cho User Story 5

- [ ] T041 [US5] Hook + trang đăng nhập game — file: `…/platform/api/loginEvents.ts`, `…/platform/login-events/LoginEventsPage.tsx`

## Phase 8: User Story 6 — Feature flag (Priority: P3)

### Tests cho User Story 6

- [ ] T042 [P] [US6] Test F059-AC-20, F059-AC-21: danh sách, sửa mặc định + ghi đè tenant, `If-Match` + `reason`, `412` tải lại; không tạo/xóa flag — file: `…/platform/__tests__/FeatureFlagEditor.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 6

- [ ] T043 [US6] Hook + trang feature flag — file: `…/platform/api/featureFlags.ts`, `…/platform/feature-flags/FeatureFlagListPage.tsx`, `FeatureFlagEditor.tsx`

## Phase 9: User Story 7 — Tích hợp chỉ xem (Priority: P3)

### Tests cho User Story 7

- [ ] T044 [P] [US7] Test F059-AC-22: panel Keycloak/APISIX từ `getIntegrationOverview`, tab SIS `listSisConnections`/`listSisConnectionSchools`/`listSisSyncRuns` — file: `…/platform/__tests__/IntegrationsPage.test.tsx` — phải FAIL trước khi code
- [ ] T045 [P] [US7] Test F059-AC-23: không ô nhập, không render trường ngoài schema; E2E quét secret trong response/DOM — file: `src/cms/frontend/apps/portal/e2e/platform/integrations.spec.ts`

### Implementation cho User Story 7

- [ ] T046 [US7] Hook + trang Tích hợp — file: `…/platform/api/integrations.ts`, `…/platform/api/sisConnections.ts`, `…/platform/integrations/IntegrationsPage.tsx`, `KeycloakPanel.tsx`, `ApisixPanel.tsx`, `SisConnectionsTab.tsx`

## Phase 10: User Story 8 — Bảo vệ màn và đầu vào (Priority: P1)

### Tests cho User Story 8

- [ ] T047 [P] [US8] E2E F059-AC-24: `ad.truong.a`, `cm.noidung`, `sm.antoan` mở URL màn F059 → 403, không tải chunk D; menu ẩn theo permission (TC-CMS-AUTHZ-05, TC-CMS-WS-01, TC-CMS-WS-03) — file: `src/cms/frontend/apps/portal/e2e/platform/authz-f059.spec.ts` — phải FAIL trước khi code
- [ ] T048 [P] [US8] Test F059-AC-25: `MFA_REQUIRED` → step-up, lỗi mạng → banner VPN, `PERMISSION_DENIED` + `trace_id` (TC-CMS-WS-02) — file: `…/platform/__tests__/f059Errors.test.tsx`
- [ ] T049 [P] [US8] Test F059-AC-26: mọi POST ghi có `Idempotency-Key` giữ khi retry; PATCH/PUT/DELETE có `If-Match` — file: `…/platform/__tests__/f059Idempotency.test.ts`
- [ ] T050 [P] [US8] Test F059-AC-27: ô nhập theo `maxLength`/`pattern` + bộ đếm (TC-TEN-024) — file: `…/platform/__tests__/f059InputLimits.test.tsx`

### Implementation cho User Story 8

- [ ] T051 [US8] Gắn guard permission từng route, nối `MfaStepUp`/`VpnRequiredBanner` (F057) vào mọi page F059 — file: `…/platform/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T052 [P] Kiểm không import component/Theme game (TC-UI-010), lint hardcode style (TC-UI-003) — kiểm: `make web-lint`
- [ ] T053 [P] axe + visual regression màn F059 sáng/tối (TC-UI-004, TC-UI-006) — file: `src/cms/frontend/apps/portal/e2e/platform/a11y-visual-f059.spec.ts`
- [ ] T054 [P] Báo mâu thuẫn permission audit (`platform.audit.read` vs `platform.audit.read_all`) để F012 chốt; cập nhật ADMIN_PORTAL §2.2 khi chốt — file: `docs/04-admin-portal/ADMIN_PORTAL.md`
- [ ] T055 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md`
- [ ] T056 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [ ] T057 [P] Test F059-AC-28: tắt `menu.platform_admin.student_activity` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/platform-admin/users-audit-feature-toggle.spec.ts`
- [ ] T058 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F059-AC-28) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational → US8 + US1 (MVP) → US2 → US4 → US3 (cần T004, T006) → US5 → US6 (cần T007) → US7 (cần T008, T009) → Polish.
- F057 phải Done (chunk D, `TenantFilter`, `MfaStepUp`, `VpnRequiredBanner`); backend F012/F015 Done cho US1, US2, US4.

## Definition of Done

- [ ] Mọi tiêu chí F059-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật qua gateway nội bộ
- [ ] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
