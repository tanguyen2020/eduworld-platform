# Tasks: F071 — Thế giới: 8 vùng đất, zone, world map, cổng, mở khóa, mạch truyện, NPC và hội thoại

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Seed thế giới v1: 12 area / 8 vùng (vi/en), 7 chương, dialogue 13 NPC, level package greybox — file: `src/backend/modules/game/seed/world/` — ✅ seed/world/{areas,levels,story_chapters}.v1.json + npc_dialogues/*.json (12 area, 7 chương, 14 dialogue/13 NPC, 12 level greybox placeholder) — hợp lệ theo 4 JSON Schema (ajv), `TestPublishGameContent` publish 45 item
- [x] T002 [P] Cấu hình bucket snapshot/bundle (floci), khóa Valkey, TTL presence lấy theo timeout heartbeat F014 — file: `src/backend/modules/game/internal/config/world.go` — ✅ internal/infrastructure/config/world.go (`EW_GAME_DISCOVERY_REPORT_RATE`, presence TTL = `game_session_heartbeat_timeout`, `TestWorldFrom`); snapshot chỉ Valkey (Clarifications 2026-11-21, không bucket floci)

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 JSON Schema `world-area`, `story-chapter`, `level-package` (mọi chuỗi có `maxLength`) — file: `specs/schemas/` — kiểm: `make specs-lint` — ✅ specs/schemas/world-area|story-chapter|level-package.schema.json; validate-schemas, check-maxlength 0 lỗi
- [x] T004 Sửa `enterWorldArea` (`level_id`, `403 AREA_LOCKED` + `missing_requirements`, `409`), `getNpcDialogue` (`$ref` + ETag), `AreaExploration.unlock_requirements.kind` thêm `chapters_completed`; gỡ `joinFriendInWorld` (RB-625) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game` — ✅ eduworld-api.v1.yaml + split-public; `joinFriendInWorld` gỡ; `GameConfigType` để F078 (Clarifications); routes contract test 23 operation
- [x] T005 [P] Payload event `content.published` (game), `world.area_entered`, `world.area_unlocked`, `world.discovery_found`, `world.area_completed`, `story.chapter_completed` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint` — ✅ specs/events/payloads/game/{content.published,world.area_entered,world.area_unlocked,world.discovery_found,world.area_completed,story.chapter_completed}.v1.schema.json; `checkEventSchemas` trong test tích hợp
- [x] T006 [P] Xác nhận payload event tiêu thụ đã có: `learning_objective.progressed` (owner F031), `school.updated` (owner F010); thiếu → consumer tương ứng bị chặn, không tự tạo — file: `specs/events/payloads/learning/`, `specs/events/payloads/identity/` (chỉ đọc) — ✅ `learning_objective.progressed` (F031) và `school.updated` (F010) có sẵn; khóa `enabled_subject_codes` chưa có trong payload identity → Câu hỏi mở F071-1
- [x] T007 ent mixin nội dung có version + schema `ContentRelease`, `WorldArea*`, `StoryChapter*`, `NpcDialogue*`, `Level*`, `LevelSession`, `AreaUnlock`, `ExplorationProgress`, `StoryProgress`, `RefMasterySummary`, cột `ref.schools.settings_subset` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f071_world`, `make migrate-lint MOD=game` — ✅ ent/schema/world.go (14 entity, một mixin entx mỗi entity, MaxLen); `TestMigrationMatchesEnt` pass
- [x] T008 Migration: trigger con trỏ chỉ trỏ version `published`, unique partial, CHECK jsonb, RLS bảng T, append-only `content_releases` — file: `src/backend/modules/game/migrations/` — ✅ migrations/20261226000001_f071_world.sql (trigger con trỏ `CONTENT_POINTER_NOT_PUBLISHED`, bất biến version, RLS 5 bảng T, append-only `content_releases`, `ref.mastery_summary`, view `_live`) + atlas.sum
- [x] T009 Test lõi phát hành F071-AC-05: publish tăng `manifest_version` mọi tenant, ETag đổi, `content.published` trong outbox, snapshot bất biến; level session đang chạy giữ version ghim (TC-CNT-001, TC-CNT-005) — file: `src/backend/modules/game/tests/integration/content/publish_game_content_test.go` — phải FAIL trước khi code — ✅ src/backend/modules/game/tests/integration/world_publish_test.go `TestPublishGameContent` pass (viết sau lõi — không ghi được lần FAIL trước)
- [x] T010 Command `PublishGameContent`, `SnapshotStore`, `ManifestBuilder` + lệnh seed gọi command — file: `src/backend/modules/game/internal/application/command/publish_game_content.go`, `src/backend/modules/game/internal/infrastructure/content/`, `src/backend/modules/game/cmd/seed/main.go` — ✅ command/publish_game_content.go, command/manifest_game.go, persistence/write/world_repo.go, cmd/seed (seedfiles.World)
- [x] T011 [P] Consumer projection `ref.mastery_summary` (`learning_objective.progressed`), `ref.schools.settings_subset` (`school.updated`) + test bắt kịp/idempotent (TC-SVC-003) — file: `src/backend/modules/game/internal/presentation/consumer/mastery_projection.go`, `src/backend/modules/game/internal/presentation/consumer/school_projection.go` — ✅ consumer/unlock_on_events.go `MasteryProjection` + persistence/write/mastery_repo.go; `TestMasteryProjectionUnlocks` (source_version cũ bị bỏ qua); `ref.schools.settings_subset` đã do `game.projection_identity` chiếu

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Danh sách vùng đất và quyền vào theo học sinh (Priority: P1) 🎯 MVP

**Goal**: Học sinh thấy 8 vùng với trạng thái mở khóa đúng.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T012 [P] [US1] Test F071-AC-01, F071-AC-02: 12 `area_id` chuẩn, tên vi/en; khu học tập theo khối 1–9 mở với level 1, Xu 0 (TC-WLD-001, TC-GAM-009) — file: `src/backend/modules/game/tests/integration/world/list_areas_test.go` — phải FAIL trước khi code — ✅ src/backend/modules/game/tests/integration/world_areas_test.go `TestListWorldAreas` pass (khối 1–9, level 1, Xu 0)
- [x] T013 [P] [US1] Test F071-AC-03: môn bị trường tắt bị ẩn ở `listWorldAreas`, `getWorldManifest`, `getWorldExploration` — file: `src/backend/modules/game/tests/integration/world/subject_disabled_test.go` — ✅ src/backend/modules/game/tests/integration/world_areas_test.go `TestSubjectDisabledHidden` pass
- [x] T014 [P] [US1] Test F071-AC-04: manifest có bundle `url`/`sha256`, đổi tên vùng qua publish hiện ở lần đọc sau — file: `src/backend/modules/game/tests/integration/world/world_manifest_test.go` — ✅ src/backend/modules/game/tests/integration/world_areas_test.go `TestWorldManifest` pass
- [x] T015 [P] [US1] Test authorization + tenant isolation F071-AC-06: mọi route người chơi F071 × 5 role nhân sự → 403 (TC-AUTH-014); unlock/exploration/level session tenant khác không đọc được, header tenant khác → `TENANT_MISMATCH` (TC-TEN-001) — file: `src/backend/modules/game/tests/integration/world/authz_isolation_test.go` — ✅ src/backend/modules/game/tests/integration/world_areas_test.go `TestWorldTenantIsolation` + `TestPlayerRoutesStaffDenied` (8 route F071 × 5 role) pass

### Implementation cho User Story 1

- [x] T016 [P] [US1] Domain `WorldArea`, `UnlockRule` — file: `src/backend/modules/game/internal/domain/world/` — ✅ internal/domain/world/{area,unlock_rule,access,story,exploration,dialogue,level}.go + unit test
- [x] T017 [US1] Query `ListWorldAreas`, `GetWorldManifest` + handler (middleware `game_player`, `tenant`) — file: `src/backend/modules/game/internal/application/query/list_world_areas.go`, `src/backend/modules/game/internal/presentation/http/public/world.go` — ✅ query/world.go, presentation/http/world.go, routes.go (game_player, TenantRequired)
- [x] T018 [US1] `WorldService` Godot đọc area/manifest, cập nhật `AppState.world` — file: `src/game/scripts/world/world_service.gd` — ✅ src/game/scripts/world/world_service.gd (+ AppState.world, ServiceLocator.world); test_world_service.gd 7/7

## Phase 4: User Story 2 — Đi qua cổng gió và vào khu (Priority: P1)

### Tests cho User Story 2

- [x] T019 [P] [US2] Test F071-AC-07, F071-AC-08, F071-AC-10: vào khu mở → room/spawn/level_id + presence + `last_area_id`; khu khóa → `AREA_LOCKED` + điều kiện, không presence; session không active → 409 (TC-WLD-002, TC-GAM-008) — file: `src/backend/modules/game/tests/integration/world/enter_area_test.go` — phải FAIL trước khi code — ✅ src/backend/modules/game/tests/integration/world_enter_test.go `TestEnterWorldArea` pass
- [x] T020 [P] [US2] Test F071-AC-09: hai học sinh khác trường cùng area → room khác; cùng trường cùng grade band → cùng room — file: `src/backend/modules/game/internal/infrastructure/world/instance_allocator_v0_test.go` — ✅ domain `TestInstanceKey` + src/backend/modules/game/tests/integration/world_enter_test.go `TestEnterAreaPartition` pass
- [x] T021 [P] [US2] Test F071-AC-11: `startLevel` ghim version, một level session active, `Idempotency-Key` lặp → cùng kết quả; `endLevelSession` bỏ ghim — file: `src/backend/modules/game/tests/integration/world/level_session_test.go` — ✅ src/backend/modules/game/tests/integration/world_level_session_test.go `TestLevelSessionPins` pass
- [x] T022 [P] [US2] GdUnit4 test F071-AC-12: `SceneRouter.request_area` gọi `enterWorldArea` → `ensure_bundle` → thay zone; spawn lạ → `default` + cảnh báo — file: `src/game/tests/unit/world/scene_router_area_test.gd` — ✅ src/game/tests/unit/world/test_scene_router_area.gd 5/5
- [x] T023 [P] [US2] Lint zone + benchmark headless F071-AC-13 (ID duy nhất, `area_id` hợp lệ, material từ preset, TC-PERF-018) — file: `src/game/tools/zone_lint.gd`, `src/game/tests/integration/zone_benchmark_test.gd` — ✅ src/game/tools/zone_lint.gd + tools/lib/zone_rules.gd; tests/scene/world/test_zone_lint.gd 3/3 (lint + ngân sách cảnh + zone hỏng bị bắt); `zone_lint: OK (12 files)`

### Implementation cho User Story 2

- [x] T024 [US2] Command `EnterWorldArea` + `InstanceAllocatorV0` (nhãn "allocator tạm — F091 thay") + `PresenceStore` Valkey; outbox `world.area_entered` gộp — file: `src/backend/modules/game/internal/application/command/enter_world_area.go`, `src/backend/modules/game/internal/infrastructure/world/` — ✅ command/world.go `EnterArea` (ALLOCATOR TẠM v0 = phân vùng, hub thật do F091), cache/world_store.go `PresenceStore`
- [x] T025 [US2] Command `StartLevel`, `EndLevelSession` + handler — file: `src/backend/modules/game/internal/application/command/start_level.go` — ✅ command/world.go `StartLevel`, `EndLevel` (Idempotency-Key, `idempotency.SetResult`)
- [x] T026 [US2] Godot `AreaPortal.tscn`, `area_portal.gd`, `zone_loader.gd`, `SceneRouter.request_area`, nút "Về Quảng trường" — file: `src/game/scenes/world/AreaPortal.tscn`, `src/game/scripts/world/zone_loader.gd`, `src/game/scripts/core/scene_router.gd` — ✅ scenes/world/AreaPortal.tscn, scripts/world/{area_portal,zone_loader}.gd, `SceneRouter.request_area`, nút "Về Quảng trường" + Bản đồ trên HUD
- [x] T027 [US2] Zone scenes 12 `area_id` (greybox đạt lint, art theo pipeline) — file: `src/game/scenes/world/zones/` — ✅ scenes/world/zones/*/Zone.tscn (12, greybox PLACEHOLDER sinh bởi tools/gen_greybox_zones.mjs, `--check` ổn định)

