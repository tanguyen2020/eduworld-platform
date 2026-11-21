# Tasks: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `src/cms/frontend/apps/portal/` trừ khi ghi rõ. MSW chỉ trong test, nhãn `// MOCK (test-only)` (R17). Fixture danh mục chỉ placeholder (R15).

## Phase 1: Setup

- [ ] T001 Route lazy `/live-ops/moderation/*`, `/platform-admin/chat-safety/*`, `/school-lms/safety/*` có `RequirePermission`; menu Safety Moderator chỉ Kiểm duyệt (khóa `menu.live_ops.*`, `menu.platform_admin.chat_safety`, `menu.platform_admin.moderation_switches`, `menu.school_lms.safety_notifications`, `menu.school_lms.chat_safety` từ registry sinh từ catalog, F051) — file: `src/workspaces/live-ops/moderation/routes.tsx`, `src/workspaces/platform-admin/chat-safety/routes.tsx`, `src/workspaces/school-lms/safety/routes.tsx`
- [ ] T002 [P] File dịch vi/en (thông điệp theo `code`: `MODERATION_CASE_CLOSED`, `MODERATION_SWITCH_REASON_REQUIRED`, `TENANT_TOGGLE_NOT_ALLOWED`, `CHAT_POLICY_RELAX_NOT_ALLOWED`, `CHAT_POLICY_ALWAYS_ON`, `CONFLICT`, `MFA_REQUIRED`) — file: `src/workspaces/*/{moderation,chat-safety,safety}/i18n/{vi,en}.json`
- [ ] T003 [P] Fixture Playwright: tài khoản `sm.antoan`, `sa.admin`, `ad.truong.a`, `ad.truong.b`, `ad.donvi` — file: `tests/e2e/fixtures/auth.ts`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract `getTenantModerationStatus` + schema `TenantModerationStatus` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 Hiện thực backend `getTenantModerationStatus` (query đọc công tắc hiệu lực, không trả lý do) + test authz/tenant isolation — file: `src/backend/modules/chat/internal/application/query/moderation/tenant_moderation_status.go`, `src/backend/modules/chat/internal/presentation/http/public/moderation_status.go`, `src/backend/modules/chat/tests/integration/moderation_status_test.go` — kiểm: `make test-integration MOD=chat`
- [ ] T006 Sinh lại client TS sau delta T004 và delta F120/F121 — file: `src/api/schema.d.ts` — kiểm: `pnpm gen:api` không diff trong CI
- [ ] T007 [P] `ops-ui`: `SensitiveTermTester`, `SlaBadge`, `AlwaysOnLock` + story sáng/tối — file: `src/cms/frontend/packages/ops-ui/src/chat-safety/`
- [ ] T008 [P] MSW handler test-only cho mọi operationId ở contracts §1 — file: `src/workspaces/live-ops/moderation/__tests__/msw/handlers.ts`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Hàng đợi case (Priority: P1) 🎯 MVP

**Goal**: Safety Team xử lý case liên tenant.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Test F122-AC-01: bộ lọc tenant (chỉ module Kiểm duyệt), mức, trạng thái, assignee, category, cột Tenant, `SlaBadge` quá hạn (TC-CHAT-042) — file: `src/workspaces/live-ops/moderation/__tests__/CaseQueuePage.test.tsx` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Test F122-AC-02: panel bằng chứng gốc/revision, tìm `q` trong case, DOM không có ô tìm chat tự do (TC-CHAT-040) — file: `…/__tests__/CaseDetailPage.test.tsx`
- [ ] T011 [P] [US1] Test F122-AC-03: assign/act/sanction gửi `Idempotency-Key`, `If-Match`, `X-Tenant-Id` tenant case; lý do bắt buộc; `account_lock` xác nhận 2 bước (TC-CHAT-007) — file: `…/__tests__/CaseActions.test.tsx`
- [ ] T012 [P] [US1] Test F122-AC-04: `412` → tải lại; `409`, `403` → thông điệp theo `code` + `trace_id`, không retry (TC-CMS-WS-02) — file: `…/__tests__/CaseErrors.test.tsx`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Hook `cases.ts`, `sanctions.ts` — file: `src/workspaces/live-ops/moderation/api/`
- [ ] T014 [US1] `CaseQueuePage`, `CaseDetailPage`, `EvidencePanel`, `CaseActionBar`, `SanctionForm` — file: `src/workspaces/live-ops/moderation/cases/`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Thông báo nhà trường (E) (Priority: P1)

