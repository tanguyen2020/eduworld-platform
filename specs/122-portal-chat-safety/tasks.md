# Tasks: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `src/cms/frontend/apps/portal/` trừ khi ghi rõ. MSW chỉ trong test, nhãn `// MOCK (test-only)` (R17). Fixture danh mục chỉ placeholder (R15).

## Phase 1: Setup

- [x] T001 Route lazy `/live-ops/moderation/*`, `/platform-admin/chat-safety/*`, `/school-lms/safety/*` có `RequirePermission`; menu Safety Moderator chỉ Kiểm duyệt (khóa `menu.live_ops.*`, `menu.platform_admin.chat_safety`, `menu.platform_admin.moderation_switches`, `menu.school_lms.safety_notifications`, `menu.school_lms.chat_safety` từ registry sinh từ catalog, F051) — file: `src/workspaces/live-ops/moderation/routes.tsx`, `src/workspaces/platform-admin/chat-safety/routes.tsx`, `src/workspaces/school-lms/safety/routes.tsx` — làm: màn gắn vào khóa menu của catalog trong `src/workspaces/{live-ops,platform-admin,school-lms}/index.tsx` (route con trong màn: `/live-ops/moderation-cases/:caseId`, `/school-lms/safety-notifications/:notificationId`); không có `routes.tsx` riêng (route sinh từ catalog, ADR-0090)
- [x] T002 [P] File dịch vi/en (thông điệp theo `code`: `MODERATION_CASE_CLOSED`, `MODERATION_SWITCH_REASON_REQUIRED`, `TENANT_TOGGLE_NOT_ALLOWED`, `CHAT_POLICY_RELAX_NOT_ALLOWED`, `CHAT_POLICY_ALWAYS_ON`, `CONFLICT`, `MFA_REQUIRED`) — file: `src/workspaces/*/{moderation,chat-safety,safety}/i18n/{vi,en}.json` — làm: `src/i18n/chat-safety.{vi,en}.json` (`chatSafety.*`) + mã lỗi trong `error.*` (`MFA_REQUIRED` bỏ theo ADR-0094)
- [x] T003 [P] Fixture Playwright: tài khoản `sm.antoan`, `sa.admin`, `ad.truong.a`, `ad.truong.b` — file: `tests/e2e/fixtures/auth.ts` — làm 2026-11-24: dùng `tests/e2e/support/auth.ts` (đăng nhập chỉ mật khẩu, ADR-0094, mọi giá trị từ env) + spec `tests/e2e/chat-safety/chat-safety.spec.ts` (@F122). **Chưa chạy với backend thật**: cần `EW_E2E_PORTAL_URL`, `EW_E2E_PASSWORD` (không có trong phiên; không đọc env/secret) — `playwright test --grep @F122` → 3 skipped

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract `getTenantModerationStatus` + schema `TenantModerationStatus` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — làm 2026-11-23 (`x-portal-shell`)
- [x] T005 Hiện thực backend `getTenantModerationStatus` (query đọc công tắc hiệu lực, không trả lý do) + test authz/tenant isolation — file: `src/backend/modules/chat/internal/application/query/moderation/tenant_moderation_status.go`, `src/backend/modules/chat/internal/presentation/http/public/moderation_status.go`, `src/backend/modules/chat/tests/integration/moderation_status_test.go` — kiểm: `make test-integration MOD=chat` — làm: `query/moderation_status.go`, handler trong `presentation/http/catalog.go`, `tests/integration/moderation_status_test.go` (`TestTenantModerationStatus` PASS)
- [x] T006 Sinh lại client TS sau delta T004 và delta F120/F121 — file: `src/api/schema.d.ts` — kiểm: `pnpm gen:api` không diff trong CI — làm (`pnpm gen:api`)
- [x] T007 [P] `ops-ui`: `SensitiveTermTester`, `SlaBadge`, `AlwaysOnLock` + story sáng/tối — file: `src/cms/frontend/packages/ops-ui/src/chat-safety/` — làm (+ `chatSafety.test.tsx`)
- [x] T008 [P] MSW handler test-only cho mọi operationId ở contracts §1 — file: `src/workspaces/live-ops/moderation/__tests__/msw/handlers.ts` — làm bằng MOCK backend của harness portal (`src/test/chat-safety-mock.ts`, test-only, R17) thay MSW

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Hàng đợi case (Priority: P1) 🎯 MVP