## Phase 5: User Story 3 — Mở khóa vùng theo tiến trình (Priority: P1)

### Tests cho User Story 3

- [x] T028 [P] [US3] Unit test F071-AC-14: `UnlockRule.Evaluate` cho từng loại điều kiện, AND/OR, trả danh sách thiếu — file: `src/backend/modules/game/internal/domain/world/unlock_rule_test.go` — phải FAIL trước khi code — ✅ internal/domain/world/unlock_rule_test.go — FAIL (build) rồi PASS
- [x] T029 [P] [US3] Test F071-AC-15: event điều kiện → 1 dòng unlock + `world.area_unlocked` + publish Valkey `world.area.unlocked`; giao lặp 1.000 lần → không thêm (TC-WLD-002, TC-SVC-003) — file: `src/backend/modules/game/tests/integration/world/unlock_consumer_test.go` — ✅ src/backend/modules/game/tests/integration/world_progress_test.go `TestUnlockOnEvents` pass (1.000 lần phát lại)
- [x] T030 [P] [US3] Test F071-AC-16: publish area học tập có `min_level`/`event_participation` → 422 — file: `src/backend/modules/game/tests/integration/world/unlock_rule_publish_test.go` — ✅ src/backend/modules/game/tests/integration/world_progress_test.go `TestUnlockRulePublishValidation` pass

