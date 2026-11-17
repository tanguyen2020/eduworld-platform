# Tasks: F052 — Workspace C — quản trị trường: trường/lớp/học sinh, SIS, người dùng và phân quyền, cài đặt, audit

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Gốc mã: `web/apps/portal/src/workspaces/school-lms/` (viết tắt `WS/`). Dùng lại từ F051: `withTenantScope`, `requireSingleTenant`, `useIdempotentMutation`, `ReasonDialog`, `mapValidationErrors`, `LargeScreenRequired`, registry menu sinh từ `specs/portal/menu-catalog.yaml`, `<ActionGate>`. Mock MSW chỉ trong test (`// MOCK (test-only)`).

## Phase 1: Setup

- [ ] T001 Đăng ký route và mục menu workspace C (Trường, Lớp, Học sinh chưa có lớp, Đồng bộ SIS, Người dùng & phân quyền, Cài đặt, Học sinh sắp bị dọn, Nhật ký) theo khóa menu từ `specs/portal/menu-catalog.yaml` (registry sinh, F051; không khai báo menu tay), bảng khóa trong plan.md — file: `web/apps/portal/src/workspaces/school-lms/routes.tsx`
- [ ] T002 [P] Khóa i18n `portal.school_lms.*` vi/en (thông điệp lỗi theo `code`, nhãn `issue_type`, `end_reason`) — file: `web/apps/portal/src/i18n/locales/vi/portal.school_lms.json`, `web/apps/portal/src/i18n/locales/en/portal.school_lms.json`
- [ ] T003 [P] Playwright project `@F052`, CSV mẫu (một dòng trùng username tenant khác) — file: `web/apps/portal/tests/e2e/fixtures/sis/students_sample.csv`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Contract: thêm `listTenantClassStudents` + schema item (maxLength mọi string) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T005 Contract: thêm `listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue`, schema `ReconciliationIssue` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T006 Contract: thêm `getSchoolSettings`, `updateSchoolSettings`, schema `SchoolSettings` (`ETag`/`If-Match`) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T007 Contract: bỏ `class` khỏi `UserRoleScopeType` và `TenantUserCreateRequest.initial_user_roles[].scope_type` (ADR-0072) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-breaking` (ghi chú breaking có chủ đích)
- [ ] T008 Sinh lại kiểu client — file: `web/apps/portal/src/api/schema.d.ts` — kiểm: `pnpm gen:api`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Trường, lớp, học sinh và hoạt động đăng nhập (Priority: P1) 🎯 MVP

**Goal**: Xem/sửa/duyệt trường, lớp, học sinh chỉ đọc, học sinh chưa có lớp, hoạt động đăng nhập.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test `SchoolEditForm`: chỉ tỉnh `active`, trường SIS chỉ đọc, gửi `If-Match` (F052-AC-02) — file: `web/apps/portal/tests/unit/school-lms/schools/SchoolEditForm.test.tsx` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Unit test `ApproveSchoolDialog` khóa khi thiếu tỉnh (F052-AC-03) — file: `web/apps/portal/tests/unit/school-lms/schools/ApproveSchoolDialog.test.tsx`
- [ ] T011 [P] [US1] Unit test view model login event chỉ giữ field cho phép, tính số ngày chưa vào game (F052-AC-06) — file: `web/apps/portal/tests/unit/school-lms/students/loginEventView.test.ts`
- [ ] T012 [P] [US1] Unit test `ClassStudentsPage`, `UnassignedStudentsPage` không có thao tác sửa/gán (F052-AC-04, F052-AC-05) — file: `web/apps/portal/tests/unit/school-lms/classes/readOnly.test.tsx`
- [ ] T013 [US1] E2E trường/lớp/học sinh/hoạt động đăng nhập với `ad.donvi`, `ad.truong.a` (F052-AC-01, F052-AC-02, F052-AC-03, F052-AC-04, F052-AC-05, F052-AC-06; TC-ORG-102, TC-ORG-104, TC-SIS-103) — file: `web/apps/portal/tests/e2e/school-lms/schools.spec.ts`
- [ ] T014 [US1] E2E tenant isolation + authorization cho `getTenantSchool`, `listTenantSchoolClasses`, `listTenantClassStudents` (mới), `listTenantStudentLoginEvents`: `ad.truong.b` mở tài nguyên TH Demo A → "Không tìm thấy" (F052-AC-07; TC-TEN-001, TC-TEN-003) — file: `web/apps/portal/tests/e2e/school-lms/isolation.spec.ts`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Hook `useSchools` (`listTenantSchools`, `getTenantSchool`, `updateTenantSchool`, `approveTenantSchool`, `listProvinces`) — file: `web/apps/portal/src/workspaces/school-lms/schools/useSchools.ts`
- [ ] T016 [US1] `SchoolListPage`, `SchoolDetailPage`, `SchoolEditForm`, `ApproveSchoolDialog` — file: `web/apps/portal/src/workspaces/school-lms/schools/SchoolDetailPage.tsx`
- [ ] T017 [US1] `ClassListPage`, `ClassStudentsPage`, `useClasses` (`listTenantSchoolClasses`, `listTenantClassStudents`) — file: `web/apps/portal/src/workspaces/school-lms/classes/ClassStudentsPage.tsx`
- [ ] T018 [P] [US1] `UnassignedStudentsPage` (`listUnassignedStudents`) — file: `web/apps/portal/src/workspaces/school-lms/students/UnassignedStudentsPage.tsx`
- [ ] T019 [P] [US1] `StudentLoginActivityPanel`, `useLoginEvents` (`listTenantStudentLoginEvents`, view model whitelist) — file: `web/apps/portal/src/workspaces/school-lms/students/StudentLoginActivityPanel.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Nhập CSV SIS và xử lý xung đột đồng bộ (Priority: P1)