**Goal**: Safety Team xử lý case liên tenant.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Test F122-AC-01: bộ lọc tenant (chỉ module Kiểm duyệt), mức, trạng thái, assignee, category, cột Tenant, `SlaBadge` quá hạn (TC-CHAT-042) — file: `src/workspaces/live-ops/moderation/__tests__/CaseQueuePage.test.tsx` — phải FAIL trước khi code
- [x] T010 [P] [US1] Test F122-AC-02: panel bằng chứng gốc/revision, tìm `q` trong case, DOM không có ô tìm chat tự do (TC-CHAT-040) — file: `…/__tests__/CaseDetailPage.test.tsx`
- [x] T011 [P] [US1] Test F122-AC-03: assign/act/sanction gửi `Idempotency-Key`, `If-Match`, `X-Tenant-Id` tenant case; lý do bắt buộc; `account_lock` xác nhận 2 bước (TC-CHAT-007) — file: `…/__tests__/CaseActions.test.tsx`
- [x] T012 [P] [US1] Test F122-AC-04: `412` → tải lại; `409`, `403` → thông điệp theo `code` + `trace_id`, không retry (TC-CMS-WS-02) — file: `…/__tests__/CaseErrors.test.tsx`

### Implementation cho User Story 1

- [x] T013 [P] [US1] Hook `cases.ts`, `sanctions.ts` — file: `src/workspaces/live-ops/moderation/api/` — `src/workspaces/live-ops/moderation/api.ts`
- [x] T014 [US1] `CaseQueuePage`, `CaseDetailPage`, `EvidencePanel`, `CaseActionBar`, `SanctionForm` — file: `src/workspaces/live-ops/moderation/cases/` — `CasesScreen.tsx`, `CaseDetail.tsx`, `SanctionsScreen.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Thông báo nhà trường (E) (Priority: P1)

### Tests cho User Story 2

- [x] T015 [P] [US2] Test F122-AC-05: chọn trường trong `school_ids`, học sinh của trường đó ≤ 20, tóm tắt 10–1000, ẩn nút với tenant `common` (TC-CHAT-043) — file: `…/__tests__/NotifySchoolDrawer.test.tsx` — phải FAIL trước khi code
- [x] T016 [P] [US2] Test F122-AC-06: danh sách platform lọc `overdue`/`acknowledged` — file: `…/__tests__/PlatformNotificationsPage.test.tsx`

### Implementation cho User Story 2

- [x] T017 [US2] Hook `schoolNotifications.ts`; `NotifySchoolDrawer`, `PlatformNotificationsPage` — file: `src/workspaces/live-ops/moderation/api/schoolNotifications.ts`, `cases/NotifySchoolDrawer.tsx`, `notifications/PlatformNotificationsPage.tsx` — `NotifySchoolDrawer.tsx`, tab trong `CasesScreen.tsx`

## Phase 5: User Story 3 — Danh mục từ nhạy cảm (Priority: P1)

### Tests cho User Story 3

- [x] T018 [P] [US3] Test F122-AC-07: CRUD category/term/allowlist theo schema, xóa mềm có xác nhận (TC-CHAT-015) — file: `src/workspaces/platform-admin/chat-safety/__tests__/Catalog.test.tsx` — phải FAIL trước khi code
- [x] T019 [P] [US3] Test F122-AC-08: thử chuỗi hiển thị quyết định/`masked_text`; publish cần `change_summary`; versions + hồi quy (TC-CHAT-012) — file: `…/__tests__/PublishAndTest.test.tsx`
- [x] T020 [P] [US3] Test F122-AC-09: Safety Moderator chỉ đọc; không có nội dung term trong storage/log (spy console, storage) — file: `…/__tests__/CatalogReadOnly.test.tsx`

### Implementation cho User Story 3

- [x] T021 [US3] Hook `catalog.ts`; `CategoriesPage`, `TermsPage`, `AllowlistPage`, `PublishDrawer`, `VersionsPage` — file: `src/workspaces/platform-admin/chat-safety/` — `src/workspaces/platform-admin/chat-safety/{api.ts,CatalogScreen.tsx}`, `src/shared/chatSafety/PublishDialog.tsx`

## Phase 6: User Story 4 — Công tắc kiểm duyệt (Priority: P1)

### Tests cho User Story 4

- [x] T022 [P] [US4] Test F122-AC-10: bảng Platform + tenant, khóa bước do cấp trên (TC-CHAT-013) — file: `…/chat-safety/__tests__/SwitchesPage.test.tsx` — phải FAIL trước khi code
- [x] T023 [P] [US4] Test F122-AC-11: hộp xác nhận rủi ro, lý do ≥ 10, `expires_at`, `X-Tenant-Id` tenant đích, map `MODERATION_SWITCH_REASON_REQUIRED` (TC-CHAT-015) — file: `…/__tests__/SwitchConfirmModal.test.tsx`
- [x] T024 [P] [US4] Test F122-AC-12: lịch sử before/after/`source` (TC-CHAT-016) — file: `…/__tests__/SwitchHistoryPage.test.tsx`
- [x] T051 [P] [US4] Test F122-AC-27 (MSW `// MOCK (test-only)`): "Chờ phê duyệt", danh sách chờ cho `sa.admin2`, người đề xuất không thấy nút Phê duyệt; map `MODERATION_SWITCH_SELF_APPROVAL`, `MODERATION_SWITCH_EXPIRY_REQUIRED`, `MODERATION_SWITCH_STEP_NOT_ALLOWED`; PII/grooming hiển thị "Luôn bật" — file: `…/chat-safety/__tests__/SwitchApproval.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 4