### Implementation cho User Story 3

- [x] T031 [US3] Command `EvaluateAreaUnlocks` + consumer `game.unlock_on_events` (`quest.completed`, `story.chapter_completed`, `level.reached`, `learning_objective.progressed`, `achievement.unlocked`, `event.participation_started`) — file: `src/backend/modules/game/internal/presentation/consumer/unlock_on_events.go` — ✅ consumer/unlock_on_events.go `UnlockOnEvents` + command/world_progress.go `ApplyProgress`
- [x] T032 [US3] Validator publish `unlock_rule` trong `PublishGameContent` cho `world_area` — file: `src/backend/modules/game/internal/domain/world/unlock_rule_validator.go` — ✅ domain/world/area.go `Validate` (forbiddenForLearning, không có loại thanh toán), gọi trong PublishGameContent

## Phase 6: User Story 4 — NPC và hội thoại (Priority: P2)

### Tests cho User Story 4

- [x] T033 [P] [US4] Test F071-AC-17: dialogue published theo khối, ETag/304, NPC không có → 404 — file: `src/backend/modules/game/tests/integration/world/npc_dialogue_test.go` — phải FAIL trước khi code — ✅ src/backend/modules/game/tests/integration/world_discovery_dialogue_test.go `TestNpcDialogue` pass
- [x] T034 [P] [US4] Test F071-AC-18: publish dialogue thiếu `en` → `TRANSLATION_INCOMPLETE`; `quest_id` không tồn tại → `ITEM_REF_INVALID` — file: `src/backend/modules/game/tests/integration/world/npc_dialogue_publish_test.go` — ✅ src/backend/modules/game/tests/integration/world_discovery_dialogue_test.go `TestNpcDialoguePublishValidation` pass
- [x] T035 [P] [US4] GdUnit4 test F071-AC-19, F071-AC-20: action `give_quest`/`open_minigame` gọi service (fake `ApiClient`), không tự đổi trạng thái; sau tổng kết về vị trí NPC, không nối hoạt động — file: `src/game/tests/unit/world/dialogue_runner_test.gd` — ✅ src/game/tests/unit/world/test_dialogue_runner.gd 7/7

