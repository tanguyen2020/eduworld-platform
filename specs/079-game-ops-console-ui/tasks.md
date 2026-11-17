# Tasks: F079 — Workspace A — Game Operations Console UI (world, NPC, quest builder, reward, item, event, season)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `web/apps/portal/` trừ khi ghi rõ. MSW chỉ trong test, nhãn `// MOCK (test-only)` (R17); E2E chạy với backend local thật.

## Phase 1: Setup

- [ ] T001 Route lazy `/game-ops/*` + `RequirePermission` (bất kỳ `game.*`), menu từ `registry/configTypes.ts` — file: `src/workspaces/game-ops/routes.tsx`, `src/workspaces/game-ops/registry/configTypes.ts`
- [ ] T002 [P] File dịch vi/en (`CONTENT_VERSION_CONFLICT`, `VALIDATION_FAILED`, `TRANSLATION_INCOMPLETE`, `MFA_REQUIRED`, trạng thái workflow) — file: `src/workspaces/game-ops/i18n/{vi,en}.json`
- [ ] T003 [P] Fixture Playwright tài khoản platform game + tenant — file: `tests/e2e/fixtures/auth.ts`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract (owner F078): `GameConfigVersion.media`, `listGameConfigs` thêm `q`/`from`/`to`/`sort`, `getGameClientManifest`, `getGameConfigSchema` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 [P] Delta contract (owner F074/F077): `simulateGameConfig` + `SimulationRequest`/`SimulationResult` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T006 [P] Kiểm permission `progression-curves`/`progression-levels` (`game.progression.write`), `collections` (`game.collection.write`), `shop-items` (`game.shop.write`) khớp giữa API_DESIGN §9.13b, ADMIN_PORTAL §2.2 và `registry/configTypes.ts` (RB-309) — file: `docs/07-backend/API_DESIGN.md`, `docs/04-admin-portal/ADMIN_PORTAL.md`
- [ ] T007 Sinh lại client TS — file: `src/api/schema.d.ts` — kiểm: `pnpm gen:api` không diff trong CI
- [ ] T008 [P] `ops-ui` `ContentEditorShell`, `SchemaForm` (rule từ JSON Schema + `maxLength` + bộ đếm), `WorkflowBar`, `VersionTab`, `DiffView`, `useAutosave` + story — file: `web/packages/ops-ui/src/content-editor/`
- [ ] T009 [P] `ops-ui` `GamePreviewFrame`, `ThumbnailImage`, `TurntableViewer`, `ManifestRefPicker` + story — file: `web/packages/ops-ui/src/game/`
- [ ] T010 [P] MSW handler test-only cho contracts §1 — file: `src/workspaces/game-ops/__tests__/msw/handlers.ts`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Danh sách và editor chung (Priority: P1) 🎯 MVP