- [x] T025 [US4] Hook `switches.ts`; `SwitchesPage`, `SwitchConfirmModal`, `SwitchHistoryPage` — file: `src/workspaces/platform-admin/chat-safety/switches/` — `SwitchesScreen.tsx`
- [x] T052 [US4] `PendingSwitchRequests`, `ApproveSwitchDialog`; bỏ công tắc `pii` khỏi `SwitchesPage`, `expires_at` bắt buộc — file: `src/cms/frontend/apps/portal/src/workspaces/platform/chat-safety/PendingSwitchRequests.tsx` — trong `SwitchesScreen.tsx`; nút Từ chối thêm ở T060 (2026-11-24)

## Phase 7: User Story 5 — Banner (Priority: P1)

### Tests cho User Story 5

- [x] T026 [P] [US5] Test F122-AC-13: banner theo `getTenantModerationStatus` (tenant) và `listModerationSwitches` (platform), poll ≤ 60 s, mất khi hết hạn (TC-CHAT-016) — file: `src/shell/banners/__tests__/ModerationDisabledBanner.test.tsx` — phải FAIL trước khi code
- [x] T027 [P] [US5] Test F122-AC-14: tenant khác không thấy; không hiển thị lý do (TC-TEN-001) — file: `tests/e2e/chat-safety/banner-isolation.spec.ts` — vitest whole-portal (`ModerationDisabledBanner.test.tsx`) thay Playwright

### Implementation cho User Story 5

- [x] T028 [US5] `useModerationStatus`, `ModerationDisabledBanner` đăng ký `BannerSlot` — file: `src/shared/chatSafety/useModerationStatus.ts`, `src/shell/banners/ModerationDisabledBanner.tsx` — `src/shared/chatSafety/useModerationStatus.ts`, `src/shell/banners/ModerationDisabledBanner.tsx`, badge `SafetyNotificationsBadge.tsx`

## Phase 8: User Story 6 — Từ bổ sung, nhóm từ tenant (Priority: P2)

### Tests cho User Story 6

