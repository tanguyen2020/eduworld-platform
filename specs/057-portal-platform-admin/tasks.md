# Tasks: F057 — Workspace D — Platform Administration: tenant, gán/clone, thùng rác, danh mục, cấu hình hệ thống

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo chunk workspace D, route lazy `/portal/platform-admin/*` có guard role `system_admin` từ `getMyPermissions` — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx` — **2026-11-19:** chunk `src/workspaces/platform-admin/index.tsx` (cấu trúc F051: `WorkspaceRoutes` sinh từ catalog, `WorkspaceGate` theo `getMyPermissions`); route `/platform-admin/*` (path của catalog, OQ F051-1)
- [x] T002 [P] Biến môi trường base URL gateway nội bộ `VITE_EW_INTERNAL_API_BASE_URL` (chỉ `.env.example`) — file: `src/cms/frontend/apps/portal/.env.example` — **2026-11-19:** đã có từ F050/F051 (`apiInternalBaseUrl` của `/config.json`, fallback `VITE_API_INTERNAL_BASE_URL`) — không thêm biến, không sửa `.env*` (OQ F057-6)
- [x] T003 [P] File dịch vi/en (`platform.*`, `error.MFA_REQUIRED`, `error.RETENTION_BELOW_MINIMUM`, `error.CONFLICT`, thông điệp "Cần kết nối mạng quản trị (VPN)") — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/i18n/vi.json`, `en.json` — **2026-11-19:** `i18n/{vi,en}.json` (`platform.*`, nạp khi chunk mount) + `error.RETENTION_BELOW_MINIMUM`, `error.INVALID_STATE_TRANSITION`, `shell.reason.minLength`, `shell.tenantFilter.disabled` ở file dịch chung; kiểm `limits.test.ts` › translations

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract (owner F010/F045): `createTenant`, `updateTenant`, schema `TenantCreate`/`TenantUpdate` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **2026-11-17:** `createTenant`, `updateTenant`, `TenantCreate`, `TenantUpdate` đã áp bởi owner F010 (contracts §6 của F057 tham chiếu bản owner) — (xác nhận 2026-11-19: `createTenant`/`updateTenant` có trong `operation-meta.gen.ts`)
- [ ] T005 Delta contract (owner F045): `previewTenantClone` + `TenantClonePreview` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-19:** chưa áp — owner F045, chưa có backend; wizard bỏ bước 3 và ghi rõ trên màn (OQ F057-1)
- [x] T006 Delta contract (owner F016): `cancelStudentCleanupRun`, thêm `cancelled` vào `StudentCleanupRun.status`; cập nhật ADMIN_PORTAL §8 — file: `specs/openapi/eduworld-api.v1.yaml`, `docs/04-admin-portal/ADMIN_PORTAL.md` — kiểm: `make specs-lint` — **2026-11-19:** chưa áp — owner F016; không có nút Hủy, màn ghi rõ (OQ F057-2) — **2026-11-20:** contract + backend identity đã áp (Clarification 2026-11-20); nút Hủy trên portal chờ bật `CLEANUP_CANCEL_AVAILABLE` — **2026-11-20 (portal):** nút "Hủy lần dọn" đã bật (`useCancelRun`, `settings/CleanupRunsPage.tsx`); cờ `CLEANUP_CANCEL_AVAILABLE` đã gỡ
- [x] T007 Sinh lại client TS — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff — **2026-11-19:** F057 không đổi contract; `pnpm gen:check` khớp spec (file sinh: `src/shared/api/schema.gen.d.ts` + `operation-meta.gen.ts`)
- [x] T008 [P] Xử lý lỗi chung workspace D: `403 MFA_REQUIRED` → step-up, lỗi mạng gateway nội bộ → `VpnRequiredBanner` — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/errors/MfaStepUp.tsx`, `VpnRequiredBanner.tsx` — **2026-11-19:** dùng `ReLoginDialog` (step-up, F051-AC-13) và `VpnRequiredNotice` của shell F051 cho workspace D (`INTERNAL_WORKSPACES`); kiểm `__tests__/workspaceGuard.test.tsx`
- [x] T009 [P] MSW handler test-only (nhãn `MOCK`) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/msw/handlers.ts` — **2026-11-19:** backend MOCK `src/test/platform-backend.ts` (nhãn MOCK, cắm vào `MockPortalBackend` của F051) thay MSW (OQ F057-9)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Bộ lọc tenant liên tenant (Priority: P1) 🎯 MVP

**Goal**: `TenantFilter` + quy ước request liên tenant dùng chung.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Test F057-AC-01: tìm `listTenants` (`q` `maxLength`, debounce, cursor), mặc định "Tất cả", chọn nhiều, giữ trong `sessionStorage` (TC-TEN-018) — file: `src/cms/frontend/packages/ops-ui/src/tenant/TenantFilter.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/TenantListPage.test.tsx` › Bộ lọc tenant: cursor và tenant đã tắt; tìm/maxLength/sessionStorage đã có ở `shell/TenantFilter.test.tsx` (F051)
- [x] T011 [P] [US1] Test F057-AC-02: list System Admin không `X-Tenant-Id`, có `tenant_ids[]`; cột Tenant + Trường với `unit` (TC-CMS-TEN-02) — file: `src/cms/frontend/apps/portal/src/shared/tenancy/useCrossTenantScope.test.ts` — **2026-11-19:** `src/shared/tenancy/useCrossTenantScope.test.ts` (3 pass) + `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/TrashPage.test.tsx` › tenant filter → tenant_ids[], cột Tenant
- [x] T012 [P] [US1] Test F057-AC-03: `TargetTenantField` bắt buộc một tenant hoặc Platform, header `X-Tenant-Id` = tenant đích (TC-CMS-TEN-03) — file: `src/cms/frontend/packages/ops-ui/src/tenant/TargetTenantField.test.tsx` — **2026-11-19:** `useCrossTenantScope.test.ts` › writeHeaders; `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/Catalog.test.tsx` › Import (tenant đích bắt buộc, `X-Tenant-Id`); `TrashPage.test.tsx` › restore (`X-Tenant-Id` của bản ghi). `TenantTargetField` là của F051 (OQ F057-5)
- [x] T013 [P] [US1] Test F057-AC-04: người dùng tenant không thấy filter/cột, không gửi `tenant_ids` (TC-CMS-TEN-01, TC-TEN-011) — file: `src/cms/frontend/apps/portal/src/shared/tenancy/useCrossTenantScope.tenantUser.test.ts` — **2026-11-19:** gộp vào `src/shared/tenancy/useCrossTenantScope.test.ts` › tenant user; `shell/TenantFilter.test.tsx` (F051) kiểm UI

### Implementation cho User Story 1

- [x] T014 [P] [US1] `TenantFilter`, `TenantColumn`, `TargetTenantField` + story — file: `src/cms/frontend/packages/ops-ui/src/tenant/TenantFilter.tsx`, `TenantColumn.tsx`, `TargetTenantField.tsx` — **2026-11-19:** dùng `TenantFilter`/`TenantTargetField`/`useTenantColumn` của F051 trong `src/shell/TenantControls.tsx` (không chuyển sang ops-ui vì gọi API, OQ F057-5); bổ sung phân trang cursor khi cuộn (`useTenantOptions` → `useInfiniteQuery`, `loadMoreOnScroll`) và nhãn "đã tắt"
- [x] T015 [US1] Hook `useCrossTenantScope` và tích hợp với `apiClient` (F050) — file: `src/cms/frontend/apps/portal/src/shared/tenancy/useCrossTenantScope.ts` — **2026-11-19:** `src/shared/tenancy/useCrossTenantScope.ts`
- [x] T016 [US1] Gắn `TenantFilter` vào topbar shell (điểm mở rộng của F051) — file: `src/cms/frontend/apps/portal/src/shell/topbar/TopbarTenantSlot.tsx` — **2026-11-19:** topbar đã gắn từ F051 (`PortalLayout`); các màn liên tenant của D đặt thêm `TenantFilter` trong thanh lọc

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Gán dữ liệu platform (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Test F057-AC-05: hộp gán chọn nhiều tenant, `assignToTenants` + `Idempotency-Key`, tab `listTenantAssignments` (TC-TEN-019) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/AssignToTenantsDialog.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/AssignToTenantsDialog.test.tsx` (4 pass)
- [x] T018 [P] [US2] Test F057-AC-06: xác nhận gỡ gán có thông điệp lịch sử giữ nguyên, `unassignFromTenants` — file: `…/platform/__tests__/AssignedTenantsTab.test.tsx` — **2026-11-19:** gộp vào `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/AssignToTenantsDialog.test.tsx` › Gỡ gán
- [ ] T019 [US2] E2E gán → `ad.truong.a` thấy chỉ đọc → gỡ gán (TC-CMS-ASG-02) — file: `src/cms/frontend/apps/portal/e2e/platform/assignments.spec.ts` — **2026-11-19:** chưa làm — E2E với backend thật chờ môi trường compose (OQ F057-10)

### Implementation cho User Story 2

- [x] T020 [P] [US2] Hook gán — file: `…/platform/api/assignments.ts` — **2026-11-19:** `api/assignments.ts`
- [x] T021 [US2] `AssignToTenantsDialog`, `AssignedTenantsTab` (export cho workspace A/B) — file: `…/platform/assignments/AssignToTenantsDialog.tsx`, `AssignedTenantsTab.tsx` — **2026-11-19:** `assignments/AssignToTenantsDialog.tsx`, `AssignedTenantsTab.tsx` (export cho A/B; `tenantData/TenantDataPage.tsx` mở theo `?resource_type=&resource_id=`)

## Phase 5: User Story 3 — Clone giữa tenant (Priority: P2)

### Tests cho User Story 3

- [x] T022 [P] [US3] Test F057-AC-07: wizard nguồn/loại/bản ghi (`maxItems`)/đích ≠ nguồn, `createTenantClone` + `Idempotency-Key`, polling `getTenantClone`, hiển thị `id_map`/`errors` (TC-CMS-CLN-01) — file: `…/platform/__tests__/CloneWizard.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/CloneWizard.test.tsx` › Clone giữa tenant (2 pass)
- [x] T023 [P] [US3] Test F057-AC-08: không đích "mọi tenant", không loại dữ liệu học sinh, bước xem trước khi có `previewTenantClone` (TC-TEN-020) — file: `…/platform/__tests__/CloneWizard.rules.test.tsx` — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/CloneWizard.test.tsx` › Quy tắc clone; `limits.test.ts` › không có loại dữ liệu học sinh

### Implementation cho User Story 3

- [x] T024 [P] [US3] Hook clone (+ preview sau T005) — file: `…/platform/api/clones.ts` — **2026-11-19:** `api/clones.ts` (preview chờ T005)
- [x] T025 [US3] `CloneWizard`, `CloneJobPage` — file: `…/platform/clones/CloneWizard.tsx`, `CloneJobPage.tsx` — **2026-11-19:** `clones/CloneWizard.tsx`, `clones/CloneJobPage.tsx` (`CloneJobView`)

## Phase 6: User Story 4 — Thùng rác mọi tenant (Priority: P2)

### Tests cho User Story 4

- [x] T026 [P] [US4] Test F057-AC-09: `listTrash` lọc `resource_type` + `tenant_ids[]`, khôi phục có lý do + `Idempotency-Key` (TC-CMS-DEL-01) — file: `…/platform/__tests__/TrashPage.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/TrashPage.test.tsx` (6 pass)
- [x] T027 [P] [US4] Test F057-AC-10: `restorable=false`/`409` → thông điệp; DOM không có "Xóa vĩnh viễn" (TC-CMS-DEL-02) — file: `…/platform/__tests__/TrashPage.conflict.test.tsx` — **2026-11-19:** gộp vào `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/TrashPage.test.tsx` › Không khôi phục được

### Implementation cho User Story 4

- [x] T028 [US4] Hook + trang Thùng rác — file: `…/platform/api/trash.ts`, `…/platform/trash/TrashPage.tsx` — **2026-11-19:** `api/trash.ts`, `trash/TrashPage.tsx`

## Phase 7: User Story 5 — Danh mục (Priority: P2)

### Tests cho User Story 5

- [x] T029 [P] [US5] Test F057-AC-11: quốc gia chỉ bật/tắt + sửa tên có lý do, không xóa — file: `…/platform/__tests__/CountriesPage.test.tsx` — phải FAIL trước khi code — **2026-11-19:** gộp vào `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/Catalog.test.tsx` › Quốc gia
- [x] T030 [P] [US5] Test F057-AC-12: tỉnh `status=all`, sửa `official_code`, tạo đơn vị + kế thừa, không sửa đè (TC-ORG-101) — file: `…/platform/__tests__/ProvincesPage.test.tsx` — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/Catalog.test.tsx` › Tỉnh/thành (2 pass) — **2026-11-20 (portal):** sửa quốc gia/tỉnh gửi `If-Match` = `version` của dòng (OQ F057-3) — `Catalog.test.tsx` › Quốc gia, Tỉnh/thành
- [x] T031 [P] [US5] Test F057-AC-13: import CSV dry-run → bảng lỗi dòng → xác nhận áp dụng; danh mục trường lọc tenant/tỉnh/cấp/trạng thái (TC-ORG-103) — file: `…/platform/__tests__/SchoolImportWizard.test.tsx` — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/Catalog.test.tsx` › Danh mục trường, Import trường CSV — **2026-11-20 (portal):** danh mục trường liên tenant `tenant_ids[]` (≥ 1 tenant, cột Tenant) + lọc `status` (OQ F057-4) — `Catalog.test.tsx` › Danh mục trường

### Implementation cho User Story 5

- [x] T032 [P] [US5] Hook catalog, school import — file: `…/platform/api/catalog.ts`, `schoolImports.ts` — **2026-11-19:** `api/catalog.ts`, `api/schoolImports.ts` — **2026-11-20 (portal):** `useCatalogSchools` theo `useCrossTenantScope.listQuery`, `status`; `versionIfMatch` cho `useUpdateCountry`/`useUpdateProvince`
- [x] T033 [US5] Trang quốc gia, tỉnh/thành, hộp kế thừa, danh mục trường, wizard import — file: `…/platform/catalog/CountriesPage.tsx`, `ProvincesPage.tsx`, `ProvinceSuccessionDialog.tsx`, `SchoolCatalogPage.tsx`, `SchoolImportWizard.tsx` — **2026-11-19:** `catalog/CatalogPage.tsx` (tab), `CountriesPage.tsx`, `ProvincesPage.tsx`, `ProvinceSuccessionDialog.tsx`, `SchoolCatalogPage.tsx`, `SchoolImportWizard.tsx`

## Phase 8: User Story 6 — Cấu hình hệ thống (Priority: P2)

### Tests cho User Story 6

- [x] T034 [P] [US6] Test F057-AC-14: form dọn học sinh theo schema, lý do bắt buộc, cảnh báo ngưỡng thấp (TC-AUTH-020) — file: `…/platform/__tests__/StudentCleanupSettingsPage.test.tsx` — phải FAIL trước khi code — **2026-11-19:** gộp vào `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/SystemSettings.test.tsx` › Dọn học sinh (2 pass)
- [x] T035 [P] [US6] Test F057-AC-15: danh sách lần dọn, Xác nhận `approveStudentCleanupRun` + `Idempotency-Key`, Hủy sau T006 (TC-AUTH-021) — file: `…/platform/__tests__/CleanupRunsPage.test.tsx` — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/SystemSettings.test.tsx` › Lần dọn chờ xác nhận (Hủy chờ T006) — **2026-11-20 (portal):** Hủy có lý do 3..500 → `cancelStudentCleanupRun`, thử lại giữ `Idempotency-Key`, `X-Tenant-Id` của lần dọn — `SystemSettings.test.tsx` › "Hủy lần dọn" (OQ F057-2)
- [x] T051 [P] [US6] Test F057-AC-23: khối giờ giới nghiêm platform hiển thị `is_default`, khung qua nửa đêm; HH:MM sai → lỗi tại ô; thiếu lý do bị chặn; hộp xác nhận; `putPlatformCurfewSettings` + `If-Match`; mock `412`/`422` hiển thị theo `code` (TC-AUTH-092) — file: `…/platform/__tests__/PlatformCurfewSettingsCard.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/SystemSettings.test.tsx` › Giờ giới nghiêm platform (2 pass)
- [x] T036 [P] [US6] Test F057-AC-16: chat retention chặn dưới `minimum_days`, hiển thị `RETENTION_BELOW_MINIMUM` — file: `…/platform/__tests__/ChatRetentionPage.test.tsx` — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/SystemSettings.test.tsx` › Thời hạn chat gameplay

### Implementation cho User Story 6

- [x] T037 [US6] Hook + trang cấu hình — file: `…/platform/api/cleanup.ts`, `chatRetention.ts`, `…/platform/settings/StudentCleanupSettingsPage.tsx`, `CleanupRunsPage.tsx`, `ChatRetentionPage.tsx` — **2026-11-19:** `api/cleanup.ts`, `api/chatRetention.ts`, `settings/SystemSettingsPage.tsx`, `StudentCleanupSettingsPage.tsx`, `CleanupRunsPage.tsx`, `ChatRetentionPage.tsx` — **2026-11-20 (portal):** `api/cleanup.ts` `useCancelRun`; `CleanupRunsPage.tsx` nút Hủy + `ReasonDialog`
- [x] T052 [US6] Hook `usePlatformCurfew` (`getPlatformCurfewSettings`, `putPlatformCurfewSettings`) + `PlatformCurfewSettingsCard` trong màn Cấu hình hệ thống (action `action.platform_admin.system_settings.view`/`.edit`) — file: `…/platform/api/curfew.ts`, `…/platform/settings/PlatformCurfewSettingsCard.tsx` — **2026-11-19:** `api/curfew.ts`, `settings/PlatformCurfewSettingsCard.tsx`

## Phase 9: User Story 7 — Quản lý tenant (Priority: P3)

### Tests cho User Story 7

- [x] T038 [P] [US7] Test F057-AC-17: danh sách lọc `q`/`tenant_type`, form tạo/sửa (sau T004) theo `maxLength`, xác nhận khi tắt, `409` — file: `…/platform/__tests__/TenantListPage.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/TenantListPage.test.tsx` (6 pass; sửa khi chưa biết version → 428 theo code, OQ F057-3) — **2026-11-20 (portal):** sửa tenant chưa ghi trong phiên gửi `If-Match` = `version` của dòng; version cũ → `412` theo code, tải lại danh sách → lần sau `W/"4"` (OQ F057-3)

### Implementation cho User Story 7

- [x] T039 [US7] Hook + trang tenant — file: `…/platform/api/tenants.ts`, `…/platform/tenants/TenantListPage.tsx`, `TenantForm.tsx` — **2026-11-19:** `api/tenants.ts`, `tenants/TenantListPage.tsx`, `TenantForm.tsx` — **2026-11-20 (portal):** `useUpdateTenant` nhận `version` (`versionIfMatch`), `412` → `reloadOnStaleVersion`

## Phase 10: User Story 8 — Bảo vệ workspace D (Priority: P1)

### Tests cho User Story 8

- [x] T040 [P] [US8] E2E F057-AC-18: `ad.truong.a`, `cm.noidung`, `sm.antoan` mở URL D → trang 403, network log không có chunk D (TC-CMS-AUTHZ-05, TC-CMS-WS-01) — file: `src/cms/frontend/apps/portal/e2e/platform/authz.spec.ts` — phải FAIL trước khi code — **2026-11-19:** mức integration `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/workspaceGuard.test.tsx` › Bảo vệ workspace D (`ad.truong.a`, `cm.noidung`, `sm.antoan` → 403, loader chunk D không được gọi, không request `/admin/*`); E2E network log chờ OQ F057-10
- [x] T041 [P] [US8] Test F057-AC-19: `MFA_REQUIRED` → step-up và quay lại URL; lỗi mạng → banner VPN (TC-CMS-WS-02) — file: `…/platform/__tests__/errors.test.tsx` — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/workspaceGuard.test.tsx` › MFA step-up và VPN
- [x] T042 [P] [US8] Test F057-AC-20: mọi mutation D có `Idempotency-Key` giữ nguyên khi retry; lỗi hiển thị `code` + `trace_id` (TC-CMS-ASG-01) — file: `…/platform/__tests__/idempotency.test.ts` — **2026-11-19:** `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/AssignToTenantsDialog.test.tsx` › retry giữ `Idempotency-Key`; khóa UUIDv7 kiểm ở mọi test ghi (gán, gỡ gán, clone, khôi phục, import, duyệt, tạo tenant/kế thừa); PUT/PATCH theo `If-Match` (OQ F057-7)
- [x] T043 [P] [US8] Test F057-AC-21: ô nhập theo `maxLength` + bộ đếm (TC-TEN-024) — file: `…/platform/__tests__/inputLimits.test.tsx` — **2026-11-19:** `src/workspaces/platform-admin/limits.test.ts` (34 pass: mọi giới hạn = OpenAPI, ngưỡng CONFIG_DEFAULTS) + bộ đếm/`maxlength` trong `TenantListPage.test.tsx`, `TrashPage.test.tsx`, `Catalog.test.tsx`

### Implementation cho User Story 8

- [x] T044 [US8] Guard route theo role, nối `MfaStepUp`/`VpnRequiredBanner` vào mọi page D — file: `…/platform/routes.tsx` — **2026-11-19:** `WorkspaceGate` (F051) + `index.tsx`; MFA/VPN qua apiClient/shell cho mọi màn D

## Phase N: Polish & Cross-Cutting

- [x] T045 [P] Kiểm không import component/Theme game (TC-UI-010), lint hardcode style (TC-UI-003) — kiểm: `make web-lint` — **2026-11-19:** `pnpm lint` (ESLint `ew/no-hardcoded-style`, `ew/no-game-imports`, i18n) xanh
- [ ] T046 [P] axe + visual regression màn D (TC-UI-004, TC-UI-006) — file: `src/cms/frontend/apps/portal/e2e/platform/a11y-visual.spec.ts` — **2026-11-19:** chưa làm — axe/visual toàn trang chờ E2E (OQ F057-10)
- [x] T047 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md` — **2026-11-19:** `CHANGELOG.md`
- [ ] T048 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng — **2026-11-19:** chưa chạy với backend thật (OQ F057-10); bằng chứng integration ở quickstart §4
- [x] T049 [P] Test F057-AC-22: tắt `action.platform_admin.tenant_data.clone` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/platform-admin/feature-toggle.spec.ts` — **2026-11-19:** mức integration `src/cms/frontend/apps/portal/src/workspaces/platform-admin/__tests__/workspaceGuard.test.tsx` › Menu/action tắt (3 pass); E2E chờ OQ F057-10
- [x] T050 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F057-AC-22) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/routes.tsx` — **2026-11-19:** `index.tsx` gắn màn vào khóa menu catalog; tab/nút theo `actionStatus`/`<ActionGate>`; `check-portal-catalog` 0 lỗi

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T053 **OQ chốt 2026-11-20** (OQ F057-1) Bước 3 wizard clone gọi `previewTenantClone` (đồ thị phụ thuộc, xung đột chặn bước tiếp), gỡ cờ `CLONE_PREVIEW_AVAILABLE`; sau task F045 — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/api/clones.ts`, `…/clones/` — kiểm: `make web-test` — cỡ M
- [x] T054 **OQ chốt 2026-11-20** (OQ F045-9) Ô `remove_default` trong hộp gỡ gán — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/assignments/` — kiểm: `make web-test` — cỡ S — ✅ 2026-11-20: ô "Bỏ luôn quy tắc gán mặc định cho tenant mới" trong hộp gỡ gán → `remove_default: true` (chỉ gửi khi chọn), cùng action `action.platform_admin.tenant_data.assign`; test `AssignToTenantsDialog.test.tsx` "Gỡ gán" mở rộng PASS

## Dependencies & Execution Order

- Setup → Foundational → US1 (chặn F053/F054/F055/F056 phần liên tenant) → US8 → US2 → US4, US5, US6 (song song) → US3 (cần T005 cho bước xem trước) → US7 (cần T004) → Polish.

## Definition of Done

- [x] Mọi tiêu chí F057-AC-* có test pass (ghi mã test/TC) — mức integration với backend MOCK theo contract; E2E backend thật còn mở (OQ F057-10)
- [x] `make web-lint`, `make web-test` xanh (lệnh pnpm tương đương); Playwright E2E với backend thật chờ OQ F057-10
- [x] Constitution Check trong plan.md vẫn đạt; backend MOCK chỉ trong test, gắn nhãn (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi (không đổi quy tắc/tham số; CHANGELOG có mục F057)
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
