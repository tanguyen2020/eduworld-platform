# Tasks: F052 — Workspace C — quản trị trường: trường/lớp/học sinh, SIS, người dùng và phân quyền, cài đặt, audit

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Gốc mã: `src/cms/frontend/apps/portal/src/workspaces/school-lms/` (viết tắt `WS/`). Dùng lại từ F051: `withTenantScope`, `requireSingleTenant`, `useIdempotentMutation`, `ReasonDialog`, `mapValidationErrors`, `LargeScreenRequired`, registry menu sinh từ `specs/portal/menu-catalog.yaml`, `<ActionGate>`. Mock MSW chỉ trong test (`// MOCK (test-only)`).

## Phase 1: Setup

- [x] T001 Đăng ký route và mục menu workspace C (Trường, Lớp, Học sinh chưa có lớp, Đồng bộ SIS, Người dùng & phân quyền, Cài đặt, Học sinh sắp bị dọn, Nhật ký) theo khóa menu từ `specs/portal/menu-catalog.yaml` (registry sinh, F051; không khai báo menu tay), bảng khóa trong plan.md — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx` — **2026-11-20:** `school-lms/index.tsx` gắn 7 màn vào khóa menu catalog (sub-route trong màn, không khai báo menu tay); `catalogMenus.test.tsx` xanh.
- [x] T002 [P] Khóa i18n `portal.school_lms.*` vi/en (thông điệp lỗi theo `code`, nhãn `issue_type`, `end_reason`) — file: `src/cms/frontend/apps/portal/src/i18n/locales/vi/portal.school_lms.json`, `src/cms/frontend/apps/portal/src/i18n/locales/en/portal.school_lms.json` — **2026-11-20:** `src/i18n/locales/{vi,en}/portal.school_lms.json` (khóa `portal.school_lms.*`); `school-lms/i18n.test.ts` (vi = en, không rỗng, mọi khóa dùng trong code tồn tại) pass.
- [ ] T003 [P] Playwright project `@F052`, CSV mẫu (một dòng trùng username tenant khác) — file: `src/cms/frontend/apps/portal/tests/e2e/fixtures/sis/students_sample.csv` — **2026-11-20:** CSV mẫu `tests/e2e/fixtures/sis/students_sample.csv` đã có; project Playwright `@F052` chờ backend thật (OQ F051-9).

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Contract: thêm `listTenantClassStudents` + schema item (maxLength mọi string) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** `ClassStudent`/`ClassStudentPage`, `x-portal-action` + catalog; `check-maxlength` 0 vi phạm, `check-portal-catalog` 0 lỗi, `split-public --check`, `routematrix check` (533 operation, 0 lỗi).
- [x] T005 Contract: thêm `listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue`, schema `ReconciliationIssue` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** dùng lại `SisReconciliationIssue`(Page) + `SchoolReconciliationResolve` (Clarifications 2026-11-19); kiểm như T004.
- [x] T006 Contract: thêm `getSchoolSettings`, `updateSchoolSettings`, schema `SchoolSettings` (`ETag`/`If-Match`, `curfew_window`, `platform_curfew`, `effective_curfew`), `CurfewWindow`; action `action.school_lms.settings.edit_school` trong `specs/portal/menu-catalog.yaml` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/portal/menu-catalog.yaml` — kiểm: `node scripts/specs/check-maxlength.mjs`, `node scripts/specs/check-portal-catalog.mjs`
- [x] T007 Contract: bỏ `class` khỏi `UserRoleScopeType` và `TenantUserCreateRequest.initial_user_roles[].scope_type` (ADR-0072) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-breaking` (ghi chú breaking có chủ đích) — **2026-11-20:** bỏ `class` ở `TenantUserCreateRequest.initial_user_roles[].scope_type` và query `listTenantUserRoles.scope_type` (`UserRoleScopeType` đã không có). Breaking có chủ đích (backend đã từ chối `class`, ADR-0072).
- [x] T008 Sinh lại kiểu client — file: `src/cms/frontend/apps/portal/src/api/schema.d.ts` — kiểm: `pnpm gen:api` — **2026-11-20:** `pnpm gen:api` → `schema.gen.d.ts`, `operation-meta.gen.ts`, `menu-catalog.ts`; `pnpm gen:check` xanh.

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Trường, lớp, học sinh và hoạt động đăng nhập (Priority: P1) 🎯 MVP

**Goal**: Xem/sửa/duyệt trường, lớp, học sinh chỉ đọc, học sinh chưa có lớp, hoạt động đăng nhập.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit test `SchoolEditForm`: chỉ tỉnh `active`, trường SIS chỉ đọc, gửi `If-Match` (F052-AC-02) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/schools/SchoolEditForm.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `school-lms/schools/schools.test.tsx` (pure `readOnlyFields`/`updateBody` + màn sửa). Test đặt cạnh mã (Vitest chỉ chạy `src/**`).
- [x] T010 [P] [US1] Unit test `ApproveSchoolDialog` khóa khi thiếu tỉnh (F052-AC-03) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/schools/ApproveSchoolDialog.test.tsx` — **2026-11-20:** `schools/schools.test.tsx` › approve is locked…
- [x] T011 [P] [US1] Unit test view model login event chỉ giữ field cho phép, tính số ngày chưa vào game (F052-AC-06) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/students/loginEventView.test.ts` — **2026-11-20:** `students/students.test.tsx` (view model whitelist, số ngày, không IP trên màn).
- [x] T012 [P] [US1] Unit test `ClassStudentsPage`, `UnassignedStudentsPage` không có thao tác sửa/gán (F052-AC-04, F052-AC-05) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/classes/readOnly.test.tsx` — **2026-11-20:** `classes/classes.test.tsx` (chỉ đọc, nhãn SIS, không nút gán lớp; học sinh chưa có lớp).
- [x] T013 [US1] E2E trường/lớp/học sinh/hoạt động đăng nhập với `ad.donvi`, `ad.truong.a` (F052-AC-01, F052-AC-02, F052-AC-03, F052-AC-04, F052-AC-05, F052-AC-06; TC-ORG-102, TC-ORG-104, TC-SIS-103) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/schools.spec.ts` — **2026-11-20:** phần tích hợp toàn portal với MOCK backend: `schools.test.tsx`, `classes.test.tsx`, `students.test.tsx`; E2E Playwright backend thật chờ OQ F051-9 + backend `listTenantClassStudents` (Câu hỏi mở F052-1).
- [x] T014 [US1] E2E tenant isolation + authorization cho `getTenantSchool`, `listTenantSchoolClasses`, `listTenantClassStudents` (mới), `listTenantStudentLoginEvents`: `ad.truong.b` mở tài nguyên TH Demo A → "Không tìm thấy" (F052-AC-07; TC-TEN-001, TC-TEN-003) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/isolation.spec.ts` — **2026-11-20:** `ad.truong.b` mở trường/lớp/học sinh TH Demo A → "Không tìm thấy" (schools/classes/students test, MOCK cô lập theo `X-Tenant-Id`); test backend của operation mới thuộc owner F010/F011.
- [x] T058 [P] [US1] Test F052-AC-31: dialog đặt lại mật khẩu (bắt buộc `reason`, `Idempotency-Key` dùng lại khi thử lại, mật khẩu hiện một lần + in thẻ, `password_redacted` → thông báo, ẩn nút khi thiếu `lms.student.password_reset`/action tắt; không ghi mật khẩu vào storage/log) — file: `src/cms/frontend/apps/portal/src/workspaces/school/classes/__tests__/ResetStudentPasswordDialog.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `classes/classes.test.tsx` › Đặt lại mật khẩu học sinh (3 test; file đặt ở `school-lms/` thay cho `school/`).

### Implementation cho User Story 1

- [x] T015 [P] [US1] Hook `useSchools` (`listTenantSchools`, `getTenantSchool`, `updateTenantSchool`, `approveTenantSchool`, `listProvinces`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/schools/useSchools.ts` — **2026-11-20:** `schools/useSchools.ts`.
- [x] T016 [US1] `SchoolListPage`, `SchoolDetailPage`, `SchoolEditForm`, `ApproveSchoolDialog` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/schools/SchoolDetailPage.tsx` — **2026-11-20:** `schools/SchoolListPage.tsx`, `SchoolDetailPage.tsx`, `SchoolEditForm.tsx`, `ApproveSchoolDialog.tsx`, `schoolForm.ts`, `SchoolsScreen.tsx`.
- [x] T017 [US1] `ClassListPage`, `ClassStudentsPage`, `useClasses` (`listTenantSchoolClasses`, `listTenantClassStudents`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/classes/ClassStudentsPage.tsx` — **2026-11-20:** `classes/ClassListPage.tsx`, `ClassStudentsPage.tsx`, `useClasses.ts`, `ClassesScreen.tsx`.
- [x] T059 [US1] `ResetStudentPasswordDialog` trên `ClassStudentsPage` gọi `resetStudentPassword` (`x-portal-action: action.school_lms.classes.reset_student_password`) — file: `src/cms/frontend/apps/portal/src/workspaces/school/classes/ResetStudentPasswordDialog.tsx` — **2026-11-20:** `classes/ResetStudentPasswordDialog.tsx` (mật khẩu chỉ trong state, in thẻ).
- [x] T018 [P] [US1] `UnassignedStudentsPage` (`listUnassignedStudents`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/students/UnassignedStudentsPage.tsx` — **2026-11-20:** `students/UnassignedStudentsPage.tsx`.
- [x] T019 [P] [US1] `StudentLoginActivityPanel`, `useLoginEvents` (`listTenantStudentLoginEvents`, view model whitelist) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/students/StudentLoginActivityPanel.tsx` — **2026-11-20:** `students/StudentLoginActivityPanel.tsx`, `useLoginEvents.ts`, `loginEventView.ts`, `StudentActivityScreen.tsx`.

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Nhập CSV SIS và xử lý xung đột đồng bộ (Priority: P1)

**Goal**: Import CSV có tiến trình, xử lý xung đột, idempotent.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T020 [P] [US2] Unit test kiểm file (`.csv`, `csv_import_max`) và `sha256` (F052-AC-08) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/sis/fileCheck.test.ts` — phải FAIL trước khi code — **2026-11-20:** `sis/sisFlow.test.ts` › file checks.
- [x] T021 [P] [US2] Unit test luồng `createSisImport` → PUT `upload_url` → poll `getSisImport`; `expired` tạo key mới; retry giữ key (F052-AC-08, F052-AC-09, F052-AC-11) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/sis/useSisImport.test.ts` — **2026-11-20:** `sis/sisFlow.test.ts` (`runImport`, PUT không Authorization, hết hạn, back-off) + `sis/sis.test.tsx` (một import cho hai lần bấm).
- [x] T022 [P] [US2] Unit test `ResolveIssueDialog`: lý do bắt buộc (độ dài theo OpenAPI), dùng lại Idempotency-Key khi retry (F052-AC-10, F052-AC-11) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/sis/ResolveIssueDialog.test.tsx` — **2026-11-20:** `sis/sis.test.tsx` › Xung đột đồng bộ (lý do bắt buộc, cùng Idempotency-Key khi thử lại).
- [x] T023 [US2] E2E import CSV + xung đột với `ad.truong.a`; authorization: `qlcm.truong.a` không có nút Tải/Xử lý; tenant isolation: `ad.truong.b` gọi `listSchoolReconciliationIssues`/`resolveSchoolReconciliationIssue` của TH Demo A → 404 (F052-AC-08, F052-AC-09, F052-AC-10, F052-AC-11; TC-AUTH-074) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/sis.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `sis/sis.test.tsx` (gồm quyền chỉ đọc); E2E + isolation backend chờ backend F011 cho route public mới (Câu hỏi mở F052-1).

### Implementation cho User Story 2

- [x] T024 [P] [US2] `sha256.ts` (Web Crypto, đọc theo khối) và `useSisImport` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/sis/useSisImport.ts` — **2026-11-20:** `sis/fileCheck.ts` (sha256 Web Crypto, đọc một lần, ≤ 10 MB — Web Crypto không băm theo khối) + `sis/useSisImport.ts`.
- [x] T025 [US2] `SisImportPage`, `SisImportProgress` (hiển thị lịch `sis_sync_schedule`, cột mẫu theo SIS Contract) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/sis/SisImportPage.tsx` — **2026-11-20:** `sis/SisImportPage.tsx`, `SisImportProgress.tsx`, `SisScreen.tsx`.
- [x] T026 [US2] `ReconciliationIssuesPage`, `ResolveIssueDialog` (`listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue`; tab `missing_class` dùng `listUnassignedStudents`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/sis/ReconciliationIssuesPage.tsx` — **2026-11-20:** `sis/ReconciliationIssuesPage.tsx`, `ResolveIssueDialog.tsx`, `useReconciliation.ts`.

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Người dùng, Role, UserRole và nhật ký phân quyền (Priority: P2)

**Goal**: Quản lý nhân sự, role, UserRole trong tenant/trường.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T027 [P] [US3] Unit test `CreateStaffForm`: trường bắt buộc, pattern/maxLength username, map `409 USERNAME_TAKEN`/`EMAIL_TAKEN` + gợi ý (F052-AC-13) và học sinh không có nút gán role (F052-AC-12) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/users/CreateStaffForm.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `users/access.test.tsx` › create staff, students read-only.
- [x] T028 [P] [US3] Unit test `PermissionPicker` chỉ permission từ `listTenantPermissions`, map sub-code `PERMISSION_NOT_ALLOWED_FOR_SCOPE` (F052-AC-14) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/roles/PermissionPicker.test.tsx` — **2026-11-20:** `users/access.test.tsx` › picker groups / refused permission.
- [x] T029 [P] [US3] Unit test `RoleEditor`: `is_system` chỉ đọc + nhân bản; lưu cần lý do + `If-Match`; xóa 409 → gợi ý (F052-AC-15) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/roles/RoleEditor.test.tsx` — **2026-11-20:** `users/access.test.tsx` › system role read-only + clone, delete 409.
- [x] T030 [P] [US3] Unit test `GrantUserRoleDialog`: phạm vi Đơn vị/Trường/Môn (không Lớp), lọc role theo `canGrant`, School Admin gọi `grantSchoolUserRole`, map `PERMISSION_DENIED`/`ROLE_NOT_FOUND`; thu hồi cần lý do + thông báo hiệu lực (F052-AC-16, F052-AC-17) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/users/GrantUserRoleDialog.test.tsx` — **2026-11-20:** `users/access.test.tsx` › grant/revoke + pure `canGrant`.
- [x] T031 [US3] E2E người dùng/role/UserRole/nhật ký phân quyền với `ad.donvi`, `ad.truong.a`, `qlcm.truong.a` (F052-AC-12, F052-AC-13, F052-AC-14, F052-AC-15, F052-AC-16, F052-AC-17, F052-AC-18; TC-CMS-RBAC-01, TC-CMS-RBAC-02, TC-CMS-RBAC-03, TC-CMS-RBAC-05, TC-CMS-RBAC-06, TC-AUTH-073, TC-AUTH-076, TC-AUTH-077, TC-AUTH-078, TC-CMS-LOGIN-04) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/users-roles.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `users/access.test.tsx`; E2E backend thật chờ OQ F051-9.

### Implementation cho User Story 3

- [x] T032 [P] [US3] `UserListPage`, `useUsers` (`listTenantUsers`, `updateTenantUser`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/users/UserListPage.tsx` — **2026-11-20:** `users/UserListPage.tsx`, `useUsers.ts`, `AccessControlScreen.tsx`.
- [x] T033 [US3] `CreateStaffForm` (`createTenantUser`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/users/CreateStaffForm.tsx` — **2026-11-20:** `users/CreateStaffForm.tsx`, `ScopeTargetField.tsx`.
- [x] T034 [US3] `RoleListPage`, `RoleEditor`, `PermissionPicker`, `useRoles` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/roles/RoleEditor.tsx` — **2026-11-20:** `roles/RoleListPage.tsx`, `RoleEditor.tsx`, `PermissionPicker.tsx`, `useRoles.ts`.
- [x] T035 [US3] `UserRolesPanel`, `GrantUserRoleDialog`, `canGrant.ts`, `useUserRoles` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/users/GrantUserRoleDialog.tsx` — **2026-11-20:** `users/UserRolesPanel.tsx`, `GrantUserRoleDialog.tsx`, `canGrant.ts`, `useUserRoles.ts`.
- [x] T036 [P] [US3] `PermissionAuditPage` (`listTenantAuditLogs` lọc `role.*`, `user_role.*`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/audit/PermissionAuditPage.tsx` — **2026-11-20:** `audit/PermissionAuditPage.tsx` (một action/lần — OQ F052-2 đã chốt 2026-11-20, task bổ sung dùng `action_prefix`).

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Cài đặt tenant/trường và dọn học sinh không đăng nhập (Priority: P2)

**Goal**: Cài đặt tenant/trường, danh sách sắp bị dọn, loại trừ/gia hạn.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T037 [P] [US4] Unit test `TenantSettingsPage`: Merge Patch + `If-Match`, hiện `effective_cleanup_days`, cảnh báo ngưỡng, map `PERMISSION_DENIED`/`RETENTION_BELOW_MINIMUM` tại trường (F052-AC-19, F052-AC-20) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/settings/TenantSettingsPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `settings/settings.test.tsx` › Cài đặt đơn vị (2 test) + pure `mergePatch`.
- [x] T038 [P] [US4] Unit test `ExcludeDialog`: tối đa `maxItems`, lý do bắt buộc, Idempotency-Key (F052-AC-21) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/student-cleanup/ExcludeDialog.test.tsx` — **2026-11-20:** `settings/settings.test.tsx` › Học sinh sắp bị dọn + `MAX_EXCLUDE`.
- [x] T055 [P] [US4] Unit test `CurfewSettingsCard`: hiển thị `platform_curfew` chỉ đọc, `curfew_window`, `effective_curfew`, khung qua nửa đêm; ẩn ô sửa khi thiếu `action.school_lms.settings.edit_school` hoặc tenant `common`; không có lựa chọn theo khối (F052-AC-29) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/settings/CurfewSettingsCard.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `settings/settings.test.tsx` › platform curfew read-only…
- [x] T056 [P] [US4] Unit test lưu khung giới nghiêm: khung không chứa trọn `platform_curfew` (vd. 22:00–06:30, 21:00–06:00) → lỗi tại ô, khóa Lưu; khung 21:00–07:00 → Merge Patch `curfew_window` + `If-Match` + `reason`; "Bỏ khung trường" → `null`; mock `422 VALIDATION_FAILED` `errors[curfew_window.start]` và `412` → lỗi tại ô, giữ giá trị (F052-AC-30) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/settings/CurfewSettingsForm.test.tsx` — **2026-11-20:** `settings/settings.test.tsx` › narrower window…, 422 tại ô + pure `curfewCovers`.
- [x] T039 [P] [US4] Unit test `SchoolSettingsPage` (`updateSchoolSettings` + `If-Match`) (F052-AC-22) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/settings/SchoolSettingsPage.test.tsx` — **2026-11-20:** `settings/settings.test.tsx` › school policy saved with If-Match.
- [x] T040 [US4] E2E cài đặt + dọn học sinh + khung giới nghiêm trường (`ad.truong.a` mở rộng 21:00–07:00 thành công, thu hẹp bị `422` tại ô — F052-AC-29, F052-AC-30); authorization: `qlcm.truong.a` không có màn cài đặt; tenant isolation: `ad.truong.b` gọi `getSchoolSettings`/`updateSchoolSettings` (mới) của TH Demo A → 404 (F052-AC-19, F052-AC-20, F052-AC-21, F052-AC-22) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/settings.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `settings/settings.test.tsx`; E2E backend thật chờ OQ F051-9.

### Implementation cho User Story 4

- [x] T041 [US4] `TenantSettingsPage`, `useTenantSettings` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/settings/TenantSettingsPage.tsx` — **2026-11-20:** `settings/TenantSettingsPage.tsx`, `tenantSettingsForm.ts`, `useSettings.ts`, `AllowedHoursField.tsx`.
- [x] T042 [P] [US4] `SchoolSettingsPage`, `useSchoolSettings` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/settings/SchoolSettingsPage.tsx` — **2026-11-20:** `settings/SchoolSettingsPage.tsx`, `SettingsScreen.tsx`.
- [x] T057 [US4] `CurfewSettingsCard`, `CurfewSettingsForm` (kiểm chứa trọn khung platform qua nửa đêm bằng hàm thuần `curfewCovers`, không tự tính giờ hiệu lực — lấy `effective_curfew` từ server; nhãn qua file dịch, style theo token) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/settings/CurfewSettingsCard.tsx`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/settings/curfewCovers.ts` — **2026-11-20:** `settings/CurfewSettingsCard.tsx`, `curfewCovers.ts`.
- [x] T043 [US4] `CleanupPreviewPage`, `ExcludeDialog`, `useCleanup` (`previewTenantStudentCleanup`, `createStudentCleanupExclusions`, `deleteStudentCleanupExclusions`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/student-cleanup/CleanupPreviewPage.tsx` — **2026-11-20:** `student-cleanup/CleanupPreviewPage.tsx`, `ExcludeDialog.tsx`, `useCleanup.ts`.

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Nhật ký audit của tenant (Priority: P3)

**Goal**: Tra cứu audit tenant.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T044 [P] [US5] Unit test `AuditLogPage`: bộ lọc, hiển thị `changes`, `trace_id` (F052-AC-23) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/audit/AuditLogPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `audit/audit.test.tsx` › Nhật ký.
- [x] T045 [US5] E2E audit `ad.donvi`; authorization: `qlcm.truong.a` không có menu Nhật ký (F052-AC-23) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/audit.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `audit/audit.test.tsx` (không menu khi thiếu `tenant.audit.read`); E2E chờ OQ F051-9.

### Implementation cho User Story 5

- [x] T046 [US5] `AuditLogPage`, `useAuditLogs` (`listTenantAuditLogs`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/audit/AuditLogPage.tsx` — **2026-11-20:** `audit/AuditLogPage.tsx`, `AuditTable.tsx`, `useAuditLogs.ts`, `AuditScreen.tsx`.

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase 8: Polish & Cross-Cutting

- [x] T047 E2E yêu cầu chung: `sa.admin` phải chọn một tenant (F052-AC-24; TC-TEN-004); ma trận ẩn hành động ghi theo permission cho `ad.donvi`/`ad.truong.a`/`qlcm.truong.a` + xử lý 403 (F052-AC-25; TC-TEN-003) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/cross.spec.ts` — **2026-11-20:** `schools.test.tsx` › System Admin must pick exactly one tenant; ẩn nút ghi theo quyền trong classes/sis/users/settings test; E2E chờ OQ F051-9.
- [x] T048 [P] E2E giới hạn `maxLength`/`pattern` và map `VALIDATION_FAILED` (F052-AC-26; TC-TEN-024); < 768 px dạng thẻ + "cần màn hình lớn hơn" + axe (F052-AC-27; TC-UI-006) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/responsive-a11y.spec.ts` — **2026-11-20:** `common/fieldSchemas.test.ts` (22 giới hạn = OpenAPI), `access.test.tsx` (maxlength, pattern), thẻ < 768 px (`classes.test.tsx`), "cần màn hình lớn hơn" (`sis.test.tsx`); axe toàn trang chờ E2E.
- [x] T049 [P] Story Storybook cho dialog/form chính (sáng/tối) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-lms.stories.tsx` — **2026-11-20:** `school-lms.stories.tsx` (Light/Dark/Compact: kết quả import, picker permission, bảng đăng nhập).
- [x] T050 [P] Cập nhật SIS_INTEGRATION §10 theo route public `/schools/{school_id}/sis-imports`, `/schools/{school_id}/reconciliation-issues`; cập nhật API_DESIGN, CHANGELOG — file: `docs/09-identity/SIS_INTEGRATION.md`, `CHANGELOG.md` — **2026-11-20:** SIS_INTEGRATION §9–§10, API_DESIGN (bảng endpoint), CHANGELOG.
- [x] T051 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/052-school-admin-workspace/quickstart.md` — **2026-11-20:** quickstart §5 (lệnh tự động + bảng kịch bản ↔ test; kịch bản backend thật chờ OQ F051-9).
- [x] T052 Cập nhật trạng thái F052 — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — **2026-11-20:** F052 Done.
- [x] T053 [P] Test F052-AC-28: tắt `menu.school_lms.audit_logs` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/feature-toggle.spec.ts` — **2026-11-20:** `audit/audit.test.tsx` › menu/action turned off (3 test).
- [x] T054 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F052-AC-28) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx` — **2026-11-20:** menu/route từ catalog; nút bọc `<ActionGate>`; `catalogMenus.test.tsx` (không path/label tay) xanh.

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T060 **OQ chốt 2026-11-20** (OQ F052-2) `PermissionAuditPage` dùng `action_prefix=role&action_prefix=user_role`, ô Hành động tùy chọn (AND); cập nhật test — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/audit/PermissionAuditPage.tsx` — kiểm: `make web-test` — cỡ S — ✅ 2026-11-20: `useAuditLogs` nhận `actionPrefixes` → `action_prefix[]` (lặp tham số, OR giữa nhóm); `PermissionAuditPage` gửi một truy vấn `action_prefix=role&action_prefix=user_role`, ô "Hành động" thành Select tùy chọn (mặc định "Mọi thay đổi role và phân quyền"; chọn một action → thêm `action`, AND ở server); MOCK backend lọc theo `action_prefix`/`action`; test `access.test.tsx` "permission log: one query action_prefix…" PASS
- [x] T061 **OQ chốt 2026-11-20** (OQ F045-7) Hành động "Đề nghị gộp" trên issue trùng định danh (`merge_requested` + lý do, `ActionGate`), sau task F011 — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/sis/` — kiểm: `make web-test` — cỡ S — ✅ 2026-11-21 (sau F011 T041): `ResolveIssueDialog` thêm "Đề nghị gộp tài khoản" (`merge_requested`, chỉ issue trùng định danh chưa đề nghị, gợi ý, lý do bắt buộc, Idempotency-Key), cột Trạng thái có thẻ "Đã đề nghị gộp"; test `sis.test.tsx` T061 (FAIL → PASS), vitest `school-lms/sis` PASS
- [x] T062 **OQ chốt 2026-11-20** (OQ-17, F052-AC-32) Ô "Giới hạn chơi mỗi ngày" trong Cài đặt trường, sau task F015; test F052-AC-32 — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/settings/` — kiểm: `make web-test` — cỡ S — ✅ 2026-11-20: `DailyPlayLimitCard` trong Cài đặt trường: "Tắt" khi `daily_play_limit_minutes` null, Switch + InputNumber với min/max lấy từ `LIMITS.dailyPlayLimit` (đối chiếu `SchoolSettings.daily_play_limit_minutes` bởi `fieldSchemas.test.ts`), lý do bắt buộc, `If-Match`, tắt → `null`, 422 tại ô, 412 tải lại; không sửa với trường ảo `common` hoặc khi thiếu action `edit_school`; test `settings.test.tsx` "Giới hạn chơi mỗi ngày" PASS
- [x] T063 **OQ chốt 2026-11-20** (OQ F012-1, F052-AC-13) Hộp "Đổi email" cho nhân sự trong Người dùng: `updateTenantUser` `email` với `If-Match` = `W/"<version>"` của dòng, `409 EMAIL_TAKEN` tại ô, `412` tải lại danh sách, ghi chú email mới chưa xác minh trên SSO; username không đổi; `ActionGate` `action.school_lms.access_control.edit_users` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/users/` — kiểm: `make web-test` — cỡ S — ✅ 2026-11-20: `ChangeEmailDialog` + hook `useUpdateStaffEmail` (`PATCH /tenant/users/{user_id}` chỉ `email`, `If-Match: W/"<version>"`), nút "Đổi email" sau `ActionGate` `edit_users`, `409 EMAIL_TAKEN` tại ô, `412` tải lại danh sách, ghi chú email chưa xác minh trên SSO, không có ô username; test `access.test.tsx` "change email" PASS

## ADR-0095 — "Học sinh theo khối", gỡ màn lớp (WP-P1, 2026-11-21)

Nguồn: Clarifications "Session 2026-11-21 (ADR-0095 — [Confirmed])" của spec; CODE_PLAN §4.2 WP-P1.

- [x] T064 Gỡ `classes/` (danh sách lớp, học sinh của lớp, `useClasses`), `students/UnassignedStudentsPage.tsx` và tab "Học sinh chưa có lớp" ở màn SIS, cột lớp ở xem trước dọn học sinh, công tắc "Bảng xếp hạng lớp" (D5); màn `menu.school_lms.students` "Học sinh theo khối" (trường → khối → học sinh qua `listTenantGradeStudents`, chỉ đọc, hoạt động đăng nhập, đặt lại mật khẩu); "Hoạt động học sinh" chọn trường → khối → học sinh; gỡ nhãn SIS `classes`/`enrollments`, loại đối soát lớp, `class_template` (F052-AC-04, AC-06, AC-07, AC-31; AC-05 Removed) — file: `students/{StudentsScreen,StudentActivityScreen,ResetStudentPasswordDialog,useStudentPassword}.tsx`, `sis/SisScreen.tsx`, `student-cleanup/CleanupPreviewPage.tsx`, `settings/SchoolSettingsPage.tsx` — kiểm: `students/studentsByGrade.test.tsx`, `settings.test.tsx`
- [ ] T065 Thao tác khóa/mở khóa học sinh (`action.school_lms.students.lock`, `lockTenantUser`/`unlockTenantUser`, lý do bắt buộc) trên "Học sinh theo khối" — chưa làm ở WP-P1 (màn lớp cũ cũng chưa có); khóa menu, quyền và `listTenantGradeStudents` đã xong ở WP-I1

## Dependencies & Execution Order

- Setup → Foundational (T004–T008; backend owner F010/F011/F015 hiện thực endpoint mới) → US1, US2 (song song) → US3, US4 (song song) → US5 → Polish.
- T016 cần T015; T026 cần T005 + backend F011; T042 cần T006 + backend F015; T035 cần T007.

## Definition of Done

- [x] Mọi tiêu chí F052-AC-* có test pass (ghi mã test/TC) — mức integration toàn portal với MOCK backend; E2E backend thật còn mở (OQ F051-9); Câu hỏi mở F052-1 đã giải quyết ở 5c7bd53 (backend 3 operation, TestSchoolScopedRoutes)
- [x] `make web-lint`, `make web-test` xanh (lệnh pnpm tương đương, quickstart §5); Playwright `@F052` chờ OQ F051-9
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — MOCK chỉ trong `src/test/` và test
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — SIS_INTEGRATION §9–§10, API_DESIGN, CHANGELOG; CONFIG_DEFAULTS không đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