- [x] T029 [P] [US6] Test F122-AC-15: CRUD term tenant đích, thử chuỗi có `tenant_id`, publish cần mã yêu cầu (TC-CHAT-015) — file: `src/workspaces/live-ops/moderation/__tests__/TenantTermsPage.test.tsx` — phải FAIL trước khi code
- [x] T030 [P] [US6] Test F122-AC-16: công tắc khóa khi `allow_tenant_toggle=false`, map `TENANT_TOGGLE_NOT_ALLOWED` — file: `…/__tests__/TenantCategoriesPage.test.tsx`
- [x] T031 [P] [US6] Test F122-AC-17: C chỉ đọc, không nội dung term, không nút ghi — file: `src/workspaces/school-lms/safety/__tests__/CategoriesPage.test.tsx` — trong `SchoolSafety.test.tsx`

### Implementation cho User Story 6

- [x] T032 [US6] Hook `tenantTerms.ts`; `TenantTermsPage`, `TenantCategoriesPage`; C `CategoriesPage` — file: `src/workspaces/live-ops/moderation/tenant/`, `src/workspaces/school-lms/safety/CategoriesPage.tsx` — `TenantTermsScreen.tsx`; nhóm từ chỉ xem ở `school-lms/safety/ChatSafetyScreen.tsx`

## Phase 9: User Story 7 — Thông báo an toàn của trường (Priority: P1)

### Tests cho User Story 7

- [x] T033 [P] [US7] Test F122-AC-18: danh sách + chi tiết không có vùng chat, chi tiết hiện khối (không lớp), badge chờ xác nhận (TC-CHAT-043) — file: `src/workspaces/school-lms/safety/__tests__/NotificationsPage.test.tsx` — phải FAIL trước khi code — trong `SchoolSafety.test.tsx`
- [x] T034 [P] [US7] Test F122-AC-19: xác nhận cần `contact_outcome`, ghi chú ≤ 500, `Idempotency-Key`, `409` → "đã được xác nhận" — file: `…/__tests__/NotificationAck.test.tsx` — trong `SchoolSafety.test.tsx`
- [x] T035 [P] [US7] E2E F122-AC-20 viewport 390 px dạng thẻ + visual regression (TC-UI-004) — file: `tests/e2e/chat-safety/school-notifications-mobile.spec.ts` — vitest viewport 390 px (`setViewportWidth`); visual regression Playwright chưa chạy
- [x] T036 [P] [US7] E2E tenant isolation: `ad.truong.b` không thấy thông báo TH Demo A (TC-CHAT-043) — file: `tests/e2e/chat-safety/school-notifications-isolation.spec.ts` — vitest (`ad.truong.b`)

### Implementation cho User Story 7

- [x] T037 [US7] Hook `notifications.ts`; `NotificationsPage`, `NotificationDetail`, badge menu — file: `src/workspaces/school-lms/safety/` — `SafetyNotificationsScreen.tsx`

## Phase 10: User Story 8 — Chính sách chat trường (Priority: P2)

### Tests cho User Story 8

- [x] T038 [P] [US8] Test F122-AC-21: tải policy + effective (scope `school`), control theo `channel_type` (không có `class`/`study_group`, không có tên thật), giới hạn từ cấp trên, nhãn kế thừa (TC-CHAT-018) — file: `src/workspaces/school-lms/safety/__tests__/ChatPolicyPage.test.tsx` — phải FAIL trước khi code — trong `SchoolSafety.test.tsx`
- [x] T039 [P] [US8] Test F122-AC-22: `match`/`team`/`party` khóa; không control theo khối; `If-Match`, lý do; map lỗi về control (TC-CHAT-018) — file: `…/__tests__/ChatPolicySave.test.tsx` — trong `SchoolSafety.test.tsx`
- [x] T053 [P] [US8] Test F122-AC-28: chọn preset cả trường, không có lựa chọn theo khối, `reason` bắt buộc, lỗi theo `code` — file: `…/school-chat-policy/__tests__/ChatPresetSelector.test.tsx` — phải FAIL trước khi code — trong `SchoolSafety.test.tsx`

### Implementation cho User Story 8

