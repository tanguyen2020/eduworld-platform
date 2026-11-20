# Tasks: F082 — Game Studio — quản lý sticker/GIF và dịch nội dung game (`/game-admin/translations*`)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Đăng ký menu "Tài nguyên chat" (`game.chat_assets.write`) và "Dịch nội dung game" (`game.content.translate`) + route lazy; thêm `sticker-packs`, `gifs` vào registry loại — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/chat-assets/routes.tsx`, `src/cms/frontend/apps/portal/src/workspaces/game-ops/translations/routes.tsx`, `src/cms/frontend/apps/portal/src/workspaces/game-ops/registry/configTypes.ts`
- [ ] T002 [P] Khóa i18n `portal.game_ops.chat_assets.*`, `portal.game_ops.translations.*` + map `ProblemCode` — file: `src/cms/frontend/apps/portal/src/i18n/locales/vi/portal.game_ops.json`, `src/cms/frontend/apps/portal/src/i18n/locales/en/portal.game_ops.json`
- [ ] T003 [P] Seed: quest thiếu `en`, NPC dialogue đã dịch; tệp webp/GIF mẫu và tệp vượt giới hạn; reward rule publish làm `unlock_rule` — file: `deploy/seed/game-config/quests/`, `src/backend/modules/game/tests/fixtures/translations/`, `src/cms/frontend/apps/portal/tests/fixtures/chat-assets/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Contract game: `GameConfigVersion.translation_status`/`missing_fields`/`outdated_fields`, lọc `translation_status` của `listGameConfigs`, `409`/`428` + `report.errors[].code` cho translations, mô tả `presignGameMediaUpload` `kind = document` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T005 Contract chat (nếu F118 chưa làm): `presignChatAssetUpload_chat`, URL ký trong payload Draft, `sticker-pack.schema.json`, `gif.schema.json` — file: `specs/openapi/modules/chat.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml`, `specs/schemas/sticker-pack.schema.json`, `specs/schemas/gif.schema.json` — kiểm: `make specs-lint`, `make gen MOD=chat`
- [ ] T006 ent schema `ConfigTranslationState`, `TranslationJob` (`MaxLen`, CHECK, index partial) + migration — file: `src/backend/modules/game/ent/schema/configtranslationstate.go`, `src/backend/modules/game/ent/schema/translationjob.go` — kiểm: `make migrate-diff MOD=game name=f082_translations`, `make migrate-lint MOD=game`
- [ ] T007 Sinh client web — file: `src/cms/frontend/apps/portal/src/shared/api/schema.gen.d.ts`, `src/cms/frontend/apps/portal/src/shared/api/operation-meta.gen.ts` — kiểm: `pnpm gen:api`, `make gen-check MOD=game`
- [ ] T008 [P] Tách phần trình bày `TranslationExportDialog`, `TranslationImportDialog`, `TranslationJobProgress`, `TranslationStatusBadge` từ F053 sang `ops-ui` (props API, không gọi API); workspace B dùng lại — file: `src/cms/frontend/packages/ops-ui/src/translations/TranslationExportDialog.tsx`, `src/cms/frontend/apps/portal/src/workspaces/content-studio/translations/TranslationExportDialog.tsx`

**Checkpoint**: contract, bảng, client sinh và component dùng chung sẵn sàng.

## Phase 3: User Story 1 — Quản lý sticker pack (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test F082-AC-01, F082-AC-02: danh sách `sticker-packs`, menu theo permission, form song ngữ, `UnlockRulePicker` chỉ từ reward-rules/items đã publish, khóa gửi duyệt khi thiếu `vi`/`en` — file: `src/cms/frontend/apps/portal/tests/unit/game-ops/chat-assets/StickerPackEditor.test.tsx` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Unit test F082-AC-03: sha256 trình duyệt, `Idempotency-Key` presign giữ khi retry, PUT không `Authorization`, lỗi `413`/`415`/`422` tại từng tệp, không hằng số giới hạn trong code — file: `src/cms/frontend/packages/ops-ui/src/upload/useChecksumUpload.test.ts`
- [ ] T011 [P] [US1] Unit test F082-AC-04: transition/publish/rollback/unpublish có `Idempotency-Key`, `If-Match` autosave, ẩn Duyệt với tác giả, nút gán tenant chỉ khi `platform.tenant_data.assign` — file: `src/cms/frontend/apps/portal/tests/unit/game-ops/chat-assets/workflow.test.tsx`

### Implementation cho User Story 1

- [ ] T012 [P] [US1] `useChecksumUpload`, `UploadQueue` — file: `src/cms/frontend/packages/ops-ui/src/upload/useChecksumUpload.ts`
- [ ] T013 [US1] Hook `chatAssets.ts`, `presignChatAsset.ts`; `StickerPackListPage`, `StickerPackEditor`, `StickerGridEditor`, `UnlockRulePicker` trên `ContentEditorShell` — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/chat-assets/StickerPackEditor.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Thư viện GIF nội bộ (Priority: P1)

### Tests cho User Story 2

- [ ] T014 [P] [US2] Unit test F082-AC-05, F082-AC-06: lưới preview, lọc tag/trạng thái/người upload, tag ≤ maxLength, ẩn Duyệt với người upload, không có ô URL/tìm GIF ngoài — file: `src/cms/frontend/apps/portal/tests/unit/game-ops/chat-assets/GifLibraryPage.test.tsx`
- [ ] T015 [P] [US2] Unit test F082-AC-07: preview alt text theo `vi`/`en`, nhãn khóa, ảnh chỉ từ URL server trả; lint không import Theme game (TC-UI-010) — file: `src/cms/frontend/apps/portal/tests/unit/game-ops/chat-assets/ChatAssetPreview.test.tsx`

### Implementation cho User Story 2

- [ ] T016 [US2] `GifLibraryPage`, `GifUploadDialog`, `GifReviewPanel`, `ChatAssetPreview` — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/chat-assets/GifLibraryPage.tsx`