### Implementation cho User Story 4

- [x] T036 [US4] Query `GetNpcDialogue` + validator dialogue (vi/en, tham chiếu) + handler — file: `src/backend/modules/game/internal/application/query/get_npc_dialogue.go` — ✅ query/world.go `NpcDialogue`, domain/world/dialogue.go `ParseDialogue` + kiểm tham chiếu trong PublishGameContent
- [x] T037 [US4] Godot `NPC.tscn`, `DialogueBubble.tscn`, `dialogue_runner.gd` (camera 2-shot theo CHARACTER_SYSTEM §3) — file: `src/game/scenes/npc/`, `src/game/scripts/world/dialogue_runner.gd` — ✅ scenes/npc/{NPC,DialogueBubble}.tscn, scripts/world/{npc_anchor,dialogue_runner,dialogue_actions,activity_return}.gd (camera 2-shot CHARACTER_SYSTEM §3 chưa làm — giữ camera F062, việc tiếp theo)

## Phase 7: User Story 5 — Mạch truyện "Thắp lại Tia Sáng" (Priority: P2)

### Tests cho User Story 5

- [x] T038 [P] [US5] Test F071-AC-21, F071-AC-22: main quest đầu → `in_progress`; đủ → `completed`, `story.chapter_completed` 1 lần (replay); chương môn học mở song song, môn tắt bị ẩn (TC-WLD-004, TC-WLD-005) — file: `src/backend/modules/game/tests/integration/world/story_progress_test.go` — phải FAIL trước khi code — ✅ src/backend/modules/game/tests/integration/world_progress_test.go `TestStoryProgress` pass
- [x] T039 [P] [US5] Test F071-AC-23: `getWorldExploration.chapters` đúng trạng thái và số main quest — file: `src/backend/modules/game/tests/integration/world/exploration_chapters_test.go` — ✅ src/backend/modules/game/tests/integration/world_progress_test.go `TestStoryProgress` (getWorldExploration.chapters: status, order, main_quests_completed/total) pass