**Goal**: Import CSV có tiến trình, xử lý xung đột, idempotent.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T020 [P] [US2] Unit test kiểm file (`.csv`, `csv_import_max`) và `sha256` (F052-AC-08) — file: `web/apps/portal/tests/unit/school-lms/sis/fileCheck.test.ts` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Unit test luồng `createSisImport` → PUT `upload_url` → poll `getSisImport`; `expired` tạo key mới; retry giữ key (F052-AC-08, F052-AC-09, F052-AC-11) — file: `web/apps/portal/tests/unit/school-lms/sis/useSisImport.test.ts`
- [ ] T022 [P] [US2] Unit test `ResolveIssueDialog`: lý do bắt buộc (độ dài theo OpenAPI), dùng lại Idempotency-Key khi retry (F052-AC-10, F052-AC-11) — file: `web/apps/portal/tests/unit/school-lms/sis/ResolveIssueDialog.test.tsx`
- [ ] T023 [US2] E2E import CSV + xung đột với `ad.truong.a`; authorization: `qlcm.truong.a` không có nút Tải/Xử lý; tenant isolation: `ad.truong.b` gọi `listSchoolReconciliationIssues`/`resolveSchoolReconciliationIssue` của TH Demo A → 404 (F052-AC-08, F052-AC-09, F052-AC-10, F052-AC-11; TC-AUTH-074) — file: `web/apps/portal/tests/e2e/school-lms/sis.spec.ts`

### Implementation cho User Story 2

- [ ] T024 [P] [US2] `sha256.ts` (Web Crypto, đọc theo khối) và `useSisImport` — file: `web/apps/portal/src/workspaces/school-lms/sis/useSisImport.ts`
- [ ] T025 [US2] `SisImportPage`, `SisImportProgress` (hiển thị lịch `sis_sync_schedule`, cột mẫu theo SIS Contract) — file: `web/apps/portal/src/workspaces/school-lms/sis/SisImportPage.tsx`
- [ ] T026 [US2] `ReconciliationIssuesPage`, `ResolveIssueDialog` (`listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue`; tab `missing_class` dùng `listUnassignedStudents`) — file: `web/apps/portal/src/workspaces/school-lms/sis/ReconciliationIssuesPage.tsx`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Người dùng, Role, UserRole và nhật ký phân quyền (Priority: P2)