## Phase 5: User Story 3 — Trạng thái bản dịch nội dung game (Priority: P1)

### Tests cho User Story 3

- [ ] T017 [P] [US3] Unit test domain F082-AC-08: `Pointers` từ schema (mảng, lồng), `ComputeStatus` partial/complete/outdated theo hash (research R1, R3) — file: `src/backend/modules/game/internal/domain/i18ntext/status_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US3] Integration test F082-AC-08, F082-AC-09: lưu draft ghi `game.config_translation_states` cùng transaction, response có `missing_fields`/`outdated_fields`, lọc `listGameConfigs?translation_status=` — file: `src/backend/modules/game/tests/integration/translation/translation_state_test.go`
- [ ] T019 [P] [US3] Unit test UI F082-AC-09, F082-AC-10, F082-AC-11: bộ lọc + huy hiệu, `BilingualField` "Gốc", drawer diff qua `diffGameConfigVersions`, khóa gửi khi thiếu, cảnh báo khi lỗi thời, map `TRANSLATION_INCOMPLETE` (TC-I18N-001) — file: `src/cms/frontend/apps/portal/tests/unit/game-ops/translations/BilingualConfigFields.test.tsx`

### Implementation cho User Story 3

- [ ] T020 [P] [US3] Domain `i18ntext` (`pointers.go`, `status.go`, `key.go`) — file: `src/backend/modules/game/internal/domain/i18ntext/status.go`
- [ ] T021 [US3] Command `RecomputeTranslationState` gắn vào `CreateGameConfig`/`UpdateGameConfigDraft`/`CreateGameConfigVersion` (F078) + repo; mở rộng `ListGameConfigs` và DTO `GameConfigVersion` — file: `src/backend/modules/game/internal/application/command/translation/recompute_translation_state.go`, `src/backend/modules/game/internal/application/query/list_game_configs.go`
- [ ] T022 [US3] `BilingualConfigFields`, `OutdatedDiffDrawer`, bộ lọc trạng thái trong danh sách F079 — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/translations/BilingualConfigFields.tsx`

## Phase 6: User Story 4 — Xuất/nhập file dịch (Priority: P2)

### Tests cho User Story 4

- [ ] T023 [P] [US4] Unit test codec XLIFF 2.1/CSV: khóa ổn định, round-trip, chặn DTD/entity ngoài (XXE), BOM — file: `src/backend/modules/game/internal/infrastructure/translation/xliff21_codec_test.go`
- [ ] T024 [P] [US4] Integration test F082-AC-12: export job `202` → worker → file `ew-exports` (floci), `getGameTranslationJob` có `download_url` ký, chỉ trường thiếu khi `only_missing` (TC-I18N-010) — file: `src/backend/modules/game/tests/integration/translation/export_job_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Integration test F082-AC-13, F082-AC-14: `dry_run` không ghi; nhập tạo Draft mới/ghi Draft mở theo research R2; version published bất biến; `stale_version`, `version_in_review`, `key_not_found`, `too_long` vào `report.errors`; file hỏng → `failed`; cùng `Idempotency-Key` → cùng job — file: `src/backend/modules/game/tests/integration/translation/import_job_test.go`
- [ ] T026 [P] [US4] Unit test UI F082-AC-15: dialog xuất/nhập, poll job, link + hạn, luồng dry run → xác nhận, Idempotency-Key mỗi ý định — file: `src/cms/frontend/apps/portal/tests/unit/game-ops/translations/TranslationsPage.test.tsx`

### Implementation cho User Story 4

- [ ] T027 [P] [US4] `XLIFF21Codec`, `CSVCodec`, `ExportStorage` (presign GET theo `report_link_ttl`) — file: `src/backend/modules/game/internal/infrastructure/translation/xliff21_codec.go`, `src/backend/modules/game/internal/infrastructure/translation/csv_codec.go`
- [ ] T028 [US4] Command `ExportGameTranslations`, `ImportGameTranslations`, `RunTranslationJob` (gọi command handler F078 qua interface có kiểu), query `GetGameTranslationJob`, handler + job `translation-jobs` trong `worker` (profile `jobs`), audit `game_translation.*` — file: `src/backend/modules/game/internal/application/command/translation/import_game_translations.go`, `src/backend/modules/game/internal/presentation/http/public/game_translations.go`, `src/backend/modules/game/internal/presentation/job/translation_jobs.go`
- [ ] T029 [US4] `TranslationsPage` dùng dialog `ops-ui` + hook `gameTranslations.ts`, `gameMediaPresign.ts` — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/translations/TranslationsPage.tsx`

