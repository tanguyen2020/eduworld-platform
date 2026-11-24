# Tasks: F079 — Workspace A — Game Operations Console UI (world, NPC, quest builder, reward, item, event, season)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `src/cms/frontend/apps/portal/` trừ khi ghi rõ. MSW chỉ trong test, nhãn `// MOCK (test-only)` (R17); E2E chạy với backend local thật.

## Phase 1: Setup

- [x] T001 Route lazy `/game-ops/*` + `RequirePermission` (bất kỳ `game.*`), menu từ `registry/configTypes.ts` — file: `src/workspaces/game-ops/routes.tsx`, `src/workspaces/game-ops/registry/configTypes.ts`
- [x] T002 [P] File dịch vi/en (`CONTENT_VERSION_CONFLICT`, `VALIDATION_FAILED`, `TRANSLATION_INCOMPLETE`, `MFA_REQUIRED`, trạng thái workflow) — file: `src/workspaces/game-ops/i18n/{vi,en}.json`
- [ ] T003 [P] Fixture Playwright tài khoản platform game + tenant — file: `tests/e2e/fixtures/auth.ts`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract (owner F078): `GameConfigVersion.media`, `listGameConfigs` thêm `q`/`from`/`to`/`sort`, `getGameClientManifest`, `getGameConfigSchema` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [x] T005 [P] Delta contract (owner F074/F077): `simulateGameConfig` + `SimulationRequest`/`SimulationResult` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [x] T006 [P] Kiểm permission `progression-curves`/`progression-levels` (`game.progression.write`), `collections` (`game.collection.write`), `shop-items` (`game.shop.write`) khớp giữa API_DESIGN §9.13b, ADMIN_PORTAL §2.2 và `registry/configTypes.ts` (RB-309) — file: `docs/07-backend/API_DESIGN.md`, `docs/04-admin-portal/ADMIN_PORTAL.md`
- [x] T007 Sinh lại client TS — file: `src/api/schema.d.ts` — kiểm: `pnpm gen:api` không diff trong CI
- [x] T008 [P] `ops-ui` `ContentEditorShell`, `SchemaForm` (rule từ JSON Schema + `maxLength` + bộ đếm), `WorkflowBar`, `VersionTab`, `DiffView`, `useAutosave` + story — file: `src/cms/frontend/packages/ops-ui/src/content-editor/`
- [x] T009 [P] `ops-ui` `GamePreviewFrame`, `ThumbnailImage`, `TurntableViewer`, `ManifestRefPicker` + story — file: `src/cms/frontend/packages/ops-ui/src/game/`
- [x] T010 [P] MSW handler test-only cho contracts §1 — file: `src/workspaces/game-ops/__tests__/msw/handlers.ts`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Danh sách và editor chung (Priority: P1) 🎯 MVP

**Goal**: Vòng đời version cho mọi loại cấu hình game.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T011 [P] [US1] Test F079-AC-01: danh sách theo `config_type`, lọc, cursor, menu theo permission (TC-CMS-001) — file: `src/workspaces/game-ops/__tests__/ConfigListPage.test.tsx` — phải FAIL trước khi code
- [x] T012 [P] [US1] Test F079-AC-02: form từ schema, autosave `If-Match`, dừng khi 409/412 (TC-CMS-002) — file: `src/cms/frontend/packages/ops-ui/src/content-editor/__tests__/ContentEditorShell.test.tsx`
- [x] T013 [P] [US1] Test F079-AC-03: transition + publish ngay/hẹn giờ có `Idempotency-Key`, ẩn duyệt với tác giả (TC-CMS-001) — file: `…/content-editor/__tests__/WorkflowBar.test.tsx`
- [x] T014 [P] [US1] Test F079-AC-04: version publish chỉ đọc, diff, releases, rollback/unpublish bắt buộc lý do (TC-CMS-003) — file: `…/content-editor/__tests__/VersionTab.test.tsx`
- [x] T015 [P] [US1] Test F079-AC-05: map `VALIDATION_FAILED`/`TRANSLATION_INCOMPLETE` về field, chặn gửi duyệt (TC-EVT-010) — file: `…/content-editor/__tests__/ServerErrors.test.tsx`