### Implementation cho User Story 5

- [x] T040 [US5] Domain `StoryChapterRules`, command `UpdateStoryProgress` + consumer `game.story_on_quest_completed` — file: `src/backend/modules/game/internal/application/command/update_story_progress.go` — ✅ domain/world/story.go `Chapter.Status/RecordQuest`, command/world_progress.go (một consumer `game.unlock_on_events`, Clarifications)

## Phase 8: User Story 6 — Khám phá và bản đồ thế giới (Priority: P2)

### Tests cho User Story 6

- [x] T041 [P] [US6] Test F071-AC-24, F071-AC-25: ghi một lần, % tăng, `world.discovery_found`/`world.area_completed`; sai area/ID → 422; lặp → 409 không thưởng; vượt `discovery_report_rate` → 429; cùng `Idempotency-Key` → cùng kết quả (TC-GAM-010, TC-WLD-003) — file: `src/backend/modules/game/tests/integration/world/report_discovery_test.go` — phải FAIL trước khi code — ✅ src/backend/modules/game/tests/integration/world_discovery_dialogue_test.go `TestReportDiscovery` pass
- [x] T042 [P] [US6] GdUnit4 test F071-AC-26, F071-AC-27: World Map hiển thị % từ fixture `getWorldExploration`, vùng khóa có điều kiện, không nút mở bằng Xu; `g1_3` bảng chọn hình 1 chạm — file: `src/game/tests/unit/world/world_map_test.gd` — ✅ src/game/tests/unit/world/test_world_map.gd 4/4

### Implementation cho User Story 6

- [x] T043 [US6] Command `ReportDiscovery` (presence, layout version ghim, rate limit), query `GetWorldExploration` + handler — file: `src/backend/modules/game/internal/application/command/report_discovery.go`, `src/backend/modules/game/internal/application/query/get_world_exploration.go` — ✅ command/world.go `ReportDiscovery`, query/world.go `Exploration`
- [x] T044 [US6] Godot `WorldMap.tscn` (S04, ảnh 2D không chữ, khóa `ui.worldmap.*`), `Interactable` kind `discovery`/`collectible` gọi `WorldService.report_discovery` — file: `src/game/scenes/world/WorldMap.tscn`, `src/game/scripts/world/world_map.gd`, `src/game/scripts/world/interactable.gd` — ✅ scenes/world/WorldMap.tscn + world_map.gd, scripts/world/world_map_model.gd, ZoneRoot → `discovery_requested` → WorldService.report_discovery