**Goal**: Quản lý nhân sự, role, UserRole trong tenant/trường.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T027 [P] [US3] Unit test `CreateStaffForm`: trường bắt buộc, pattern/maxLength username, map `409 USERNAME_TAKEN`/`EMAIL_TAKEN` + gợi ý (F052-AC-13) và học sinh không có nút gán role (F052-AC-12) — file: `web/apps/portal/tests/unit/school-lms/users/CreateStaffForm.test.tsx` — phải FAIL trước khi code
- [ ] T028 [P] [US3] Unit test `PermissionPicker` chỉ permission từ `listTenantPermissions`, map sub-code `PERMISSION_NOT_ALLOWED_FOR_SCOPE` (F052-AC-14) — file: `web/apps/portal/tests/unit/school-lms/roles/PermissionPicker.test.tsx`
- [ ] T029 [P] [US3] Unit test `RoleEditor`: `is_system` chỉ đọc + nhân bản; lưu cần lý do + `If-Match`; xóa 409 → gợi ý (F052-AC-15) — file: `web/apps/portal/tests/unit/school-lms/roles/RoleEditor.test.tsx`
- [ ] T030 [P] [US3] Unit test `GrantUserRoleDialog`: phạm vi Đơn vị/Trường/Môn (không Lớp), lọc role theo `canGrant`, School Admin gọi `grantSchoolUserRole`, map `PERMISSION_DENIED`/`ROLE_NOT_FOUND`; thu hồi cần lý do + thông báo hiệu lực (F052-AC-16, F052-AC-17) — file: `web/apps/portal/tests/unit/school-lms/users/GrantUserRoleDialog.test.tsx`
- [ ] T031 [US3] E2E người dùng/role/UserRole/nhật ký phân quyền với `ad.donvi`, `ad.truong.a`, `qlcm.truong.a` (F052-AC-12, F052-AC-13, F052-AC-14, F052-AC-15, F052-AC-16, F052-AC-17, F052-AC-18; TC-CMS-RBAC-01, TC-CMS-RBAC-02, TC-CMS-RBAC-03, TC-CMS-RBAC-05, TC-CMS-RBAC-06, TC-AUTH-073, TC-AUTH-076, TC-AUTH-077, TC-AUTH-078, TC-CMS-LOGIN-04) — file: `web/apps/portal/tests/e2e/school-lms/users-roles.spec.ts`

### Implementation cho User Story 3

- [ ] T032 [P] [US3] `UserListPage`, `useUsers` (`listTenantUsers`, `updateTenantUser`) — file: `web/apps/portal/src/workspaces/school-lms/users/UserListPage.tsx`
- [ ] T033 [US3] `CreateStaffForm` (`createTenantUser`) — file: `web/apps/portal/src/workspaces/school-lms/users/CreateStaffForm.tsx`
- [ ] T034 [US3] `RoleListPage`, `RoleEditor`, `PermissionPicker`, `useRoles` — file: `web/apps/portal/src/workspaces/school-lms/roles/RoleEditor.tsx`
- [ ] T035 [US3] `UserRolesPanel`, `GrantUserRoleDialog`, `canGrant.ts`, `useUserRoles` — file: `web/apps/portal/src/workspaces/school-lms/users/GrantUserRoleDialog.tsx`
- [ ] T036 [P] [US3] `PermissionAuditPage` (`listTenantAuditLogs` lọc `role.*`, `user_role.*`) — file: `web/apps/portal/src/workspaces/school-lms/audit/PermissionAuditPage.tsx`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Cài đặt tenant/trường và dọn học sinh không đăng nhập (Priority: P2)

**Goal**: Cài đặt tenant/trường, danh sách sắp bị dọn, loại trừ/gia hạn.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T037 [P] [US4] Unit test `TenantSettingsPage`: Merge Patch + `If-Match`, hiện `effective_cleanup_days`, cảnh báo ngưỡng, map `PERMISSION_DENIED`/`RETENTION_BELOW_MINIMUM` tại trường (F052-AC-19, F052-AC-20) — file: `web/apps/portal/tests/unit/school-lms/settings/TenantSettingsPage.test.tsx` — phải FAIL trước khi code
- [ ] T038 [P] [US4] Unit test `ExcludeDialog`: tối đa `maxItems`, lý do bắt buộc, Idempotency-Key (F052-AC-21) — file: `web/apps/portal/tests/unit/school-lms/student-cleanup/ExcludeDialog.test.tsx`
- [ ] T039 [P] [US4] Unit test `SchoolSettingsPage` (`updateSchoolSettings` + `If-Match`) (F052-AC-22) — file: `web/apps/portal/tests/unit/school-lms/settings/SchoolSettingsPage.test.tsx`
- [ ] T040 [US4] E2E cài đặt + dọn học sinh; authorization: `qlcm.truong.a` không có màn cài đặt; tenant isolation: `ad.truong.b` gọi `getSchoolSettings`/`updateSchoolSettings` (mới) của TH Demo A → 404 (F052-AC-19, F052-AC-20, F052-AC-21, F052-AC-22) — file: `web/apps/portal/tests/e2e/school-lms/settings.spec.ts`