## Phase 7: User Story 5 — Bảo vệ, phân quyền, giới hạn (Priority: P1)

- [ ] T030 [P] [US5] Integration test authorization F082-AC-16: thiếu `game.content.translate`/MFA/azp/role platform → 403 + `authz.denied`; đọc job người khác → 404, System Admin đọc được (TC-CMS-WS-02) — file: `src/backend/modules/game/tests/integration/translation/translations_authz_test.go`
- [ ] T031 [P] [US5] Integration test tenant isolation F082-AC-17: `content_ids`/khóa trỏ sự kiện trường tenant → như không tồn tại, không đọc/ghi; route APISIX public 404 (TC-CMS-WS-03) — file: `src/backend/modules/game/tests/integration/translation/translations_tenant_test.go`, `deploy/apisix/tests/routes_internal_test.sh`
- [ ] T032 [P] [US5] E2E F082-AC-18: `ad.truong.a`, `sm.antoan` → 403, không tải chunk `game-ops`; `MFA_REQUIRED` → step-up; mất gateway nội bộ → thông báo VPN (TC-CMS-WS-01) — file: `src/cms/frontend/apps/portal/tests/e2e/game-ops/translations/access.spec.ts`
- [ ] T033 [P] [US5] Test giới hạn F082-AC-19: `content_ids` 501, `content_types` 21, tag 65 ký tự, chuỗi file vượt `maxLength` → `422 VALIDATION_FAILED` đúng field; UI bộ đếm ký tự (TC-TEN-024) — file: `src/backend/modules/game/tests/integration/translation/translations_limits_test.go`, `src/cms/frontend/apps/portal/tests/unit/game-ops/chat-assets/limits.test.tsx`

## Phase 8: Polish & Cross-Cutting

- [ ] T034 [P] Metric `eduworld_game_translation_jobs_total`, `eduworld_game_translation_rows_total`, span job — file: `src/backend/modules/game/internal/infrastructure/observability/translation_metrics.go`
- [ ] T035 [P] Cập nhật tài liệu: DATABASE_DESIGN (2 bảng mới), CONFIG_DEFAULTS (đề xuất TTL link/giữ file dịch), API_DESIGN (`presignChatAssetUpload_chat`), ADMIN_PORTAL §1.5 (trạng thái dịch game), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/07-backend/API_DESIGN.md`, `CHANGELOG.md`
- [ ] T036 [P] Storybook sáng/tối + visual regression màn sticker/GIF/dịch (TC-UI-004, TC-UI-005) — file: `src/cms/frontend/packages/ops-ui/src/translations/TranslationExportDialog.stories.tsx`, `src/cms/frontend/apps/portal/tests/e2e/game-ops/visual.spec.ts`
- [ ] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/082-game-studio-chat-assets-translations/quickstart.md`
- [ ] T038 [P] Test F082-AC-20: tắt `menu.game_ops.translations` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/game-ops/chat-assets-feature-toggle.spec.ts`
- [ ] T039 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F082-AC-20) — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/routes.tsx`

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T040 **OQ chốt 2026-11-20** (OQ F045-3) Facade platform-ops của module chat cho sticker pack/GIF (gán, gỡ gán, clone, Thùng rác); facade test với identity — file: `src/backend/modules/chat/internal/presentation/facade/` — kiểm: `make test-contract` — cỡ M

## Dependencies & Execution Order

- Setup → Foundational (T004–T008) → US1 → US2 (dùng `useChecksumUpload` của US1) → US3 → US4 (cần domain/command US3) → US5 → Polish.
- US1–US2 (frontend, module chat) và US3–US4 (module game) có thể song song sau Phase 2.
- Phụ thuộc ngoài: F118 US3 Done (backend sticker/GIF + delta T005), F078 Done (command lưu draft, presign game-media), F079 Done (`ContentEditorShell`, registry), F053 (`BilingualField`).

## Definition of Done

- [ ] Mọi tiêu chí F082-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` (module game) và `make web-lint`, `make web-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