- [x] T040 [US8] Hook `chatPolicy.ts`; `ChatPolicyPage`, `ChannelPolicyCard` (dùng `AlwaysOnLock`) — file: `src/workspaces/school-lms/safety/` — `ChatPolicyPanel.tsx` (dùng `AlwaysOnLock`)
- [x] T054 [US8] `ChatPresetSelector` trong màn Chính sách chat trường — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/safety/ChatPresetSelector.tsx` — chọn preset trong `ChatPolicyPanel.tsx`

## Phase 11: User Story 9 — Club của trường (Priority: P3)

### Tests cho User Story 9

- [x] T041 [P] [US9] Test F122-AC-23: `listClubs` trường mình, duyệt/lưu trữ/gỡ thành viên có `Idempotency-Key`, không màn chat (TC-SOC-008) — file: `src/workspaces/school-lms/safety/__tests__/ClubsPage.test.tsx` — phải FAIL trước khi code — trong `SchoolSafety.test.tsx`

### Implementation cho User Story 9

- [x] T042 [US9] Hook `clubs.ts`; `ClubsPage` — file: `src/workspaces/school-lms/safety/ClubsPage.tsx` — `ClubsScreen.tsx`; Thành viên + Gỡ thành viên thêm ở T061 (2026-11-24)

## Phase 12: User Story 10 — Bảo vệ màn an toàn chat (Priority: P1)

### Tests cho User Story 10

- [x] T043 [P] [US10] E2E F122-AC-24: ma trận tài khoản × URL (`sm.antoan` chỉ Kiểm duyệt; `ad.truong.a`, `qlcm.truong.a`, `cm.noidung` → 403, không tải chunk) (TC-CMS-WS-01, TC-CMS-AUTHZ-05 tương ứng ADMIN_PORTAL §12) — file: `tests/e2e/chat-safety/authz.spec.ts` — phải FAIL trước khi code — vitest whole-portal (sm.antoan, spy chunk loader) thay Playwright
- [x] T044 [P] [US10] Test F122-AC-25: step-up MFA quay lại URL; VPN notice (TC-CMS-WS-02) — file: `src/workspaces/live-ops/moderation/__tests__/errors.test.tsx` — bỏ step-up MFA (ADR-0094): 403 + trace_id, VPN notice
- [x] T045 [P] [US10] Test F122-AC-26: `maxLength` + bộ đếm mọi ô; `Idempotency-Key` giữ khi retry; lint không import component game (TC-TEN-024, TC-UI-010) — file: `src/workspaces/live-ops/moderation/__tests__/inputLimits.test.tsx` — `Cases.test.tsx` + `src/shared/chatSafety/limits.test.ts`

### Implementation cho User Story 10

- [x] T046 [US10] Nối `RequirePermission`, `MfaStepUp`, `VpnRequiredNotice` vào mọi route của feature — file: các `routes.tsx` ở T001 — `RequirePermission`/`MenuGate` của F051 + `SafetyError`; không có `MfaStepUp` (ADR-0094)

## Phase N: Polish & Cross-Cutting

- [ ] T047 [P] axe + visual regression sáng/tối các màn chính (TC-UI-004, TC-UI-006) — file: `tests/e2e/chat-safety/a11y-visual.spec.ts` — một phần 2026-11-24: smoke a11y (một h1, mọi control có tên) ở sáng + tối trong `tests/e2e/chat-safety/chat-safety.spec.ts`; axe-core đầy đủ và baseline visual chưa có (portal chưa khai báo `axe-playwright`; baseline cần backend thật) — **chưa chạy**
- [x] T048 [P] Ghi chú thống nhất MODERATION §11.3 theo ADR-0072 (workspace E, Safety Team bật/tắt nhóm từ) — file: `docs/03-social-multiplayer/MODERATION.md`
- [x] T049 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T050 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng — **chưa chạy** (2026-11-24): stack local của owner chạy mã `main` (không có T060–T066), không được khởi động lại; lệnh đọc thử gateway bị chặn quyền trong phiên; không có mật khẩu demo. Quickstart đã cập nhật kịch bản T060–T066
- [x] T055 [P] Test F122-AC-29: tắt `menu.live_ops.tenant_terms` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/safety/feature-toggle.spec.ts` — vitest (`SchoolSafety.test.tsx`) thay Playwright
- [x] T056 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F122-AC-29) — file: `src/cms/frontend/apps/portal/src/workspaces/live-ops/moderation/routes.tsx`