### Tests cho User Story 2

- [ ] T015 [P] [US2] Test F122-AC-05: chọn trường trong `school_ids`, học sinh của trường đó ≤ 20, tóm tắt 10–1000, ẩn nút với tenant `common` (TC-CHAT-043) — file: `…/__tests__/NotifySchoolDrawer.test.tsx` — phải FAIL trước khi code
- [ ] T016 [P] [US2] Test F122-AC-06: danh sách platform lọc `overdue`/`acknowledged` — file: `…/__tests__/PlatformNotificationsPage.test.tsx`

### Implementation cho User Story 2

- [ ] T017 [US2] Hook `schoolNotifications.ts`; `NotifySchoolDrawer`, `PlatformNotificationsPage` — file: `src/workspaces/live-ops/moderation/api/schoolNotifications.ts`, `cases/NotifySchoolDrawer.tsx`, `notifications/PlatformNotificationsPage.tsx`

## Phase 5: User Story 3 — Danh mục từ nhạy cảm (Priority: P1)

### Tests cho User Story 3

- [ ] T018 [P] [US3] Test F122-AC-07: CRUD category/term/allowlist theo schema, xóa mềm có xác nhận (TC-CHAT-015) — file: `src/workspaces/platform-admin/chat-safety/__tests__/Catalog.test.tsx` — phải FAIL trước khi code
- [ ] T019 [P] [US3] Test F122-AC-08: thử chuỗi hiển thị quyết định/`masked_text`; publish cần `change_summary`; versions + hồi quy (TC-CHAT-012) — file: `…/__tests__/PublishAndTest.test.tsx`
- [ ] T020 [P] [US3] Test F122-AC-09: Safety Moderator chỉ đọc; không có nội dung term trong storage/log (spy console, storage) — file: `…/__tests__/CatalogReadOnly.test.tsx`

### Implementation cho User Story 3

- [ ] T021 [US3] Hook `catalog.ts`; `CategoriesPage`, `TermsPage`, `AllowlistPage`, `PublishDrawer`, `VersionsPage` — file: `src/workspaces/platform-admin/chat-safety/`

## Phase 6: User Story 4 — Công tắc kiểm duyệt (Priority: P1)

### Tests cho User Story 4

