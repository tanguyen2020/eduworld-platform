# Tasks: F070 — Hồ sơ người chơi, nickname, tạo avatar lần đầu, trang phục, emote

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Cấu hình `nickname_change_monthly_limit`, `nickname_suggestion_ttl` đọc từ platform settings (không hardcode) — file: `src/backend/modules/game/internal/config/profile.go`
- [ ] T002 [P] Seed danh mục v1 (6 preset, dáng, bảng màu, item/emote mặc định, bộ từ nickname vi/en) + item definitions mặc định — file: `src/backend/modules/game/seed/character_catalog.v1.json`, `src/backend/modules/game/seed/items.default.v1.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 JSON Schema `avatar-config.schema.json`, `character-catalog.schema.json` (mọi chuỗi có `maxLength`) — file: `specs/schemas/` — kiểm: `make specs-lint`
- [ ] T004 Sửa `AvatarConfig`, `GameProfile`, `GameProfileUpdate`; thêm `getCharacterCatalog`; ProblemCode `NICKNAME_UNAVAILABLE`, `NICKNAME_CHANGE_LIMIT`; mọi `nickname` `maxLength: 32` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T005 [P] Payload event `avatar.created`, `avatar.updated`, `game_profile.updated` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [ ] T006 ent schema `GameProfile`, `NicknameChange` (AppendOnly), `AvatarPreset`, `CharacterCatalog`/`CharacterCatalogVersion`, `ItemDefinition`/`ItemDefinitionVersion`, `InventoryItem` với `MaxLen` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f070_profile`, `make migrate-lint MOD=game`
- [ ] T007 Migration: partial unique nickname theo tenant, CHECK kích thước jsonb, RLS, `REVOKE UPDATE, DELETE, TRUNCATE` trên `nickname_changes` — file: `src/backend/modules/game/migrations/`
- [ ] T008 Lệnh seed nạp danh mục/item qua command publish nội bộ (không ghi bảng trực tiếp) — file: `src/backend/modules/game/cmd/seed/main.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo nhân vật lần đầu (Priority: P1) 🎯 MVP

**Goal**: Học sinh mới tạo avatar và vào Main Hub.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Test F070-AC-01: `StartGameSession` tạo profile một lần (gọi lại không tạo thêm), `getGameProfile` trả `setup_required` — file: `src/backend/modules/game/tests/integration/profile/ensure_profile_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Test F070-AC-02: `getCharacterCatalog` trả bản publish, đủ vi/en, ETag/304 — file: `src/backend/modules/game/tests/integration/profile/catalog_test.go`
- [ ] T011 [P] [US1] Test F070-AC-03, F070-AC-04, F070-AC-19: lưu avatar lần đầu đặt `avatar_created_at`, `avatar.created` đúng 1 lần (lặp + 10 request đồng thời), emote mặc định; màu/dáng ngoài danh mục → 422 — file: `src/backend/modules/game/tests/integration/profile/avatar_create_test.go`
- [ ] T012 [P] [US1] Unit test `AvatarConfig.Validate` mọi tổ hợp slot hợp lệ/không hợp lệ — file: `src/backend/modules/game/internal/domain/profile/avatar_config_test.go`
- [ ] T013 [P] [US1] GdUnit4 test F070-AC-05, F070-AC-06: luồng `g1_3` ≤ 3 bước không cần bàn phím; đổi lựa chọn không gọi `ApiClient` — file: `src/game/tests/unit/player/character_creation_test.gd`

### Implementation cho User Story 1

- [ ] T014 [P] [US1] Domain `GameProfile`, `AvatarConfig`, `CharacterCatalog` — file: `src/backend/modules/game/internal/domain/profile/`, `src/backend/modules/game/internal/domain/catalog/`
- [ ] T015 [US1] Command `EnsureGameProfile` gắn vào transaction `StartGameSession` (cập nhật `last_game_login_at`) — file: `src/backend/modules/game/internal/application/command/ensure_game_profile.go`
- [ ] T016 [US1] Command `UpdateAvatar` (outbox `avatar.created`/`avatar.updated`/`game_profile.updated`) + query `GetGameProfile`, `GetCharacterCatalog` — file: `src/backend/modules/game/internal/application/command/update_avatar.go`, `src/backend/modules/game/internal/application/query/get_character_catalog.go`
- [ ] T017 [US1] Handler `getGameProfile`, `updateAvatar`, `getCharacterCatalog` — file: `src/backend/modules/game/internal/presentation/http/public/profile.go`
- [ ] T018 [US1] `ProfileService`, `avatar_builder.gd`, scene `CharacterCreation.tscn` (S02, Theme token, khóa `ui.avatar.*`) — file: `src/game/scripts/player/profile_service.gd`, `src/game/scripts/player/avatar_builder.gd`, `src/game/scenes/character/CharacterCreation.tscn`

## Phase 4: User Story 2 — Nickname an toàn (Priority: P1)

### Tests cho User Story 2

- [ ] T019 [P] [US2] Test F070-AC-07: gợi ý theo `Accept-Language`, ≤ 24 ký tự, loại tên đã dùng trong tenant; 10.000 gợi ý không trùng tên thật seed (SC-002) — file: `src/backend/modules/game/tests/integration/profile/nickname_suggest_test.go` — phải FAIL trước khi code
- [ ] T020 [P] [US2] Test F070-AC-08, F070-AC-09, F070-AC-10: đặt nickname qua option; option hết hạn/của người khác/bị lấy → 409; body có `nickname` → 422 — file: `src/backend/modules/game/tests/integration/profile/nickname_change_test.go`
- [ ] T021 [P] [US2] Test F070-AC-11: vượt `nickname_change_monthly_limit` → `409 NICKNAME_CHANGE_LIMIT` (Clock giả) — file: `src/backend/modules/game/tests/integration/profile/nickname_limit_test.go`
- [ ] T022 [P] [US2] Contract test F070-AC-12: schema `GameProfile`, payload `game_profile.updated`/`avatar.updated` không có trường họ tên/username/trường/lớp — file: `src/backend/modules/game/tests/contract/profile_privacy_test.go`