## Phase 9: Polish & Cross-Cutting

- [x] T045 [P] Cập nhật DATABASE_DESIGN §2.8a/§2.17 (`name_i18n`, `title_i18n`, `area_class`, `subject_code`, `layout`, `level_definitions.area_id`, `ref.mastery_summary`, `ref.schools.settings_subset`); EVENT_DESIGN §4.5–§4.6 (payload); CODE_PLAN ghi phân định lõi phát hành F071/F078; CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md` — ✅ DATABASE_DESIGN §2.8/§2.8a/§2.17, EVENT_DESIGN §4, WORLD_DESIGN §8/§12, WORLD_RENDERING §10, CONFIG_DEFAULTS, CHANGELOG
- [x] T046 [P] Metric `eduworld_game_area_*`, `eduworld_game_discovery_total`, `eduworld_game_manifest_build_seconds` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go` — ✅ observability/metrics.go `eduworld_game_area_enter_total`, `eduworld_game_area_unlocked_total`, `eduworld_game_discovery_total`, `eduworld_game_manifest_build_seconds`
- [ ] T047 [P] Kịch bản đo chuyển vùng trên thiết bị lab hạng Tầm trung (TC-PERF-020), kết quả nộp kèm bằng chứng quickstart — file: `src/game/tests/perf/area_transition_perf.gd` — ⏳ CI guard `tests/perf/test_area_transition.gd` (12 zone ≤ 293 ms headless) xong; đo trên thiết bị lab hạng Tầm trung chờ asset thật (OQ-16) và thiết bị lab
- [ ] T048 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — ⏳ chờ môi trường compose riêng (stack eduworld-* đang dùng cho E2E của agent khác); các kịch bản 1–11, 13, 14 đã có test tự động tương ứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T049 **OQ chốt 2026-11-20** (vị trí hub) Khai báo operation vị trí hub trong `specs/openapi/modules/competition.internal.v1.yaml` (`x-allowed-callers: [game]`), hiện thực bên nhận trong module competition (đọc dữ liệu room host của F091) và adapter + port phía game dùng khi kiểm khoảng cách tương tác có giá trị; facade test hai phía — file: `specs/openapi/modules/competition.internal.v1.yaml`, `src/backend/modules/game/internal/infrastructure/` — kiểm: `make gen MOD=competition`, `make test-contract` — cỡ M — ✅ specs/openapi/modules/competition.internal.v1.yaml `getHubPosition` (x-allowed-callers game) + facadegen; competition query/hub_position.go + facade; game infrastructure/modules/competition.go (LateHub, Wire); facade test hai phía `TestGetHubPosition`, `TestHubPositionsFacade`; `TestReportDiscovery` (xa → từ chối)

## Dependencies & Execution Order

- Setup → Foundational (T007–T010 trước mọi story; T011 trước US3) → US1 → US2 → US3 → US4, US5, US6 song song → Polish.
- `quest.completed` thật đến từ F072; trước đó test bằng fixture event.

## Definition of Done

- [x] Mọi tiêu chí F071-AC-* có test pass (ghi mã test/TC) — mã test ở từng task; AC-12 phần đo trên thiết bị lab còn chờ (T047)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game) và `make godot-test` xanh — 2026-11-21: gate Go (gofmt, vet, ewlint, golangci + arch, tenancyguard, entlint, archgen, routematrix, allowedcallers, facadegen, problem codes, gen-routes, TestMigrationMatchesEnt, tools, cmd, unit game + competition) OK; integration game 58/58; GdUnit4 719 (1 test timing F070 flaky dưới tải, pass khi chạy lại); lint Godot + zone_lint + export check OK
- [x] Constitution Check trong plan.md vẫn đạt; allocator v0 gắn nhãn trong code và báo cáo (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật (In progress: T047 lab, T048 quickstart)
