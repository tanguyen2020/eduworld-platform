# Contracts: F091 — Hub instance, presence, đồng bộ vị trí

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| — | F091 không thêm endpoint REST (FR-MP-006: không REST cho chuyển động) | — | — | — |
| `enterWorldArea` | `POST /api/v1/game/world/areas/{area_id}:enter` → `AreaEnterResult` | `specs/openapi/modules/game.v1.yaml` | `game_player` | Cần sửa (cùng F071): gán instance thuộc module competition — bỏ `room_id` khỏi response hoặc chuyển endpoint |
| `joinFriendInWorld` | `POST /api/v1/game/world/friends/{user_id}:join` | `specs/openapi/modules/game.v1.yaml` | `game_player` | Cần sửa (cùng F071): thay bằng `hub.instance.join {follow_user_id}` |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không gọi HTTP đồng bộ; dữ liệu qua projection (research R2) | — | — | — |

NATS (lệnh từ gateway, F090):

| Subject | Hướng | Trạng thái |
|---|---|---|
| `ew.cmd.competition.hub.instance.join` | gateway → competition (request/reply, reply có `subscribe: ["ew:rt:room:<id>:out"]`) | Cần thêm vào `x-routing` AsyncAPI |
| `ew.cmd.competition.hub.player.emote` | gateway → competition | Cần thêm vào `x-routing` |
| `ew.in.room.<room_id>` | gateway → room host (`hub.player.move`) | Có sẵn |
| `ew.rt.presence.<tenant_id>` | gateway → module chat (F095), NATS core | Cần thêm |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `hub.instance.join` | client → server | Cần sửa: thêm `follow_user_id` (uuid, tùy chọn) |
| `hub.player.move` | client → server (≤ 10 Hz, không ack) | Có sẵn |
| `hub.player.emote` | client → server | Có sẵn |
| `hub.instance.assigned` | server → client | Có sẵn |
| `hub.snapshot.updated` | server → client (droppable) | Có sẵn |
| `hub.player.joined` / `hub.player.left` / `hub.player.emoted` / `hub.player.appearance_changed` / `hub.player.corrected` | server → client | Có sẵn |
| `hub.instance.migrated` | server → client | Có sẵn |
| `presence.status.set` | client → server (gateway) | Có sẵn |
| `ErrorCode` `ROOM_FULL`, `ROOM_NOT_MEMBER`, `PERMISSION_DENIED`, `NOT_FOUND`, `RATE_LIMITED`, `VALIDATION_FAILED` | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `content.published`, `content.unpublished` (`item_type = world_area`) | game (F071/F078) → competition (`ref.world_areas`) | `specs/events/payloads/game/content.published.v1.schema.json` (khối `world_area`) | Cần thêm (owner F071; `content.unpublished` owner F078) |
| `world.area_unlocked` | game → competition (`ref.area_unlocks`) | `specs/events/payloads/game/world.area_unlocked.v1.schema.json` | Cần thêm |
| `game_profile.updated` | game (F070) → competition (`ref.player_profiles`, appearance) | `specs/events/payloads/game/game_profile.updated.v1.schema.json` (có `avatar_config`) | Cần thêm |
| `friend.added` / `friend.removed`, `user.blocked` / `user.unblocked` | chat (F095) → competition | `specs/events/payloads/chat/*.v1.schema.json` | Cần thêm (F095) |
| `tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up/restored`, `user.access_revoked` | identity → competition | `specs/events/payloads/identity/` | Cần thêm (F010, F011, F016) |
| `audit.recorded` | competition → identity | schema audit chung | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/realtime/presence-changed.schema.json` | Payload `ew.rt.presence.<tenant_id>` | Cần thêm |
| `specs/schemas/realtime/rt-pubsub-envelope.schema.json` | Control `presence.update`, gói snapshot theo node | Cần thêm (F090) — bổ sung action `presence.update` |

## 6. Delta cần thực hiện

- AsyncAPI: thêm `follow_user_id` vào `HubInstanceJoin`; sửa `info.description` (bảng domain → module: `hub.*` → module competition); `x-routing` cho `hub.*`; thống nhất enum presence (`PresenceStatusSet`, `PresenceFriendUpdated`) với `PlayerCard.presence` theo MULTIPLAYER_GAMEPLAY §5 (`offline`, `online`, `in_activity`, `in_match`, `away`; bỏ `busy`, `invisible`; thêm cờ `hidden` boolean vào `PresenceStatusSet`) qua `specs/schemas/common/enums.schema.json`; `PublicPlayer.nickname` `maxLength` 32.
- OpenAPI game: `enterWorldArea`, `joinFriendInWorld` — quyết định cùng F071 (bỏ hoặc chỉ kiểm quyền area, không trả `room_id`).
- Payload tiêu thụ (owner khác, F091 không tạo): khối `world_area` trong `content.published` của game (owner F071), `world.area_unlocked` (F071), `game_profile.updated` có `avatar_config` (F070).
- Schema `presence-changed.schema.json`; mở rộng `rt-pubsub-envelope` action `presence.update`.
- Kiểm: `make specs-lint`, `make gen MOD=competition`, `make test-contract`.