- [ ] T022 [P] [US4] Test F122-AC-10: bảng Platform + tenant, khóa bước do cấp trên (TC-CHAT-013) — file: `…/chat-safety/__tests__/SwitchesPage.test.tsx` — phải FAIL trước khi code
- [ ] T023 [P] [US4] Test F122-AC-11: hộp xác nhận rủi ro, lý do ≥ 10, `expires_at`, `X-Tenant-Id` tenant đích, map `MODERATION_SWITCH_REASON_REQUIRED` (TC-CHAT-015) — file: `…/__tests__/SwitchConfirmModal.test.tsx`
- [ ] T024 [P] [US4] Test F122-AC-12: lịch sử before/after/`source` (TC-CHAT-016) — file: `…/__tests__/SwitchHistoryPage.test.tsx`
- [ ] T051 [P] [US4] Test F122-AC-27 (MSW `// MOCK (test-only)`): "Chờ phê duyệt", danh sách chờ cho `sa.admin2`, người đề xuất không thấy nút Phê duyệt; map `MODERATION_SWITCH_SELF_APPROVAL`, `MODERATION_SWITCH_EXPIRY_REQUIRED`, `MODERATION_SWITCH_STEP_NOT_ALLOWED`; PII/grooming hiển thị "Luôn bật" — file: `…/chat-safety/__tests__/SwitchApproval.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T025 [US4] Hook `switches.ts`; `SwitchesPage`, `SwitchConfirmModal`, `SwitchHistoryPage` — file: `src/workspaces/platform-admin/chat-safety/switches/`
- [ ] T052 [US4] `PendingSwitchRequests`, `ApproveSwitchDialog`; bỏ công tắc `pii` khỏi `SwitchesPage`, `expires_at` bắt buộc — file: `src/cms/frontend/apps/portal/src/workspaces/platform/chat-safety/PendingSwitchRequests.tsx`

## Phase 7: User Story 5 — Banner (Priority: P1)

### Tests cho User Story 5

- [ ] T026 [P] [US5] Test F122-AC-13: banner theo `getTenantModerationStatus` (tenant) và `listModerationSwitches` (platform), poll ≤ 60 s, mất khi hết hạn (TC-CHAT-016) — file: `src/shell/banners/__tests__/ModerationDisabledBanner.test.tsx` — phải FAIL trước khi code
- [ ] T027 [P] [US5] Test F122-AC-14: tenant khác không thấy; không hiển thị lý do (TC-TEN-001) — file: `tests/e2e/chat-safety/banner-isolation.spec.ts`

### Implementation cho User Story 5

- [ ] T028 [US5] `useModerationStatus`, `ModerationDisabledBanner` đăng ký `BannerSlot` — file: `src/shared/chatSafety/useModerationStatus.ts`, `src/shell/banners/ModerationDisabledBanner.tsx`

## Phase 8: User Story 6 — Từ bổ sung, nhóm từ tenant (Priority: P2)

### Tests cho User Story 6

- [ ] T029 [P] [US6] Test F122-AC-15: CRUD term tenant đích, thử chuỗi có `tenant_id`, publish cần mã yêu cầu (TC-CHAT-015) — file: `src/workspaces/live-ops/moderation/__tests__/TenantTermsPage.test.tsx` — phải FAIL trước khi code
- [ ] T030 [P] [US6] Test F122-AC-16: công tắc khóa khi `allow_tenant_toggle=false`, map `TENANT_TOGGLE_NOT_ALLOWED` — file: `…/__tests__/TenantCategoriesPage.test.tsx`
- [ ] T031 [P] [US6] Test F122-AC-17: C chỉ đọc, không nội dung term, không nút ghi — file: `src/workspaces/school-lms/safety/__tests__/CategoriesPage.test.tsx`

### Implementation cho User Story 6

- [ ] T032 [US6] Hook `tenantTerms.ts`; `TenantTermsPage`, `TenantCategoriesPage`; C `CategoriesPage` — file: `src/workspaces/live-ops/moderation/tenant/`, `src/workspaces/school-lms/safety/CategoriesPage.tsx`

## Phase 9: User Story 7 — Thông báo an toàn của trường (Priority: P1)

### Tests cho User Story 7

- [ ] T033 [P] [US7] Test F122-AC-18: danh sách + chi tiết không có vùng chat, chi tiết hiện khối (không lớp), badge chờ xác nhận (TC-CHAT-043) — file: `src/workspaces/school-lms/safety/__tests__/NotificationsPage.test.tsx` — phải FAIL trước khi code
- [ ] T034 [P] [US7] Test F122-AC-19: xác nhận cần `contact_outcome`, ghi chú ≤ 500, `Idempotency-Key`, `409` → "đã được xác nhận" — file: `…/__tests__/NotificationAck.test.tsx`
- [ ] T035 [P] [US7] E2E F122-AC-20 viewport 390 px dạng thẻ + visual regression (TC-UI-004) — file: `tests/e2e/chat-safety/school-notifications-mobile.spec.ts`
- [ ] T036 [P] [US7] E2E tenant isolation: `ad.truong.b` không thấy thông báo TH Demo A (TC-CHAT-043) — file: `tests/e2e/chat-safety/school-notifications-isolation.spec.ts`

### Implementation cho User Story 7

- [ ] T037 [US7] Hook `notifications.ts`; `NotificationsPage`, `NotificationDetail`, badge menu — file: `src/workspaces/school-lms/safety/`

## Phase 10: User Story 8 — Chính sách chat trường (Priority: P2)

### Tests cho User Story 8

- [ ] T038 [P] [US8] Test F122-AC-21: tải policy + effective (scope `school`), control theo `channel_type` (không có `class`/`study_group`, không có tên thật), giới hạn từ cấp trên, nhãn kế thừa (TC-CHAT-018) — file: `src/workspaces/school-lms/safety/__tests__/ChatPolicyPage.test.tsx` — phải FAIL trước khi code
- [ ] T039 [P] [US8] Test F122-AC-22: `match`/`team`/`party` khóa; không control theo khối; `If-Match`, lý do; map lỗi về control (TC-CHAT-018) — file: `…/__tests__/ChatPolicySave.test.tsx`
- [ ] T053 [P] [US8] Test F122-AC-28: chọn preset cả trường, không có lựa chọn theo khối, `reason` bắt buộc, lỗi theo `code` — file: `…/school-chat-policy/__tests__/ChatPresetSelector.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 8