### Implementation cho User Story 2

- [ ] T023 [P] [US2] `NicknameGenerator` + `NicknameOptionStore` Valkey — file: `src/backend/modules/game/internal/domain/profile/nickname_generator.go`, `src/backend/modules/game/internal/infrastructure/cache/nickname_option_store.go`
- [ ] T024 [US2] Query `SuggestNicknames`, command `ChangeNickname` (ghi `nickname_changes`, outbox) + handler `suggestNicknames`, `updateGameProfile` — file: `src/backend/modules/game/internal/application/command/change_nickname.go`
- [ ] T025 [US2] Bước Tên ở S02 (`NicknameChoiceChips`, "Gợi ý khác", tự làm mới khi 409) — file: `src/game/scripts/player/character_creation.gd`

## Phase 5: User Story 3 — Tủ đồ: trang phục, phụ kiện, bộ phối đồ (Priority: P2)

### Tests cho User Story 3

- [ ] T026 [P] [US3] Test F070-AC-13, F070-AC-14, F070-AC-15: item không sở hữu → `ITEM_NOT_OWNED`; sai slot/trùng slot/quá số emote → `EQUIP_SLOT_INVALID`; đổi thành công → hash mới + outbox cùng transaction (TC-ECO-007) — file: `src/backend/modules/game/tests/integration/profile/avatar_equip_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Test F070-AC-16, F070-AC-17: preset ô 1–3, ô 4 → 422; chuỗi/jsonb quá giới hạn → 422 — file: `src/backend/modules/game/tests/integration/profile/avatar_preset_test.go`
- [ ] T028 [P] [US3] GdUnit4 test F070-AC-18: item thiếu visual → item mặc định cùng slot, không crash — file: `src/game/tests/unit/player/avatar_builder_test.gd`

### Implementation cho User Story 3

- [ ] T029 [US3] `OwnershipReader` (default items + `inventory_items`), command `SaveAvatarPreset`, query `ListAvatarPresets` + handler — file: `src/backend/modules/game/internal/infrastructure/persistence/ownership_reader.go`, `src/backend/modules/game/internal/application/command/save_avatar_preset.go`
- [ ] T030 [US3] Scene `CustomizationStudio.tscn` (Tủ đồ, lớp 1–3 gộp tab "Trang phục") + `InventoryService` đọc danh mục — file: `src/game/scenes/character/CustomizationStudio.tscn`, `src/game/scripts/inventory/inventory_service.gd`

## Phase 6: User Story 4 — Emote mặc định (Priority: P2)

### Tests cho User Story 4

- [ ] T031 [P] [US4] GdUnit4 scene test F070-AC-20: chọn emote phát clip one-shot rồi về Locomotion; wheel chỉ hiện emote trang bị — file: `src/game/tests/unit/player/emote_wheel_test.gd` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T032 [US4] `EmoteWheel.tscn` + `emote_wheel.gd` (input `emote_wheel`, cảm ứng) gọi `anim_driver.gd` — file: `src/game/scenes/character/EmoteWheel.tscn`, `src/game/scripts/player/emote_wheel.gd`

## Phase 7: User Story 5 — Bảo vệ hồ sơ: phân quyền và cô lập tenant (Priority: P1)

### Tests cho User Story 5

- [ ] T033 [P] [US5] Test authorization F070-AC-21: mọi route F070 × {school_admin, academic_manager, tenant_admin, safety_moderator, system_admin} → 403 (sinh từ OpenAPI, TC-AUTH-014) — file: `src/backend/modules/game/tests/integration/profile/player_routes_authz_test.go` — phải FAIL trước khi code
- [ ] T034 [P] [US5] Test tenant isolation F070-AC-22: header tenant khác → `TENANT_MISMATCH`; nickname trùng ở tenant khác vẫn hợp lệ; RLS chặn đọc profile tenant khác — file: `src/backend/modules/game/tests/integration/profile/tenant_isolation_test.go`

### Implementation cho User Story 5

- [ ] T035 [US5] Gắn middleware `game_player` + `tenant` cho router profile; repository dùng `TenantScope` — file: `src/backend/modules/game/internal/presentation/http/public/router.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T036 [P] Đăng ký `nickname_change_monthly_limit`, `nickname_suggestion_ttl` (CONFIG_DEFAULTS + CHARACTER_SYSTEM §5.3); DATABASE_DESIGN §2.8/§2.8a (nickname unique theo tenant, `game.nickname_changes`, `character_catalog*`, cột mới); EVENT_DESIGN §4.5 (`game_profile.updated`); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`
- [ ] T037 [P] Metric `eduworld_game_avatar_*`, `eduworld_game_nickname_conflict_total` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T039 **OQ chốt 2026-11-20** (OQ F016-2, F045-3) Facade `cleanupStudentData`/`restoreStudentData`/`anonymizeUserData`/`mergeUserData` của module game cho GameProfile; facade test hai phía với identity — file: `src/backend/modules/game/internal/presentation/facade/`, `src/backend/modules/game/internal/application/command/` — kiểm: `make test-contract`, `make test-integration MOD=game` — cỡ M

## Dependencies & Execution Order

- Setup → Foundational (T004, T006 trước mọi story) → US1 → US2, US5 song song → US3 → US4 → Polish.
- T015 sửa transaction `StartGameSession` của F014 (F014 phải Done).

## Definition of Done

- [ ] Mọi tiêu chí F070-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game) và `make godot-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