### Implementation cho User Story 1

- [x] T016 [P] [US1] Hook `gameConfigs.ts`, `schemas.ts` — file: `src/workspaces/game-ops/api/`
- [x] T017 [US1] `ConfigListPage`, `GenericSchemaBuilder` nối `ContentEditorShell` — file: `src/workspaces/game-ops/list/ConfigListPage.tsx`, `builders/generic/GenericSchemaBuilder.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Builder thế giới, NPC, quest, phần thưởng, item (Priority: P1)

### Tests cho User Story 2

- [x] T018 [P] [US2] Test F079-AC-06: bước quest kéo-thả đúng `quest.schema.json`; graph NPC đúng `npc-dialogue.schema.json` (TC-GAM-012) — file: `src/workspaces/game-ops/__tests__/QuestStepEditor.test.tsx`, `DialogueGraphEditor.test.tsx` — phải FAIL trước khi code
- [x] T019 [P] [US2] Test F079-AC-07: bộ chọn tham chiếu chỉ từ manifest/nội dung publish, không nhập ID tự do — file: `src/cms/frontend/packages/ops-ui/src/game/__tests__/ManifestRefPicker.test.tsx`
- [x] T020 [P] [US2] Test F079-AC-08: form loại còn lại theo schema; không trường tiền thật (TC-ECO-003) — file: `src/workspaces/game-ops/__tests__/GenericBuilders.test.tsx`

### Implementation cho User Story 2

- [x] T021 [P] [US2] Hook `manifest.ts`; `QuestStepEditor`, `DialogueGraphEditor` — file: `src/workspaces/game-ops/api/manifest.ts`, `builders/quest/`, `builders/npc/`
- [x] T022 [US2] Đăng ký builder cho `reward-rules`, `items`, `shop-items`, `currencies`, `world-areas`, `story-chapters`, `minigame-configs`, `achievements`, `challenges`, `bosses` — file: `src/workspaces/game-ops/registry/configTypes.ts`

## Phase 5: User Story 3 — Level & XP (Priority: P1)

### Tests cho User Story 3

- [x] T023 [P] [US3] Test F079-AC-09: công thức/bảng, danh hiệu, nguồn XP + cap, biểu đồ từ server (TC-GAM-006) — file: `src/workspaces/game-ops/__tests__/ProgressionCurveEditor.test.tsx` — phải FAIL trước khi code
- [x] T024 [P] [US3] Test F079-AC-10: mô phỏng gọi `simulateGameConfig`, không tính phía client (spy) — file: `…/__tests__/SimulationPanel.test.tsx`
- [x] T025 [P] [US3] Test F079-AC-11: lỗi ngưỡng/cap/`effective_from`, hộp xác nhận "không giảm level" — file: `…/__tests__/ProgressionPublish.test.tsx`

### Implementation cho User Story 3

- [x] T026 [US3] Hook `simulate.ts`; `ProgressionCurveEditor`, `SimulationPanel` — file: `src/workspaces/game-ops/api/simulate.ts`, `builders/progression/`

## Phase 6: User Story 4 — Event Builder, lịch, season, template (Priority: P1)

### Tests cho User Story 4

- [x] T027 [P] [US4] Test F079-AC-12: wizard 8 bước, song ngữ có chỉ báo, tham chiếu chỉ nội dung publish (TC-EVT-010) — file: `src/workspaces/game-ops/__tests__/EventWizard.test.tsx` — phải FAIL trước khi code
- [x] T028 [P] [US4] Test F079-AC-13: chặn theo lỗi server; cảnh báo > 3 phiên/tuần không chặn — file: `…/__tests__/EventWizardValidation.test.tsx`
- [x] T029 [P] [US4] Test F079-AC-14: lịch ICT, màu theo token, cảnh báo chồng chéo từ server — file: `…/__tests__/EventCalendarPage.test.tsx`
- [x] T030 [P] [US4] Test F079-AC-15: season track không control `paid_track`, mô phỏng; template `allowed_params` (TC-EVT-007) — file: `…/__tests__/SeasonTrackEditor.test.tsx`, `EventTemplateEditor.test.tsx`

### Implementation cho User Story 4

- [x] T031 [US4] `EventWizard` + `steps/*`, `EventCalendarPage`, `EventTemplateEditor`, `SeasonTrackEditor` — file: `src/workspaces/game-ops/builders/event/`, `builders/season/`

## Phase 7: User Story 5 — Thumbnail và preview (Priority: P2)

### Tests cho User Story 5

- [x] T032 [P] [US5] Test F079-AC-16: thumbnail đúng version, placeholder khi `pending`/`failed` (TC-UI-009) — file: `src/cms/frontend/packages/ops-ui/src/game/__tests__/ThumbnailImage.test.tsx` — phải FAIL trước khi code
- [x] T033 [P] [US5] Test F079-AC-17: `createGamePreviewSession` + iframe `GamePreviewFrame`; lint không import game ngoài khung (TC-UI-010) — file: `src/cms/frontend/packages/ops-ui/src/game/__tests__/GamePreviewFrame.test.tsx`

### Implementation cho User Story 5

- [x] T034 [US5] Hook `preview.ts`; gắn thumbnail vào danh sách, bộ chọn item/phần thưởng, chi tiết — file: `src/workspaces/game-ops/api/preview.ts`, `list/ConfigListPage.tsx`

## Phase 8: User Story 6 — Bảo vệ workspace A (Priority: P1)

### Tests cho User Story 6

- [ ] T035 [P] [US6] E2E F079-AC-18: `ad.truong.a`, `qlcm.truong.a`, `sm.antoan` → 403, không tải chunk (TC-CMS-AUTHZ-05, TC-CMS-WS-01) — file: `tests/e2e/game-ops/authz.spec.ts` — phải FAIL trước khi code
- [x] T036 [P] [US6] Test F079-AC-19: step-up MFA, VPN notice (TC-CMS-WS-02) — file: `src/workspaces/game-ops/__tests__/errors.test.tsx`
- [x] T037 [P] [US6] Test F079-AC-20: `Idempotency-Key` giữ khi retry, `maxLength` + bộ đếm, lỗi `code` + `trace_id` (TC-TEN-024) — file: `src/workspaces/game-ops/__tests__/idempotencyAndLimits.test.tsx`

### Implementation cho User Story 6

- [x] T038 [US6] Nối `MfaStepUp`, `VpnRequiredNotice`, `LargeScreenRequired` (F051) vào mọi route — file: `src/workspaces/game-ops/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T039 [P] axe + visual regression sáng/tối danh sách, editor, Event Builder (TC-UI-004, TC-UI-006) — file: `tests/e2e/game-ops/a11y-visual.spec.ts`
- [x] T040 [P] Lint hardcode style, không import game (TC-UI-003, TC-UI-010) — kiểm: `make web-lint`
- [x] T041 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T042 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [x] T043 [P] Test F079-AC-21: tắt `menu.game_ops.shop` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/game-ops/feature-toggle.spec.ts`
- [x] T044 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F079-AC-21) — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/routes.tsx`

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T045 **OQ chốt 2026-11-20** Game Dashboard workspace A (đếm cấu hình theo trạng thái + item cần dịch), menu theo catalog — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/dashboard/` — kiểm: `make web-test` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational (T004–T010) → US1 → US6 → US2 → US3, US4 (song song; cần T005) → US5 (cần T004 `media`) → Polish.
- F108 dùng lại T008 sau khi US1 xong.

## Definition of Done

- [ ] Mọi tiêu chí F079-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật qua gateway nội bộ
- [ ] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Kết quả triển khai (2026-11-24)

Quyết định ở [spec.md](spec.md) Clarifications "Session 2026-11-24 (triển khai)". T010: MOCK test-only là `src/test/game-ops-mock.ts` (override của MockPortalBackend dùng chung, nhãn `MOCK (test only)`) thay cho MSW. T038: không còn step-up MFA (ADR-0094); VPN do shell (`vpn_required`) + `GameOpsError`; builder < 1024 px chỉ đọc kèm thông báo màn hình lớn. T035, T043: kiểm bằng Vitest dựng cả portal (router, phiên, chunk lazy được spy); Playwright E2E (T003, T035, T039) và quickstart thật (T042) còn lại — ghi CODE_PLAN.

| AC | Test (file › tên) |
|---|---|
| F079-AC-01 | `game-ops/__tests__/ConfigListPage.test.tsx` › lists the newest version…; menus follow getMyPermissions…; a type without type module… |
| F079-AC-02 | `ConfigEditor.test.tsx` › saves the draft with If-Match… 412…; ops-ui `game-console/__tests__/SchemaForm.test.tsx` (4 + 3 test) |
| F079-AC-03 | `ConfigEditor.test.tsx` › author: "Gửi duyệt"…; reviewer: approve, reject with a mandatory note; Live-ops publishes an approved version now |
| F079-AC-04 | `ConfigEditor.test.tsx` › published version is read only…; rollback to an older published version (quests)… |
| F079-AC-05 | `ConfigEditor.test.tsx` › 422 VALIDATION_FAILED on save…; 422 TRANSLATION_INCOMPLETE on submit_review…; `SchemaForm.test.tsx` › shows server field errors… |
| F079-AC-06 | `Builders.test.tsx` › adds steps talk_npc → activity → collect…; adds nodes, links n1 → n2… |
| F079-AC-07 | `Builders.test.tsx` (manifest / published pickers); ops-ui `ManifestRefPicker.test.tsx` (2); Go integration `TestGameAdminClientManifest` |
| F079-AC-08 | `Builders.test.tsx` › price = amount of the in-game currency only… |
| F079-AC-09 | `Progression.test.tsx` › formula fields from the schema; thresholds, chart… |
| F079-AC-10 | `Progression.test.tsx` (lần gọi `:simulate` thứ hai, số liệu là của server); Go integration `TestGameAdminSimulate` + unit `progression/simulate_test.go` |
| F079-AC-11 | `Progression.test.tsx` › server errors on thresholds / caps…; the publish confirmation of a curve… |
| F079-AC-12 | `Events.test.tsx` › 8 steps; bilingual name with status… |
| F079-AC-13 | `Events.test.tsx` › AC-13: server errors mark and block their steps…; (lời nhắc > 3 phiên/tuần ở test AC-12) |
| F079-AC-14 | `Events.test.tsx` › asks the server for the ICT window…; Go integration `TestGameAdminEventCalendar` + unit `liveops/calendar_test.go` |
| F079-AC-15 | `Events.test.tsx` › season track…; unit `season/simulate_test.go`; event template: allowed_params…; `SchemaForm.test.tsx` (const `paid_track` không control) |
| F079-AC-16 | ops-ui `ThumbnailImage.test.tsx` (5); `ConfigListPage.test.tsx` (ảnh/nhãn trong danh sách); Go integration `TestGameAdminRenderStatus` |
| F079-AC-17 | ops-ui `GamePreviewFrame.test.tsx`; `Errors.test.tsx` › creates a preview session… |
| F079-AC-18 | `ConfigListPage.test.tsx` › ad.truong.a / qlcm.truong.a: 403, chunk game_ops không tải |
| F079-AC-19 | `Errors.test.tsx` › 403 PERMISSION_DENIED → 403 page with the trace id; internal gateway unreachable → VPN |
| F079-AC-20 | `Errors.test.tsx` › a retry of the same create reuses the Idempotency-Key…; `SchemaForm.test.tsx` (maxLength + bộ đếm) |
| F079-AC-21 | `ConfigListPage.test.tsx` › a turned-off menu (shop)…; 403 FEATURE_DISABLED…; the create button is inside ActionGate… |
| T045 | `Dashboard.test.tsx` › shows the server counts…; Go integration `TestGameAdminConfigCounts` |