- [ ] T040 [US8] Hook `chatPolicy.ts`; `ChatPolicyPage`, `ChannelPolicyCard` (dùng `AlwaysOnLock`) — file: `src/workspaces/school-lms/safety/`
- [ ] T054 [US8] `ChatPresetSelector` trong màn Chính sách chat trường — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/safety/ChatPresetSelector.tsx`

## Phase 11: User Story 9 — Club của trường (Priority: P3)

### Tests cho User Story 9

- [ ] T041 [P] [US9] Test F122-AC-23: `listClubs` trường mình, duyệt/lưu trữ/gỡ thành viên có `Idempotency-Key`, không màn chat (TC-SOC-008) — file: `src/workspaces/school-lms/safety/__tests__/ClubsPage.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 9

- [ ] T042 [US9] Hook `clubs.ts`; `ClubsPage` — file: `src/workspaces/school-lms/safety/ClubsPage.tsx`

## Phase 12: User Story 10 — Bảo vệ màn an toàn chat (Priority: P1)

### Tests cho User Story 10

- [ ] T043 [P] [US10] E2E F122-AC-24: ma trận tài khoản × URL (`sm.antoan` chỉ Kiểm duyệt; `ad.truong.a`, `qlcm.truong.a`, `cm.noidung` → 403, không tải chunk) (TC-CMS-WS-01, TC-CMS-AUTHZ-05 tương ứng ADMIN_PORTAL §12) — file: `tests/e2e/chat-safety/authz.spec.ts` — phải FAIL trước khi code
- [ ] T044 [P] [US10] Test F122-AC-25: step-up MFA quay lại URL; VPN notice (TC-CMS-WS-02) — file: `src/workspaces/live-ops/moderation/__tests__/errors.test.tsx`
- [ ] T045 [P] [US10] Test F122-AC-26: `maxLength` + bộ đếm mọi ô; `Idempotency-Key` giữ khi retry; lint không import component game (TC-TEN-024, TC-UI-010) — file: `src/workspaces/live-ops/moderation/__tests__/inputLimits.test.tsx`

### Implementation cho User Story 10

- [ ] T046 [US10] Nối `RequirePermission`, `MfaStepUp`, `VpnRequiredNotice` vào mọi route của feature — file: các `routes.tsx` ở T001

## Phase N: Polish & Cross-Cutting

- [ ] T047 [P] axe + visual regression sáng/tối các màn chính (TC-UI-004, TC-UI-006) — file: `tests/e2e/chat-safety/a11y-visual.spec.ts`
- [ ] T048 [P] Ghi chú thống nhất MODERATION §11.3 theo ADR-0072 (workspace E, Safety Team bật/tắt nhóm từ) — file: `docs/03-social-multiplayer/MODERATION.md`
- [ ] T049 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T050 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [ ] T055 [P] Test F122-AC-29: tắt `menu.live_ops.tenant_terms` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions`; mục được bảo vệ không có công tắc (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/safety/feature-toggle.spec.ts`
- [ ] T056 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F122-AC-29) — file: `src/cms/frontend/apps/portal/src/workspaces/live-ops/moderation/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004–T008) → US10 song song US1 → US2 → US3, US4 → US5 (cần T005) → US7 → US6, US8 (cần delta F120), US9 → Polish.

## Definition of Done

- [ ] Mọi tiêu chí F122-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật; `make test-integration MOD=chat` cho T005
- [ ] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