## Phase N+1: Việc chốt 2026-11-23 (Clarifications [Confirmed] — orchestrator)

- [x] T060 `rejectModerationSwitchChange` (module chat: contract, command, audit, event `moderation_switch.change_rejected`, test integration + authz) và nút Từ chối trên portal (F122-AC-27)
- [x] T061 Route nhân sự `listSchoolClubMembers` (keyset SQL) + `removeSchoolClubMember` (`social.club.approve`, lý do, audit, gỡ channel `team` cùng transaction) + nút Gỡ thành viên (F122-AC-23)
- [x] T062 Menu `menu.platform_admin.chat_safety` nhận `chat.sensitive_terms.read` hoặc `.write` (catalog bump, sync); test AC-09 với Safety Moderator thật
- [x] T063 `listSensitiveTerms` thêm `locale`, `match_type`, `status` lọc trong SQL; bật bộ lọc portal (F122-AC-07)
- [x] T064 `tenant_name` trong case + `listModerationCaseTenants` (keyset, `moderation.case.read`, `tenancy.Bypass(ctx, "moderation_case")`) cho bộ lọc tenant của Safety Team
- [x] T065 `listModerationSwitches` không lọc tenant trả `disabled_tenant_count` (đếm SQL); banner hiện số tenant đang tắt
- [x] T066 `listSchoolSafetyNotifications` thêm `created_from` lọc trong SQL; portal mặc định 30 ngày (F122-AC-18)

Bằng chứng T060–T066 (2026-11-24): integration `src/backend/modules/chat/tests/integration/f122_delta_test.go` — `TestSwitchReject`, `TestSwitchDisabledTenantCount`, `TestSensitiveTermFilters`, `TestModerationCaseTenants`, `TestSchoolNotificationsCreatedFrom`, `TestSchoolClubMembersStaff`, `TestSensitiveCategoryPages` (viết trước, chạy đỏ trước khi hiện thực: 6/6 FAIL trên mã chưa sửa — `TestSensitiveCategoryPages` viết cùng lúc với mã của yêu cầu phân trang owner). Portal: test mới trong `ChatSafety.test.tsx`, `Cases.test.tsx`, `ModerationDisabledBanner.test.tsx`, `SchoolSafety.test.tsx` (đỏ trước: 7 FAIL). Migration `20261231000007_f122_tenant_name.sql`. Thêm (owner 2026-11-23): `listSensitiveCategories`/`listTenantSensitiveCategories` trang keyset `(code, category_id)`.

## AC → test (bằng chứng, 2026-11-23)

Test vitest whole-portal với backend MOCK (`src/cms/frontend/apps/portal/src/test/chat-safety-mock.ts`, chỉ trong test — R17); backend `getTenantModerationStatus` có test integration PostgreSQL/Valkey/NATS. Đường dẫn test tương đối `src/cms/frontend/apps/portal/src/` trừ khi ghi rõ. Kết quả: 60/60 test F122 PASS (`vitest --maxWorkers=2` 7 file: 55 pass + 5 sửa test và chạy lại riêng: 5 pass; máy dùng chung bị tải cao nên một số test chạy 1–6 phút); `TestTenantModerationStatus` PASS. Test được viết cùng lúc với màn hình (không chạy đỏ trước trên placeholder) — sai lệch so với quy trình test-first, ghi nhận.