**Goal**: Vòng đời version cho mọi loại cấu hình game.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T011 [P] [US1] Test F079-AC-01: danh sách theo `config_type`, lọc, cursor, menu theo permission (TC-CMS-001) — file: `src/workspaces/game-ops/__tests__/ConfigListPage.test.tsx` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Test F079-AC-02: form từ schema, autosave `If-Match`, dừng khi 409/412 (TC-CMS-002) — file: `web/packages/ops-ui/src/content-editor/__tests__/ContentEditorShell.test.tsx`
- [ ] T013 [P] [US1] Test F079-AC-03: transition + publish ngay/hẹn giờ có `Idempotency-Key`, ẩn duyệt với tác giả (TC-CMS-001) — file: `…/content-editor/__tests__/WorkflowBar.test.tsx`
- [ ] T014 [P] [US1] Test F079-AC-04: version publish chỉ đọc, diff, releases, rollback/unpublish bắt buộc lý do (TC-CMS-003) — file: `…/content-editor/__tests__/VersionTab.test.tsx`
- [ ] T015 [P] [US1] Test F079-AC-05: map `VALIDATION_FAILED`/`TRANSLATION_INCOMPLETE` về field, chặn gửi duyệt (TC-EVT-010) — file: `…/content-editor/__tests__/ServerErrors.test.tsx`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Hook `gameConfigs.ts`, `schemas.ts` — file: `src/workspaces/game-ops/api/`
- [ ] T017 [US1] `ConfigListPage`, `GenericSchemaBuilder` nối `ContentEditorShell` — file: `src/workspaces/game-ops/list/ConfigListPage.tsx`, `builders/generic/GenericSchemaBuilder.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Builder thế giới, NPC, quest, phần thưởng, item (Priority: P1)

### Tests cho User Story 2

- [ ] T018 [P] [US2] Test F079-AC-06: bước quest kéo-thả đúng `quest.schema.json`; graph NPC đúng `npc-dialogue.schema.json` (TC-GAM-012) — file: `src/workspaces/game-ops/__tests__/QuestStepEditor.test.tsx`, `DialogueGraphEditor.test.tsx` — phải FAIL trước khi code
- [ ] T019 [P] [US2] Test F079-AC-07: bộ chọn tham chiếu chỉ từ manifest/nội dung publish, không nhập ID tự do — file: `web/packages/ops-ui/src/game/__tests__/ManifestRefPicker.test.tsx`
- [ ] T020 [P] [US2] Test F079-AC-08: form loại còn lại theo schema; không trường tiền thật (TC-ECO-003) — file: `src/workspaces/game-ops/__tests__/GenericBuilders.test.tsx`

### Implementation cho User Story 2

- [ ] T021 [P] [US2] Hook `manifest.ts`; `QuestStepEditor`, `DialogueGraphEditor` — file: `src/workspaces/game-ops/api/manifest.ts`, `builders/quest/`, `builders/npc/`
- [ ] T022 [US2] Đăng ký builder cho `reward-rules`, `items`, `shop-items`, `currencies`, `world-areas`, `story-chapters`, `minigame-configs`, `achievements`, `challenges`, `bosses` — file: `src/workspaces/game-ops/registry/configTypes.ts`

## Phase 5: User Story 3 — Level & XP (Priority: P1)

### Tests cho User Story 3

- [ ] T023 [P] [US3] Test F079-AC-09: công thức/bảng, danh hiệu, nguồn XP + cap, biểu đồ từ server (TC-GAM-006) — file: `src/workspaces/game-ops/__tests__/ProgressionCurveEditor.test.tsx` — phải FAIL trước khi code
- [ ] T024 [P] [US3] Test F079-AC-10: mô phỏng gọi `simulateGameConfig`, không tính phía client (spy) — file: `…/__tests__/SimulationPanel.test.tsx`
- [ ] T025 [P] [US3] Test F079-AC-11: lỗi ngưỡng/cap/`effective_from`, hộp xác nhận "không giảm level" — file: `…/__tests__/ProgressionPublish.test.tsx`

### Implementation cho User Story 3

- [ ] T026 [US3] Hook `simulate.ts`; `ProgressionCurveEditor`, `SimulationPanel` — file: `src/workspaces/game-ops/api/simulate.ts`, `builders/progression/`

## Phase 6: User Story 4 — Event Builder, lịch, season, template (Priority: P1)

### Tests cho User Story 4

- [ ] T027 [P] [US4] Test F079-AC-12: wizard 8 bước, song ngữ có chỉ báo, tham chiếu chỉ nội dung publish (TC-EVT-010) — file: `src/workspaces/game-ops/__tests__/EventWizard.test.tsx` — phải FAIL trước khi code
- [ ] T028 [P] [US4] Test F079-AC-13: chặn theo lỗi server; cảnh báo > 3 phiên/tuần không chặn — file: `…/__tests__/EventWizardValidation.test.tsx`
- [ ] T029 [P] [US4] Test F079-AC-14: lịch ICT, màu theo token, cảnh báo chồng chéo từ server — file: `…/__tests__/EventCalendarPage.test.tsx`
- [ ] T030 [P] [US4] Test F079-AC-15: season track không control `paid_track`, mô phỏng; template `allowed_params` (TC-EVT-007) — file: `…/__tests__/SeasonTrackEditor.test.tsx`, `EventTemplateEditor.test.tsx`

### Implementation cho User Story 4

- [ ] T031 [US4] `EventWizard` + `steps/*`, `EventCalendarPage`, `EventTemplateEditor`, `SeasonTrackEditor` — file: `src/workspaces/game-ops/builders/event/`, `builders/season/`

## Phase 7: User Story 5 — Thumbnail và preview (Priority: P2)

### Tests cho User Story 5

- [ ] T032 [P] [US5] Test F079-AC-16: thumbnail đúng version, placeholder khi `pending`/`failed` (TC-UI-009) — file: `web/packages/ops-ui/src/game/__tests__/ThumbnailImage.test.tsx` — phải FAIL trước khi code
- [ ] T033 [P] [US5] Test F079-AC-17: `createGamePreviewSession` + iframe `GamePreviewFrame`; lint không import game ngoài khung (TC-UI-010) — file: `web/packages/ops-ui/src/game/__tests__/GamePreviewFrame.test.tsx`

### Implementation cho User Story 5

- [ ] T034 [US5] Hook `preview.ts`; gắn thumbnail vào danh sách, bộ chọn item/phần thưởng, chi tiết — file: `src/workspaces/game-ops/api/preview.ts`, `list/ConfigListPage.tsx`

## Phase 8: User Story 6 — Bảo vệ workspace A (Priority: P1)

### Tests cho User Story 6

- [ ] T035 [P] [US6] E2E F079-AC-18: `ad.truong.a`, `qlcm.truong.a`, `sm.antoan` → 403, không tải chunk (TC-CMS-AUTHZ-05, TC-CMS-WS-01) — file: `tests/e2e/game-ops/authz.spec.ts` — phải FAIL trước khi code
- [ ] T036 [P] [US6] Test F079-AC-19: step-up MFA, VPN notice (TC-CMS-WS-02) — file: `src/workspaces/game-ops/__tests__/errors.test.tsx`
- [ ] T037 [P] [US6] Test F079-AC-20: `Idempotency-Key` giữ khi retry, `maxLength` + bộ đếm, lỗi `code` + `trace_id` (TC-TEN-024) — file: `src/workspaces/game-ops/__tests__/idempotencyAndLimits.test.tsx`

### Implementation cho User Story 6

- [ ] T038 [US6] Nối `MfaStepUp`, `VpnRequiredNotice`, `LargeScreenRequired` (F051) vào mọi route — file: `src/workspaces/game-ops/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T039 [P] axe + visual regression sáng/tối danh sách, editor, Event Builder (TC-UI-004, TC-UI-006) — file: `tests/e2e/game-ops/a11y-visual.spec.ts`
- [ ] T040 [P] Lint hardcode style, không import game (TC-UI-003, TC-UI-010) — kiểm: `make web-lint`
- [ ] T041 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T042 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [ ] T043 [P] Test F079-AC-21: tắt `menu.game_ops.shop` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `web/apps/portal/tests/e2e/game-ops/feature-toggle.spec.ts`
- [ ] T044 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F079-AC-21) — file: `web/apps/portal/src/workspaces/game-ops/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004–T010) → US1 → US6 → US2 → US3, US4 (song song; cần T005) → US5 (cần T004 `media`) → Polish.
- F108 dùng lại T008 sau khi US1 xong.

## Definition of Done

- [ ] Mọi tiêu chí F079-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật qua gateway nội bộ
- [ ] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
