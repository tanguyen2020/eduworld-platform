# Tasks: F070 — Hồ sơ người chơi, nickname, tạo avatar lần đầu, trang phục, emote

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Cấu hình `nickname_change_monthly_limit`, `nickname_suggestion_ttl` đọc từ platform settings (không hardcode) — file: `src/backend/modules/game/internal/config/profile.go` — ✅ `internal/infrastructure/config/profile.go` (`EW_GAME_NICKNAME_CHANGE_MONTHLY_LIMIT` 1, `EW_GAME_NICKNAME_SUGGESTION_TTL` 10m, `EW_GAME_EMOTE_WHEEL_SLOTS` 6, `EW_GAME_SAVED_OUTFITS_MAX` 3; trường chỉ siết qua `ref.schools.settings_subset.nickname_change_monthly_limit`) — `TestProfileFrom` (2026-11-21)
- [x] T002 [P] Seed danh mục v1 (6 preset, dáng, bảng màu, item/emote mặc định, bộ từ nickname vi/en) + item definitions mặc định — file: `src/backend/modules/game/seed/character_catalog.v1.json`, `src/backend/modules/game/seed/items.default.v1.json` — ✅ `seed/character_catalog.v1.json` (6 preset, 4 dáng, 6 màu da, 8 màu tóc, 16 item + 6 emote mặc định, bộ từ vi/en tránh tên học sinh seed), `seed/items.default.v1.json` (22 item) — `TestSeedCatalogMatchesSchema` (2026-11-21)

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 JSON Schema `avatar-config.schema.json`, `character-catalog.schema.json` (mọi chuỗi có `maxLength`) — file: `specs/schemas/` — kiểm: `make specs-lint` — ✅ `specs/schemas/avatar-config.schema.json`, `character-catalog.schema.json` (2026-11-20; `validate-schemas` 0 lỗi, `check-maxlength` 0 vi phạm)
- [x] T004 Sửa `AvatarConfig`, `GameProfile`, `GameProfileUpdate`; thêm `getCharacterCatalog`; ProblemCode `NICKNAME_UNAVAILABLE`, `NICKNAME_CHANGE_LIMIT`; mọi `nickname` `maxLength: 32` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game` — ✅ `AvatarConfig` theo CHARACTER_SYSTEM §5.1 (+ `CatalogCode`), `GameProfile` (`setup_required`, `avatar_created_at`, `avatar_config_hash`, `nickname` nullable), `GameProfileUpdate` `additionalProperties: false` + `409`, `getCharacterCatalog` (`CharacterCatalog`, ETag/304), ProblemCode `NICKNAME_UNAVAILABLE`/`NICKNAME_CHANGE_LIMIT` (API_DESIGN §4.1, `codes_gen.go`), mọi `*nickname` `maxLength: 32` (OpenAPI + AsyncAPI); `split-public`, routematrix gen, `gen-routes`, portal `gen:api` (2026-11-20)
- [x] T005 [P] Payload event `avatar.created`, `avatar.updated`, `game_profile.updated` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint` — ✅ `specs/events/payloads/game/avatar.created.v1`, `avatar.updated.v1`, `game_profile.updated.v1` (không PII, F070-AC-12) (2026-11-20)
- [x] T006 ent schema `GameProfile`, `NicknameChange` (AppendOnly), `AvatarPreset`, `CharacterCatalog`/`CharacterCatalogVersion`, `ItemDefinition`/`ItemDefinitionVersion`, `InventoryItem` với `MaxLen` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f070_profile`, `make migrate-lint MOD=game` — ✅ `ent/schema/profile.go` (8 bảng, mọi chuỗi `MaxLen`); `game_profiles`, `avatar_presets`, `inventory_items` dùng khóa thay thế + unique partial (ent không có khóa ghép; một học sinh có thể ở nhiều tenant) — `TestMigrationMatchesEnt`, entlint 0 (2026-11-21)
- [x] T007 Migration: partial unique nickname theo tenant, CHECK kích thước jsonb, RLS, `REVOKE UPDATE, DELETE, TRUNCATE` trên `nickname_changes` — file: `src/backend/modules/game/migrations/` — ✅ `migrations/20261224000001_f070_profile.sql` (unique `(tenant_id, lower(nickname))`, CHECK jsonb ≤ 16/8/64 KB, RLS FORCE, REVOKE UPDATE/DELETE/TRUNCATE `nickname_changes`, nội dung platform chỉ `game_platform` ghi, trigger version bất biến, view `_live`); `atlas.sum` làm mới; tenancyguard 0 (2026-11-21)
- [x] T008 Lệnh seed nạp danh mục/item qua command publish nội bộ (không ghi bảng trực tiếp) — file: `src/backend/modules/game/cmd/seed/main.go` — ✅ `cmd/seed/main.go` → `command.CatalogPublish` dưới `SystemPlatformScope("game-seed")`, chạy lại không tạo version mới; Makefile `seed-game` — `TestCharacterCatalog` (publish lại cùng payload giữ ETag) (2026-11-21)

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo nhân vật lần đầu (Priority: P1) 🎯 MVP

**Goal**: Học sinh mới tạo avatar và vào Main Hub.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Test F070-AC-01: `StartGameSession` tạo profile một lần (gọi lại không tạo thêm), `getGameProfile` trả `setup_required` — file: `src/backend/modules/game/tests/integration/profile/ensure_profile_test.go` — phải FAIL trước khi code — ✅ `tests/integration/profile_setup_test.go::TestEnsureProfileOnSessionStart` (đặt cùng gói env F014 thay vì `tests/integration/profile/`) (2026-11-21)
- [x] T010 [P] [US1] Test F070-AC-02: `getCharacterCatalog` trả bản publish, đủ vi/en, ETag/304 — file: `src/backend/modules/game/tests/integration/profile/catalog_test.go` — ✅ `profile_setup_test.go::TestCharacterCatalog` (6 preset, vi/en, ETag/304, version mới đổi ETag, không gửi bộ từ nickname) (2026-11-21)
- [x] T011 [P] [US1] Test F070-AC-03, F070-AC-04, F070-AC-19: lưu avatar lần đầu đặt `avatar_created_at`, `avatar.created` đúng 1 lần (lặp + 10 request đồng thời), emote mặc định; màu/dáng ngoài danh mục → 422 — file: `src/backend/modules/game/tests/integration/profile/avatar_create_test.go` — ✅ `profile_setup_test.go::TestAvatarCreatedOnce` (10 request đồng thời + lặp → 1 `avatar.created`, 6 emote mặc định, 422 ngoài danh mục, schema event) (2026-11-21)
- [x] T012 [P] [US1] Unit test `AvatarConfig.Validate` mọi tổ hợp slot hợp lệ/không hợp lệ — file: `src/backend/modules/game/internal/domain/profile/avatar_config_test.go` — ✅ `internal/domain/profile/avatar_config_test.go` (19 tổ hợp lỗi + hợp lệ, hash, `SetAvatar` một lần, `NicknamePolicy`) (2026-11-21)
- [x] T013 [P] [US1] GdUnit4 test F070-AC-05, F070-AC-06: luồng `g1_3` ≤ 3 bước không cần bàn phím; đổi lựa chọn không gọi `ApiClient` — file: `src/game/tests/unit/player/character_creation_test.gd` — ✅ Godot `tests/unit/player/test_character_creation.gd`, `tests/scene/character/test_character_creation_scene.gd` (g1_3 ≤ 3 bước, không ô nhập, nút ≥ 64, xem trước < 100 ms không request) (2026-11-21)

### Implementation cho User Story 1

- [x] T014 [P] [US1] Domain `GameProfile`, `AvatarConfig`, `CharacterCatalog` — file: `src/backend/modules/game/internal/domain/profile/`, `src/backend/modules/game/internal/domain/catalog/` — ✅ `internal/domain/profile/` (`Profile`, `AvatarConfig`, `Validate`, `NicknamePolicy`, `MergeSettings`), `internal/domain/catalog/` (2026-11-21)
- [x] T015 [US1] Command `EnsureGameProfile` gắn vào transaction `StartGameSession` (cập nhật `last_game_login_at`) — file: `src/backend/modules/game/internal/application/command/ensure_game_profile.go` — ✅ `command/ensure_game_profile.go`, gọi trong transaction `Sessions.Start` (cập nhật `last_game_login_at`) (2026-11-21)
- [x] T016 [US1] Command `UpdateAvatar` (outbox `avatar.created`/`avatar.updated`/`game_profile.updated`) + query `GetGameProfile`, `GetCharacterCatalog` — file: `src/backend/modules/game/internal/application/command/update_avatar.go`, `src/backend/modules/game/internal/application/query/get_character_catalog.go` — ✅ `command/profile.go` (`UpdateAvatar`: outbox `avatar.created`/`avatar.updated` + `game_profile.updated`, `preset_code` khi trùng preset), `query/profile.go` (`GetGameProfile`, `GetCharacterCatalog`) (2026-11-21)
- [x] T017 [US1] Handler `getGameProfile`, `updateAvatar`, `getCharacterCatalog` — file: `src/backend/modules/game/internal/presentation/http/public/profile.go` — ✅ `presentation/http/profile.go` + 7 operation trong `routes.go` (`game_player`, X-Tenant-Id) — `TestOperationsMatchSpec` (15 operation) (2026-11-21)
- [x] T018 [US1] `ProfileService`, `avatar_builder.gd`, scene `CharacterCreation.tscn` (S02, Theme token, khóa `ui.avatar.*`) — file: `src/game/scripts/player/profile_service.gd`, `src/game/scripts/player/avatar_builder.gd`, `src/game/scenes/character/CharacterCreation.tscn` — ✅ Godot `scripts/player/profile_service.gd`, `avatar_builder.gd`, `character_creation.gd`, `scenes/character/CharacterCreation.tscn` (khóa `ui.avatar.*`, Theme token) (2026-11-21)

## Phase 4: User Story 2 — Nickname an toàn (Priority: P1)

### Tests cho User Story 2

- [x] T019 [P] [US2] Test F070-AC-07: gợi ý theo `Accept-Language`, ≤ 24 ký tự, loại tên đã dùng trong tenant; 10.000 gợi ý không trùng tên thật seed (SC-002) — file: `src/backend/modules/game/tests/integration/profile/nickname_suggest_test.go` — phải FAIL trước khi code — ✅ `profile_nickname_test.go::TestSuggestNicknames` (vi/en, ≤ 24, loại tên đã dùng trong tenant, tenant khác không tính); SC-002: `tests/contract/profile_test.go::TestSuggestionsNeverContainRealNames` (10.000 gợi ý) (2026-11-21)
- [x] T020 [P] [US2] Test F070-AC-08, F070-AC-09, F070-AC-10: đặt nickname qua option; option hết hạn/của người khác/bị lấy → 409; body có `nickname` → 422 — file: `src/backend/modules/game/tests/integration/profile/nickname_change_test.go` — ✅ `profile_nickname_test.go::TestChangeNickname` (option của người khác/không biết/hết hạn/bị lấy → 409, `nickname` tự do → 422, lịch sử `initial`, event) (2026-11-21)
- [x] T021 [P] [US2] Test F070-AC-11: vượt `nickname_change_monthly_limit` → `409 NICKNAME_CHANGE_LIMIT` (Clock giả) — file: `src/backend/modules/game/tests/integration/profile/nickname_limit_test.go` — ✅ `profile_nickname_test.go::TestNicknameMonthlyLimit` (Clock giả, `retry_after` = đầu tháng sau ICT, trường siết = 0) (2026-11-21)
- [x] T022 [P] [US2] Contract test F070-AC-12: schema `GameProfile`, payload `game_profile.updated`/`avatar.updated` không có trường họ tên/username/trường/lớp — file: `src/backend/modules/game/tests/contract/profile_privacy_test.go` — ✅ `tests/contract/profile_test.go::TestProfileContractsHaveNoPII` (2026-11-21)

### Implementation cho User Story 2

- [x] T023 [P] [US2] `NicknameGenerator` + `NicknameOptionStore` Valkey — file: `src/backend/modules/game/internal/domain/profile/nickname_generator.go`, `src/backend/modules/game/internal/infrastructure/cache/nickname_option_store.go` — ✅ `domain/profile/nickname_generator.go` (vi: danh từ + tính từ + 2 số; en: tính từ + danh từ), `infrastructure/cache/nickname_option_store.go` — `TestNicknameGenerator`, `TestNicknameOptionStoreValkey` (2026-11-21)
- [x] T024 [US2] Query `SuggestNicknames`, command `ChangeNickname` (ghi `nickname_changes`, outbox) + handler `suggestNicknames`, `updateGameProfile` — file: `src/backend/modules/game/internal/application/command/change_nickname.go` — ✅ `query.Profile.SuggestNicknames` (nhiều vòng sinh khi tenant đông), `command.Profiles.UpdateProfile` (ghi `nickname_changes`, outbox, xóa option sau commit), handler `suggestNicknames`, `updateGameProfile` (merge patch) (2026-11-21)
- [x] T025 [US2] Bước Tên ở S02 (`NicknameChoiceChips`, "Gợi ý khác", tự làm mới khi 409) — file: `src/game/scripts/player/character_creation.gd` — ✅ Godot bước Tên (`NicknameChoiceChips`, "Gợi ý khác", tự lấy gợi ý mới khi 409) — `test_profile_service.gd`, `test_character_creation.gd` (2026-11-21)

## Phase 5: User Story 3 — Tủ đồ: trang phục, phụ kiện, bộ phối đồ (Priority: P2)

### Tests cho User Story 3

- [x] T026 [P] [US3] Test F070-AC-13, F070-AC-14, F070-AC-15: item không sở hữu → `ITEM_NOT_OWNED`; sai slot/trùng slot/quá số emote → `EQUIP_SLOT_INVALID`; đổi thành công → hash mới + outbox cùng transaction (TC-ECO-007) — file: `src/backend/modules/game/tests/integration/profile/avatar_equip_test.go` — phải FAIL trước khi code — ✅ `profile_equip_test.go::TestAvatarEquipment` (ITEM_NOT_OWNED, EQUIP_SLOT_INVALID gồm quá `emote_wheel_slots`=4, item sở hữu hợp lệ, hash mới, 2 event cùng transaction `xmin`, metric) (2026-11-21)
- [x] T027 [P] [US3] Test F070-AC-16, F070-AC-17: preset ô 1–3, ô 4 → 422; chuỗi/jsonb quá giới hạn → 422 — file: `src/backend/modules/game/tests/integration/profile/avatar_preset_test.go` — ✅ `profile_equip_test.go::TestAvatarPresets` (ô 1–3 ghi đè, ô 4/tên 65/item_id 65/16 KB → 422, chỉ preset của mình) (2026-11-21)
- [x] T028 [P] [US3] GdUnit4 test F070-AC-18: item thiếu visual → item mặc định cùng slot, không crash — file: `src/game/tests/unit/player/avatar_builder_test.gd` — ✅ Godot `tests/unit/player/test_avatar_builder.gd` (thiếu visual → item mặc định cùng slot, không crash) (2026-11-21)

### Implementation cho User Story 3

- [x] T029 [US3] `OwnershipReader` (default items + `inventory_items`), command `SaveAvatarPreset`, query `ListAvatarPresets` + handler — file: `src/backend/modules/game/internal/infrastructure/persistence/ownership_reader.go`, `src/backend/modules/game/internal/application/command/save_avatar_preset.go` — ✅ `persistence/write/profile_repo.go` (`OwnershipRepo`, `PresetRepo`, `CatalogRepo`), `command.Profiles.SavePreset`, `query.ListAvatarPresets` + handler (2026-11-21)
- [x] T030 [US3] Scene `CustomizationStudio.tscn` (Tủ đồ, lớp 1–3 gộp tab "Trang phục") + `InventoryService` đọc danh mục — file: `src/game/scenes/character/CustomizationStudio.tscn`, `src/game/scripts/inventory/inventory_service.gd` — ✅ Godot `scenes/character/CustomizationStudio.tscn`, `scripts/inventory/inventory_service.gd` (g1_3 gộp "Trang phục") — `tests/scene/character/test_customization_studio.gd` (10 test) (2026-11-21)

## Phase 6: User Story 4 — Emote mặc định (Priority: P2)

### Tests cho User Story 4

- [x] T031 [P] [US4] GdUnit4 scene test F070-AC-20: chọn emote phát clip one-shot rồi về Locomotion; wheel chỉ hiện emote trang bị — file: `src/game/tests/unit/player/emote_wheel_test.gd` — phải FAIL trước khi code — ✅ Godot `tests/unit/player/test_emote_wheel.gd` (2026-11-21)

### Implementation cho User Story 4

- [x] T032 [US4] `EmoteWheel.tscn` + `emote_wheel.gd` (input `emote_wheel`, cảm ứng) gọi `anim_driver.gd` — file: `src/game/scenes/character/EmoteWheel.tscn`, `src/game/scripts/player/emote_wheel.gd` — ✅ Godot `scenes/character/EmoteWheel.tscn`, `scripts/player/emote_wheel.gd`, `anim_driver.gd` (state Emote one-shot), input `emote_wheel` (2026-11-21)

## Phase 7: User Story 5 — Bảo vệ hồ sơ: phân quyền và cô lập tenant (Priority: P1)

### Tests cho User Story 5

- [x] T033 [P] [US5] Test authorization F070-AC-21: mọi route F070 × {school_admin, academic_manager, tenant_admin, safety_moderator, system_admin} → 403 (sinh từ OpenAPI, TC-AUTH-014) — file: `src/backend/modules/game/tests/integration/profile/player_routes_authz_test.go` — phải FAIL trước khi code — ✅ `player_routes_authz_test.go::TestPlayerRoutesStaffDenied` mở rộng cho 7 route F070 (5 role × 7 route → 403, không ghi profile/preset) (2026-11-21)
- [x] T034 [P] [US5] Test tenant isolation F070-AC-22: header tenant khác → `TENANT_MISMATCH`; nickname trùng ở tenant khác vẫn hợp lệ; RLS chặn đọc profile tenant khác — file: `src/backend/modules/game/tests/integration/profile/tenant_isolation_test.go` — ✅ `profile_tenant_test.go::TestProfileTenantIsolation` (7 route × X-Tenant-Id khác → TENANT_MISMATCH, nickname trùng ở tenant khác hợp lệ, RLS + không DELETE + append-only) (2026-11-21)

### Implementation cho User Story 5

- [x] T035 [US5] Gắn middleware `game_player` + `tenant` cho router profile; repository dùng `TenantScope` — file: `src/backend/modules/game/internal/presentation/http/public/router.go` — ✅ 7 operation khai báo `httpx.GuardGamePlayer` + `TenantRequired` trong `routes.go` (chain httpx), repository lọc `tenant_id` + `deleted_at` (router riêng `public/router.go` không cần: module dùng chung `Register`) (2026-11-21)

## Phase 8: Polish & Cross-Cutting

- [x] T036 [P] Đăng ký `nickname_change_monthly_limit`, `nickname_suggestion_ttl` (CONFIG_DEFAULTS + CHARACTER_SYSTEM §5.3); DATABASE_DESIGN §2.8/§2.8a (nickname unique theo tenant, `game.nickname_changes`, `character_catalog*`, cột mới); EVENT_DESIGN §4.5 (`game_profile.updated`); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md` — ✅ CONFIG_DEFAULTS đã có 2 tham số (kiểm 2026-11-21); DATABASE_DESIGN §2.8/§2.8a (khóa thay thế, `nickname_changes`, `character_catalog*`, `item_definition_versions`, cột mới); EVENT_DESIGN §4.5 đã có `game_profile.updated`; CHANGELOG (2026-11-21)
- [x] T037 [P] Metric `eduworld_game_avatar_*`, `eduworld_game_nickname_conflict_total` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go` — ✅ `infrastructure/observability/profile_metrics.go` (`eduworld_game_avatar_created_total`, `eduworld_game_avatar_update_rejected_total{reason}`, `eduworld_game_nickname_conflict_total`) (2026-11-21)
- [x] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — ✅ Kịch bản 1–10 của quickstart chạy tự động bằng integration test (bảng bằng chứng trong tasks); 11–12 bằng GdUnit4 headless. Ảnh S02 trên DEV-PC-MID/DEV-AND-MID1 và trace_id thật chưa có (cần stack compose + thiết bị lab) (2026-11-21)

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-21.

- [x] T039 **OQ chốt 2026-11-20** (OQ F016-2, F045-3) Facade `cleanupStudentData`/`restoreStudentData`/`anonymizeUserData`/`mergeUserData` của module game cho GameProfile; facade test hai phía với identity — file: `src/backend/modules/game/internal/presentation/facade/`, `src/backend/modules/game/internal/application/command/` — kiểm: `make test-contract`, `make test-integration MOD=game` — cỡ M — ✅ `command/student_lifecycle.go` (cleanup soft-delete profile + preset, restore xóa nickname bị chiếm, anonymize `an_<16 hex>`, merge giữ profile đích/di chuyển, idempotent), facade `MergeUserData`, gateway identity `GameMerge` — `facade_test.go::TestProfileLifecycleFacade`, `tests/integration/profile_lifecycle_test.go::TestProfileLifecycleFacade`, identity `tests/contract/owner_gateway_game_test.go::TestOwnerGatewayGameMerge` (2026-11-21)
- [x] T040 **OQ chốt 2026-11-21** (OQ F070-1) Tên item đa ngôn ngữ trong danh mục nhân vật: thêm `name` (`vi`, `en`, mỗi chuỗi ≤ 64) vào `character-catalog` JSON Schema + seed `character_catalog.v1.json`/`items.default.v1.json` + `getCharacterCatalog`; client hiển thị tên theo locale, thiếu bản dịch → nhãn `ui.learning.no_translation`; test contract + GdUnit4 — file: `specs/schemas/`, `src/backend/modules/game/seed/`, `src/game/scripts/player/` — ✅ `items[]` (`item_id`, `equip_slot`, `name` vi/en ≤ 64) trong `character-catalog.schema.json`, OpenAPI `CharacterCatalog.items`/`CatalogItem`, seed 22 mục, `CheckPayload` buộc đúng một mục cho mỗi item/emote mặc định; client `CharacterCatalogModel.item_name/item_label` (tên theo locale, thiếu bản dịch → tên ngôn ngữ khác + `ui.learning.no_translation`, thiếu mục → nhãn thứ tự) cho carousel S02, Tủ đồ, vòng emote — `tests/contract/profile_test.go::TestSeedCatalogMatchesSchema`, `TestCatalogItemsAreChecked`, `tests/integration/profile_setup_test.go::TestCharacterCatalog`, GdUnit4 `tests/unit/player/test_catalog_item_names.gd` (5 test) (2026-11-21)
- [ ] T041 **OQ chốt 2026-11-21** (OQ F070-2) Bảng màu da/tóc là token: khai báo palette avatar trong `design/game-tokens/` (mã màu nghiệp vụ theo R32), sinh sang Godot bằng `make design-tokens`; danh mục tham chiếu khóa palette thay mã màu; bỏ resource placeholder màu ở client; kiểm `design-tokens-check` — file: `design/game-tokens/`, `src/game/resources/`

## Dependencies & Execution Order

- Setup → Foundational (T004, T006 trước mọi story) → US1 → US2, US5 song song → US3 → US4 → Polish.
- T015 sửa transaction `StartGameSession` của F014 (F014 phải Done).

## Definition of Done

- [x] Mọi tiêu chí F070-AC-* có test pass (ghi mã test/TC) — 22/22 AC có test pass (Go: 73 integration + unit/contract; Godot 530 test)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game) và `make godot-test` xanh — chạy lệnh gốc của Makefile (2026-11-21)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — test double gắn nhãn TEST DOUBLE; visual Godot là placeholder có nhãn (OQ-16)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — Done
