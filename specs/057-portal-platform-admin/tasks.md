# Tasks: F057 — Workspace D — Platform Administration: tenant, gán/clone, thùng rác, danh mục, cấu hình hệ thống

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo chunk workspace D, route lazy `/portal/platform-admin/*` có guard role `system_admin` từ `getMyPermissions` — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx`
- [ ] T002 [P] Biến môi trường base URL gateway nội bộ `VITE_EW_INTERNAL_API_BASE_URL` (chỉ `.env.example`) — file: `src/cms/frontend/apps/portal/.env.example`
- [ ] T003 [P] File dịch vi/en (`platform.*`, `error.MFA_REQUIRED`, `error.RETENTION_BELOW_MINIMUM`, `error.CONFLICT`, thông điệp "Cần kết nối mạng quản trị (VPN)") — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/i18n/vi.json`, `en.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract (owner F010/F045): `createTenant`, `updateTenant`, schema `TenantCreate`/`TenantUpdate` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **2026-11-17:** `createTenant`, `updateTenant`, `TenantCreate`, `TenantUpdate` đã áp bởi owner F010 (contracts §6 của F057 tham chiếu bản owner)
- [ ] T005 Delta contract (owner F045): `previewTenantClone` + `TenantClonePreview` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T006 Delta contract (owner F016): `cancelStudentCleanupRun`, thêm `cancelled` vào `StudentCleanupRun.status`; cập nhật ADMIN_PORTAL §8 — file: `specs/openapi/eduworld-api.v1.yaml`, `docs/04-admin-portal/ADMIN_PORTAL.md` — kiểm: `make specs-lint`
- [ ] T007 Sinh lại client TS — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff
- [ ] T008 [P] Xử lý lỗi chung workspace D: `403 MFA_REQUIRED` → step-up, lỗi mạng gateway nội bộ → `VpnRequiredBanner` — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/errors/MfaStepUp.tsx`, `VpnRequiredBanner.tsx`
- [ ] T009 [P] MSW handler test-only (nhãn `MOCK`) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/msw/handlers.ts`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Bộ lọc tenant liên tenant (Priority: P1) 🎯 MVP

**Goal**: `TenantFilter` + quy ước request liên tenant dùng chung.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Test F057-AC-01: tìm `listTenants` (`q` `maxLength`, debounce, cursor), mặc định "Tất cả", chọn nhiều, giữ trong `sessionStorage` (TC-TEN-018) — file: `src/cms/frontend/packages/ops-ui/src/tenant/TenantFilter.test.tsx` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Test F057-AC-02: list System Admin không `X-Tenant-Id`, có `tenant_ids[]`; cột Tenant + Trường với `unit` (TC-CMS-TEN-02) — file: `src/cms/frontend/apps/portal/src/shared/tenancy/useCrossTenantScope.test.ts`
- [ ] T012 [P] [US1] Test F057-AC-03: `TargetTenantField` bắt buộc một tenant hoặc Platform, header `X-Tenant-Id` = tenant đích (TC-CMS-TEN-03) — file: `src/cms/frontend/packages/ops-ui/src/tenant/TargetTenantField.test.tsx`
- [ ] T013 [P] [US1] Test F057-AC-04: người dùng tenant không thấy filter/cột, không gửi `tenant_ids` (TC-CMS-TEN-01, TC-TEN-011) — file: `src/cms/frontend/apps/portal/src/shared/tenancy/useCrossTenantScope.tenantUser.test.ts`

### Implementation cho User Story 1

- [ ] T014 [P] [US1] `TenantFilter`, `TenantColumn`, `TargetTenantField` + story — file: `src/cms/frontend/packages/ops-ui/src/tenant/TenantFilter.tsx`, `TenantColumn.tsx`, `TargetTenantField.tsx`
- [ ] T015 [US1] Hook `useCrossTenantScope` và tích hợp với `apiClient` (F050) — file: `src/cms/frontend/apps/portal/src/shared/tenancy/useCrossTenantScope.ts`
- [ ] T016 [US1] Gắn `TenantFilter` vào topbar shell (điểm mở rộng của F051) — file: `src/cms/frontend/apps/portal/src/shell/topbar/TopbarTenantSlot.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Gán dữ liệu platform (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Test F057-AC-05: hộp gán chọn nhiều tenant, `assignToTenants` + `Idempotency-Key`, tab `listTenantAssignments` (TC-TEN-019) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/AssignToTenantsDialog.test.tsx` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Test F057-AC-06: xác nhận gỡ gán có thông điệp lịch sử giữ nguyên, `unassignFromTenants` — file: `…/platform/__tests__/AssignedTenantsTab.test.tsx`
- [ ] T019 [US2] E2E gán → `ad.truong.a` thấy chỉ đọc → gỡ gán (TC-CMS-ASG-02) — file: `src/cms/frontend/apps/portal/e2e/platform/assignments.spec.ts`

### Implementation cho User Story 2

- [ ] T020 [P] [US2] Hook gán — file: `…/platform/api/assignments.ts`
- [ ] T021 [US2] `AssignToTenantsDialog`, `AssignedTenantsTab` (export cho workspace A/B) — file: `…/platform/assignments/AssignToTenantsDialog.tsx`, `AssignedTenantsTab.tsx`

## Phase 5: User Story 3 — Clone giữa tenant (Priority: P2)

### Tests cho User Story 3

- [ ] T022 [P] [US3] Test F057-AC-07: wizard nguồn/loại/bản ghi (`maxItems`)/đích ≠ nguồn, `createTenantClone` + `Idempotency-Key`, polling `getTenantClone`, hiển thị `id_map`/`errors` (TC-CMS-CLN-01) — file: `…/platform/__tests__/CloneWizard.test.tsx` — phải FAIL trước khi code
- [ ] T023 [P] [US3] Test F057-AC-08: không đích "mọi tenant", không loại dữ liệu học sinh, bước xem trước khi có `previewTenantClone` (TC-TEN-020) — file: `…/platform/__tests__/CloneWizard.rules.test.tsx`

### Implementation cho User Story 3

- [ ] T024 [P] [US3] Hook clone (+ preview sau T005) — file: `…/platform/api/clones.ts`
- [ ] T025 [US3] `CloneWizard`, `CloneJobPage` — file: `…/platform/clones/CloneWizard.tsx`, `CloneJobPage.tsx`

## Phase 6: User Story 4 — Thùng rác mọi tenant (Priority: P2)

### Tests cho User Story 4

- [ ] T026 [P] [US4] Test F057-AC-09: `listTrash` lọc `resource_type` + `tenant_ids[]`, khôi phục có lý do + `Idempotency-Key` (TC-CMS-DEL-01) — file: `…/platform/__tests__/TrashPage.test.tsx` — phải FAIL trước khi code
- [ ] T027 [P] [US4] Test F057-AC-10: `restorable=false`/`409` → thông điệp; DOM không có "Xóa vĩnh viễn" (TC-CMS-DEL-02) — file: `…/platform/__tests__/TrashPage.conflict.test.tsx`

### Implementation cho User Story 4

- [ ] T028 [US4] Hook + trang Thùng rác — file: `…/platform/api/trash.ts`, `…/platform/trash/TrashPage.tsx`

## Phase 7: User Story 5 — Danh mục (Priority: P2)

### Tests cho User Story 5

- [ ] T029 [P] [US5] Test F057-AC-11: quốc gia chỉ bật/tắt + sửa tên có lý do, không xóa — file: `…/platform/__tests__/CountriesPage.test.tsx` — phải FAIL trước khi code
- [ ] T030 [P] [US5] Test F057-AC-12: tỉnh `status=all`, sửa `official_code`, tạo đơn vị + kế thừa, không sửa đè (TC-ORG-101) — file: `…/platform/__tests__/ProvincesPage.test.tsx`
- [ ] T031 [P] [US5] Test F057-AC-13: import CSV dry-run → bảng lỗi dòng → xác nhận áp dụng; danh mục trường lọc tenant/tỉnh/cấp/trạng thái (TC-ORG-103) — file: `…/platform/__tests__/SchoolImportWizard.test.tsx`

### Implementation cho User Story 5

- [ ] T032 [P] [US5] Hook catalog, school import — file: `…/platform/api/catalog.ts`, `schoolImports.ts`
- [ ] T033 [US5] Trang quốc gia, tỉnh/thành, hộp kế thừa, danh mục trường, wizard import — file: `…/platform/catalog/CountriesPage.tsx`, `ProvincesPage.tsx`, `ProvinceSuccessionDialog.tsx`, `SchoolCatalogPage.tsx`, `SchoolImportWizard.tsx`

## Phase 8: User Story 6 — Cấu hình hệ thống (Priority: P2)

### Tests cho User Story 6

- [ ] T034 [P] [US6] Test F057-AC-14: form dọn học sinh theo schema, lý do bắt buộc, cảnh báo ngưỡng thấp (TC-AUTH-020) — file: `…/platform/__tests__/StudentCleanupSettingsPage.test.tsx` — phải FAIL trước khi code
- [ ] T035 [P] [US6] Test F057-AC-15: danh sách lần dọn, Xác nhận `approveStudentCleanupRun` + `Idempotency-Key`, Hủy sau T006 (TC-AUTH-021) — file: `…/platform/__tests__/CleanupRunsPage.test.tsx`
- [ ] T051 [P] [US6] Test F057-AC-23: khối giờ giới nghiêm platform hiển thị `is_default`, khung qua nửa đêm; HH:MM sai → lỗi tại ô; thiếu lý do bị chặn; hộp xác nhận; `putPlatformCurfewSettings` + `If-Match`; mock `412`/`422` hiển thị theo `code` (TC-AUTH-092) — file: `…/platform/__tests__/PlatformCurfewSettingsCard.test.tsx` — phải FAIL trước khi code
- [ ] T036 [P] [US6] Test F057-AC-16: chat retention chặn dưới `minimum_days`, hiển thị `RETENTION_BELOW_MINIMUM` — file: `…/platform/__tests__/ChatRetentionPage.test.tsx`

### Implementation cho User Story 6

- [ ] T037 [US6] Hook + trang cấu hình — file: `…/platform/api/cleanup.ts`, `chatRetention.ts`, `…/platform/settings/StudentCleanupSettingsPage.tsx`, `CleanupRunsPage.tsx`, `ChatRetentionPage.tsx`
- [ ] T052 [US6] Hook `usePlatformCurfew` (`getPlatformCurfewSettings`, `putPlatformCurfewSettings`) + `PlatformCurfewSettingsCard` trong màn Cấu hình hệ thống (action `action.platform_admin.system_settings.view`/`.edit`) — file: `…/platform/api/curfew.ts`, `…/platform/settings/PlatformCurfewSettingsCard.tsx`

## Phase 9: User Story 7 — Quản lý tenant (Priority: P3)

### Tests cho User Story 7

- [ ] T038 [P] [US7] Test F057-AC-17: danh sách lọc `q`/`tenant_type`, form tạo/sửa (sau T004) theo `maxLength`, xác nhận khi tắt, `409` — file: `…/platform/__tests__/TenantListPage.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 7

- [ ] T039 [US7] Hook + trang tenant — file: `…/platform/api/tenants.ts`, `…/platform/tenants/TenantListPage.tsx`, `TenantForm.tsx`

## Phase 10: User Story 8 — Bảo vệ workspace D (Priority: P1)

### Tests cho User Story 8

- [ ] T040 [P] [US8] E2E F057-AC-18: `ad.truong.a`, `cm.noidung`, `sm.antoan` mở URL D → trang 403, network log không có chunk D (TC-CMS-AUTHZ-05, TC-CMS-WS-01) — file: `src/cms/frontend/apps/portal/e2e/platform/authz.spec.ts` — phải FAIL trước khi code
- [ ] T041 [P] [US8] Test F057-AC-19: `MFA_REQUIRED` → step-up và quay lại URL; lỗi mạng → banner VPN (TC-CMS-WS-02) — file: `…/platform/__tests__/errors.test.tsx`
- [ ] T042 [P] [US8] Test F057-AC-20: mọi mutation D có `Idempotency-Key` giữ nguyên khi retry; lỗi hiển thị `code` + `trace_id` (TC-CMS-ASG-01) — file: `…/platform/__tests__/idempotency.test.ts`
- [ ] T043 [P] [US8] Test F057-AC-21: ô nhập theo `maxLength` + bộ đếm (TC-TEN-024) — file: `…/platform/__tests__/inputLimits.test.tsx`

### Implementation cho User Story 8

- [ ] T044 [US8] Guard route theo role, nối `MfaStepUp`/`VpnRequiredBanner` vào mọi page D — file: `…/platform/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T045 [P] Kiểm không import component/Theme game (TC-UI-010), lint hardcode style (TC-UI-003) — kiểm: `make web-lint`
- [ ] T046 [P] axe + visual regression màn D (TC-UI-004, TC-UI-006) — file: `src/cms/frontend/apps/portal/e2e/platform/a11y-visual.spec.ts`
- [ ] T047 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md`
- [ ] T048 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [ ] T049 [P] Test F057-AC-22: tắt `action.platform_admin.tenant_data.clone` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/platform-admin/feature-toggle.spec.ts`
- [ ] T050 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F057-AC-22) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational → US1 (chặn F053/F054/F055/F056 phần liên tenant) → US8 → US2 → US4, US5, US6 (song song) → US3 (cần T005 cho bước xem trước) → US7 (cần T004) → Polish.

## Definition of Done

- [ ] Mọi tiêu chí F057-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật qua gateway nội bộ
- [ ] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