### Implementation cho User Story 4

- [ ] T041 [US4] `TenantSettingsPage`, `useTenantSettings` — file: `web/apps/portal/src/workspaces/school-lms/settings/TenantSettingsPage.tsx`
- [ ] T042 [P] [US4] `SchoolSettingsPage`, `useSchoolSettings` — file: `web/apps/portal/src/workspaces/school-lms/settings/SchoolSettingsPage.tsx`
- [ ] T043 [US4] `CleanupPreviewPage`, `ExcludeDialog`, `useCleanup` (`previewTenantStudentCleanup`, `createStudentCleanupExclusions`, `deleteStudentCleanupExclusions`) — file: `web/apps/portal/src/workspaces/school-lms/student-cleanup/CleanupPreviewPage.tsx`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Nhật ký audit của tenant (Priority: P3)

**Goal**: Tra cứu audit tenant.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T044 [P] [US5] Unit test `AuditLogPage`: bộ lọc, hiển thị `changes`, `trace_id` (F052-AC-23) — file: `web/apps/portal/tests/unit/school-lms/audit/AuditLogPage.test.tsx` — phải FAIL trước khi code
- [ ] T045 [US5] E2E audit `ad.donvi`; authorization: `qlcm.truong.a` không có menu Nhật ký (F052-AC-23) — file: `web/apps/portal/tests/e2e/school-lms/audit.spec.ts`

### Implementation cho User Story 5

- [ ] T046 [US5] `AuditLogPage`, `useAuditLogs` (`listTenantAuditLogs`) — file: `web/apps/portal/src/workspaces/school-lms/audit/AuditLogPage.tsx`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase 8: Polish & Cross-Cutting

- [ ] T047 E2E yêu cầu chung: `sa.admin` phải chọn một tenant (F052-AC-24; TC-TEN-004); ma trận ẩn hành động ghi theo permission cho `ad.donvi`/`ad.truong.a`/`qlcm.truong.a` + xử lý 403 (F052-AC-25; TC-TEN-003) — file: `web/apps/portal/tests/e2e/school-lms/cross.spec.ts`
- [ ] T048 [P] E2E giới hạn `maxLength`/`pattern` và map `VALIDATION_FAILED` (F052-AC-26; TC-TEN-024); < 768 px dạng thẻ + "cần màn hình lớn hơn" + axe (F052-AC-27; TC-UI-006) — file: `web/apps/portal/tests/e2e/school-lms/responsive-a11y.spec.ts`
- [ ] T049 [P] Story Storybook cho dialog/form chính (sáng/tối) — file: `web/apps/portal/src/workspaces/school-lms/school-lms.stories.tsx`
- [ ] T050 [P] Cập nhật SIS_INTEGRATION §10 theo route public `/schools/{school_id}/sis-imports`, `/schools/{school_id}/reconciliation-issues`; cập nhật API_DESIGN, CHANGELOG — file: `docs/09-identity/SIS_INTEGRATION.md`, `CHANGELOG.md`
- [ ] T051 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/052-school-admin-workspace/quickstart.md`
- [ ] T052 Cập nhật trạng thái F052 — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T053 [P] Test F052-AC-28: tắt `menu.school_lms.audit_logs` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `web/apps/portal/tests/e2e/school-lms/feature-toggle.spec.ts`
- [ ] T054 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F052-AC-28) — file: `web/apps/portal/src/workspaces/school-lms/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004–T008; backend owner F010/F011/F015 hiện thực endpoint mới) → US1, US2 (song song) → US3, US4 (song song) → US5 → Polish.
- T016 cần T015; T026 cần T005 + backend F011; T042 cần T006 + backend F015; T035 cần T007.

## Definition of Done

- [ ] Mọi tiêu chí F052-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright `@F052` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
