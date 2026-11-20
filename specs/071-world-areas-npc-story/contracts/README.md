# Contracts: F071 — Thế giới: 8 vùng đất, zone, world map, cổng, mở khóa, mạch truyện, NPC và hội thoại

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `specs/openapi/modules/game.v1.yaml`). Mọi route `game_player`, header `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getWorldManifest` | `GET /api/v1/game/world/manifest` | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `listWorldAreas` | `GET /api/v1/game/world/areas` | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `enterWorldArea` | `POST /api/v1/game/world/areas/{area_id}:enter` (`game_session_id`, `portal_id`) | eduworld-api.v1.yaml | `game_player` | Cần sửa (`AreaEnterResult.level_id`; `403` `AREA_LOCKED` có extension `missing_requirements[]`; `409`) |
| `startLevel` | `POST /api/v1/game/levels/{level_id}:start` (`Idempotency-Key`) | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `endLevelSession` | `POST /api/v1/game/level-sessions/{level_session_id}:end` (`Idempotency-Key`) | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `getNpcDialogue` | `GET /api/v1/game/world/npcs/{npc_id}/dialogue` | eduworld-api.v1.yaml | `game_player` | Cần sửa (response `$ref` npc-dialogue delivery thay `type: object`; `ETag`/`304`) |
| `getWorldExploration` | `GET /api/v1/game/world/exploration` | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `reportDiscovery` | `POST /api/v1/game/world/areas/{area_id}/discoveries/{interactable_id}:discover` (`Idempotency-Key`) | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `getContentManifest` | `GET /api/v1/game/content-manifest` | eduworld-api.v1.yaml | `game_player` | Có sẵn (F014; F071 ghi dữ liệu manifest) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (mastery, quest, level qua event) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `world.area.unlocked` | server → học sinh (Valkey `ew:rt:user:<id>`) | Có sẵn |
| `content.manifest.updated` | server → client (đặt cờ có bản mới) | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `content.published` | game → manifest/snapshot worker, AUD | `specs/events/payloads/game/content.published.v1.schema.json` | Cần thêm |
| `world.area_entered` | game → analytics | `specs/events/payloads/game/world.area_entered.v1.schema.json` | Cần thêm |
| `world.area_unlocked` | game → RT, NTF, quest | `specs/events/payloads/game/world.area_unlocked.v1.schema.json` | Cần thêm |
| `world.discovery_found` | game → quest (F072), achievement, reward | `specs/events/payloads/game/world.discovery_found.v1.schema.json` | Cần thêm |
| `world.area_completed` | game → achievement, reward | `specs/events/payloads/game/world.area_completed.v1.schema.json` | Cần thêm |
| `story.chapter_completed` | game → achievement, reward, RT, analytics; game (unlock) | `specs/events/payloads/game/story.chapter_completed.v1.schema.json` | Cần thêm |
| `quest.completed` | game (F072) → game (unlock, story) | `specs/events/payloads/game/quest.completed.v1.schema.json` | Cần thêm (F072) |
| `level.reached`, `achievement.unlocked`, `event.participation_started` | game (F074/F075/F077) → game (unlock) | `specs/events/payloads/game/` | Cần thêm (feature sở hữu) |
| `learning_objective.progressed` | learning → game (`ref.mastery_summary`) | `specs/events/payloads/learning/learning_objective.progressed.v1.schema.json` | Cần thêm |
| `school.updated` | identity → game (`ref.schools.settings_subset`) | `specs/events/payloads/identity/school.updated.v1.schema.json` | Có sẵn (áp 2026-11-17, owner F010) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/npc-dialogue.schema.json` | NPC dialogue version | Có sẵn |
| `specs/schemas/world-area.schema.json` | `WorldAreaVersion` (`area_class`, `subject_code`, `unlock_rule`, `layout`, `name {vi,en}`) | Cần thêm |
| `specs/schemas/story-chapter.schema.json` | `StoryChapterVersion` | Cần thêm |
| `specs/schemas/level-package.schema.json` | `LevelVersion` (bundle, sha256, platforms) | Cần thêm |

## 6. Delta cần thực hiện

- `eduworld-api.v1.yaml`: `AreaEnterResult` thêm `level_id`; `enterWorldArea` khai báo `403 AREA_LOCKED` (Problem extension `missing_requirements[]` cùng shape `AreaExploration.unlock_requirements`, thêm `kind = chapters_completed`) và `409`; `getNpcDialogue` response `$ref` delivery schema + `ETag`.
- `AreaExploration.unlock_requirements[].kind` thêm `chapters_completed` (khớp AsyncAPI `world.area.unlocked.unlocked_by`).
- JSON Schema mới: `world-area`, `story-chapter`, `level-package` (mọi chuỗi có `maxLength`); `GameConfigType` đã có `world-areas`, `story-chapters`, `npc-dialogues`; `levels` đổi thành `progression-levels` (bảng level F074, RB-308) nên **Câu hỏi mở**: config type gói level (`game.level_definitions`) chưa có trong API_DESIGN — tạm `level-packages` [Confirmed], chờ tài liệu sở hữu.
- Gỡ `joinFriendInWorld` (`POST /game/world/friends/{user_id}:join`) khỏi `game.v1.yaml` và spec gộp: API_DESIGN ghi "Không dùng", "Tham gia bạn" là WS `hub.instance.join {follow_user_id}` (F091, RB-625). Task T004.
- Payload event game: `content.published`, `world.*`, `story.chapter_completed`; payload `learning_objective.progressed` (learning), `school.updated` (identity) nếu feature sở hữu chưa thêm.
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make test-contract`.