| AC | Test | Kết quả |
|---|---|---|
| F122-AC-01 | `workspaces/live-ops/moderation/__tests__/Cases.test.tsx` › "sm.antoan: only the Kiểm duyệt menus; queue with Tenant column, SLA overdue and cursor pages", "filters: module tenant filter …", "F122-AC-01 (T064): the tenant filter lists the tenants having cases …"; ops-ui `chatSafety.test.tsx` › SlaBadge; `src/backend/modules/chat/tests/integration/f122_delta_test.go` › `TestModerationCaseTenants` | PASS |
| F122-AC-02 | `Cases.test.tsx` › "evidence panel: snapshot, original, revisions, recall; search inside the case only; no free chat search" | PASS |
| F122-AC-03 | `Cases.test.tsx` › "act: note required, Idempotency-Key + If-Match + X-Tenant-Id of the case; account_lock needs a second confirmation; take the case" | PASS |
| F122-AC-04 | `Cases.test.tsx` › "F122-AC-04: 412 → reloaded message …; 409/403 by code with trace id; no automatic retry" | PASS |
| F122-AC-05 | `Cases.test.tsx` › "school of the case, only its students, summary 10–1000 …", "a case of the common tenant has no notify button …" | PASS |
| F122-AC-06 | `Cases.test.tsx` › "F122-AC-06: school notifications sent by the platform, overdue / acknowledged filters" | PASS |
| F122-AC-07 | `workspaces/platform-admin/chat-safety/__tests__/ChatSafety.test.tsx` › "categories: create, edit, soft delete …; terms with variants and RE2 warning; allowlist", "F122-AC-07: term filters locale / match type / status go to the server …", "owner rule 2026-11-23: the categories table pages with the cursor …"; `src/backend/modules/chat/tests/integration/f122_delta_test.go` › `TestSensitiveTermFilters`, `TestSensitiveCategoryPages` | PASS |
| F122-AC-08 | `ChatSafety.test.tsx` › "test text: decision, masked text, categories, highlight …; publish needs a summary; versions with regression"; ops-ui `chatSafety.test.tsx` › SensitiveTermTester, highlightParts | PASS |
| F122-AC-09 | `ChatSafety.test.tsx` › "F122-AC-09: without the edit/test/publish actions …", "F122-AC-09 (T062): a real Safety Moderator sees the catalogue read only; switches stay hidden; nothing stored" (principal = quyền role `safety_moderator` + workspaces suy ra từ catalog như identity) | PASS |
| F122-AC-10 | `ChatSafety.test.tsx` › "platform row + filtered tenants; PII and grooming "Luôn bật"; a step off at platform level is locked …" | PASS |
| F122-AC-11 | `ChatSafety.test.tsx` › "turning a step off: risk text, reason ≥ 10 …, expiry required, X-Tenant-Id; 422 reason code on the field; then "Chờ phê duyệt"" | PASS |
| F122-AC-12 | `ChatSafety.test.tsx` › "F122-AC-12: history with before/after, reason, expiry, actor, approver and source" | PASS |
| F122-AC-13 | `shell/banners/__tests__/ModerationDisabledBanner.test.tsx` › "tenant staff of TH Demo A …", "expired switch …", "System Admin: banner for the tenants of the filter …", "F122-AC-13 (T065): System Admin without a tenant filter — number of tenants off …"; `src/backend/modules/chat/tests/integration/f122_delta_test.go` › `TestSwitchDisabledTenantCount`; backend `src/backend/modules/chat/tests/integration/moderation_status_test.go` › `TestTenantModerationStatus` | PASS |
| F122-AC-14 | `ModerationDisabledBanner.test.tsx` › "F122-AC-14: a staff member of another tenant sees no banner"; `TestTenantModerationStatus` (không lý do, tenant B, cross-tenant 403/404) | PASS |
| F122-AC-15 | `workspaces/live-ops/moderation/__tests__/TenantTerms.test.tsx` › "target tenant: list, add, delete, test with tenant_id, publish with the request code" | PASS |
| F122-AC-16 | `TenantTerms.test.tsx` › "toggle only when allow_tenant_toggle …; TENANT_TOGGLE_NOT_ALLOWED by code" | PASS |
| F122-AC-17 | `workspaces/school-lms/safety/__tests__/SchoolSafety.test.tsx` › "F122-AC-17: tenant categories read only …" | PASS |
| F122-AC-18 | `workspaces/school-lms/safety/__tests__/SchoolSafety.test.tsx` › "list of the tenant (pending by default), detail with grade and no chat area, menu badge; acknowledge …", "F122-AC-18 (T066): the list asks the server for the last 30 days …", "tenant isolation: ad.truong.b …"; `src/backend/modules/chat/tests/integration/f122_delta_test.go` › `TestSchoolNotificationsCreatedFrom` | PASS |
| F122-AC-19 | `SchoolSafety.test.tsx` › (cùng test trên: outcome bắt buộc, ghi chú ≤ 500, Idempotency-Key) + "409 CONFLICT → "Thông báo đã được xác nhận" …" | PASS |
| F122-AC-20 | `SchoolSafety.test.tsx` › "F122-AC-20: 390 px → cards and a detail page with the confirm button" | PASS |
| F122-AC-21 | `SchoolSafety.test.tsx` › "per channel type with limits from above and "Kế thừa"; match/team/party locked; no class/grade/real-name control" | PASS |
| F122-AC-22 | `SchoolSafety.test.tsx` › "save: If-Match "0", reason required, preset …; RELAX error lands on the channel card", "F122-AC-22: CHAT_POLICY_ALWAYS_ON and VALIDATION_FAILED by code" | PASS |
| F122-AC-23 | `SchoolSafety.test.tsx` › "clubs of the school waiting for approval; approve / archive …; no chat screen", "members of a club (cursor pages, nicknames only) and "Gỡ thành viên" …", "club approve action off → … no "Gỡ thành viên""; `src/backend/modules/chat/tests/integration/f122_delta_test.go` › `TestSchoolClubMembersStaff` | PASS |
| F122-AC-24 | `Cases.test.tsx` › "sm.antoan: only the Kiểm duyệt menus …", "F122-AC-24: sm.antoan opening a C URL gets the 403 page without the chunk"; `ChatSafety.test.tsx` › "F122-AC-24 (T062): the Safety Moderator still gets the 403 page on the switches URL" | PASS |
| F122-AC-25 | `Cases.test.tsx` › "403 PERMISSION_DENIED → 403 page with the trace id (no MFA step-up); internal gateway down → VPN notice" | PASS |
| F122-AC-26 | `Cases.test.tsx` › "F122-AC-26: the same intent retried keeps its Idempotency-Key …" + kiểm `maxlength` ở các test trên; `shared/chatSafety/limits.test.ts` (giới hạn = OpenAPI); lint `no-game-imports` | PASS |
| F122-AC-27 | `ChatSafety.test.tsx` › "F122-AC-27: pending request — no Approve button for the proposer …", "F122-AC-27 (T060): another System Admin rejects with a reason …", "F122-AC-27: EXPIRY_REQUIRED and STEP_NOT_ALLOWED are shown by code"; `src/backend/modules/chat/tests/integration/f122_delta_test.go` › `TestSwitchReject` | PASS |
| F122-AC-28 | `SchoolSafety.test.tsx` › "save: If-Match "0", reason required, preset for the whole school …" | PASS |
| F122-AC-29 | `SchoolSafety.test.tsx` › "chat-safety menu off → hidden …; club approve action off → no button", "403 FEATURE_DISABLED from a chat-safety API → notice and permissions reloaded"; `ChatSafety.test.tsx` › AC-09 (action tắt) | PASS |

