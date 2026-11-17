# Contracts: F070 — Hồ sơ người chơi, nickname, tạo avatar lần đầu, trang phục, emote

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `specs/openapi/modules/game.v1.yaml`). Mọi route: `game_player` (học sinh), header `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getGameProfile` | `GET /api/v1/game/profile` | eduworld-api.v1.yaml | `game_player` | Cần sửa (`GameProfile`: `nickname` nullable, thêm `setup_required`, `avatar_created_at`, `avatar_config_hash`) |
| `updateGameProfile` | `PATCH /api/v1/game/profile` (`nickname_option_id`, `settings`, `preferred_language`) | eduworld-api.v1.yaml | `game_player` | Cần sửa (`additionalProperties: false`; response `409` `NICKNAME_UNAVAILABLE`, `NICKNAME_CHANGE_LIMIT`) |
| `suggestNicknames` | `GET /api/v1/game/profile/nickname-suggestions?count=` | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `updateAvatar` | `PUT /api/v1/game/profile/avatar` | eduworld-api.v1.yaml | `game_player` | Cần sửa (`AvatarConfig` theo CHARACTER_SYSTEM §5.1) |
| `listAvatarPresets` | `GET /api/v1/game/avatar-presets` | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `saveAvatarPreset` | `PUT /api/v1/game/avatar-presets/{slot}` | eduworld-api.v1.yaml | `game_player` | Có sẵn (dùng `AvatarConfig` đã sửa) |
| `getCharacterCatalog` | `GET /api/v1/game/character/catalog` (`ETag`) | eduworld-api.v1.yaml | `game_player` | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `hub.player.emote`, `hub.player.emoted`, `hub.player.appearance_changed` | Không dùng ở F070 (F091/F092) | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `avatar.created` | game → achievement (F075), analytics | `specs/events/payloads/game/avatar.created.v1.schema.json` | Cần thêm |
| `avatar.updated` | game → analytics; realtime fan-out (F092) | `specs/events/payloads/game/avatar.updated.v1.schema.json` | Cần thêm |
| `game_profile.updated` | game → competition, chat (`ref.player_profiles`) | `specs/events/payloads/game/game_profile.updated.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/character-catalog.schema.json` | Payload `CharacterCatalogVersion` (preset, dáng, bảng màu, item mặc định, bộ từ nickname vi/en; mọi chuỗi có `maxLength`) | Cần thêm |
| `specs/schemas/avatar-config.schema.json` | Shape `AvatarConfig` dùng chung OpenAPI/AsyncAPI (`schema_version`, `body_type`, `skin_tone`, `hair{item_id,color}`, `outfit{item_id}`, `accessories[{slot,item_id}]` ≤ 4, `emote_slots[]` ≤ `emote_wheel_slots`, `pet`, `title`, `frame`, `badge_showcase[]` ≤ 3) | Cần thêm |

## 6. Delta cần thực hiện

- `eduworld-api.v1.yaml`: `AvatarConfig` → `$ref` `avatar-config.schema.json` (bỏ `body` 1000 ký tự, `hair_item_id` UUID, `emote_item_ids` ≤ 8); `GameProfile` thêm `setup_required`, `avatar_created_at`, `avatar_config_hash`, `nickname` nullable; `GameProfileUpdate` `additionalProperties: false`.
- Mọi field `nickname` (`GameProfile`, `PlayerCard`, payload `game_profile.updated`, …): `maxLength: 32` thay 128/64 (RB-302, CHARACTER_SYSTEM §5.3). Task T004.
- Thêm `getCharacterCatalog` (`GET /game/character/catalog`, response `CharacterCatalog` + `ETag`/`304`).
- `ProblemCode` thêm `NICKNAME_UNAVAILABLE`, `NICKNAME_CHANGE_LIMIT`.
- Payload event `specs/events/payloads/game/`: `avatar.created`, `avatar.updated`, `game_profile.updated` (thêm `game_profile.updated` vào catalog EVENT_DESIGN §4.5).
- JSON Schema mới: `character-catalog.schema.json`, `avatar-config.schema.json`.
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make test-contract`.