Toàn bộ suite web (128 file) chưa chạy hết: máy dùng chung tải 100% CPU (6 file trong ~2 giờ, đã dừng). Đã chạy riêng các file chung bị chạm: `catalogMenus`, `errors`, `keys`, `problem` PASS; `wizard.test.tsx` (F055) lỗi khi chạy cùng suite, chạy riêng 11/11 PASS; `shell/layout.test.tsx` lỗi timeout 10 s ở lần tải chunk đầu — trên bản không có thay đổi F122 (HEAD) cùng file lỗi 4/8 do tải máy, có thay đổi F122 lỗi 1–2/8 → không do F122.

## Dependencies & Execution Order

- Setup → Foundational (T004–T008) → US10 song song US1 → US2 → US3, US4 → US5 (cần T005) → US7 → US6, US8 (cần delta F120), US9 → Polish.

## Definition of Done

- [x] Mọi tiêu chí F122-AC-* có test pass (ghi mã test/TC) — bảng AC → test ở trên (2026-11-24)
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật; `make test-integration MOD=chat` cho T005 — lint/typecheck/gen:check xanh, integration chat xanh, vitest 42 file chat-safety/shell/shared/app/permissions/platform-admin xanh; **Playwright với backend thật chưa chạy** (T003/T047/T050)
- [x] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
