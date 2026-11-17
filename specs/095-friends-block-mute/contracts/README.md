# Contracts: F095 — Bạn bè, block, mute (module social)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → sinh `specs/openapi/modules/chat.v1.yaml`. Mọi route: `game_player` (chỉ học sinh), `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `sendFriendRequest` | `POST /api/v1/social/friend-requests` (`Idempotency-Key`, body `addressee_id`) → `201 FriendRequest` | chat.v1.yaml | `game_player` | Cần sửa — thêm `404`, `409 FRIEND_LIMIT_REACHED`/`CONFLICT`, `422` |
| `listFriendRequests` | `GET /api/v1/social/friend-requests?direction=` | chat.v1.yaml | `game_player` (của mình) | Cần sửa — mô tả: lời mời bị từ chối hiển thị `pending` với người gửi |
| `acceptFriendRequest` | `POST /api/v1/social/friend-requests/{request_id}:accept` (`Idempotency-Key`) → `204` | chat.v1.yaml | `game_player` (addressee) | Có sẵn |
| `rejectFriendRequest` | `POST /api/v1/social/friend-requests/{request_id}:reject` → `204` | chat.v1.yaml | `game_player` (addressee) | Có sẵn |
| `listFriends` | `GET /api/v1/social/friends` → `PlayerCard[]` + `page` | chat.v1.yaml | `game_player` | Cần sửa — enum `PlayerCard.presence` thống nhất (F091 delta) |
| `removeFriend` | `DELETE /api/v1/social/friends/{user_id}` → `204` | chat.v1.yaml | `game_player` | Có sẵn |
| `createFriendCode` | `POST /api/v1/social/friend-codes` (`Idempotency-Key`) → `201 FriendCode` | chat.v1.yaml | `game_player` | Có sẵn |
| `redeemFriendCode` | `POST /api/v1/social/friend-codes:redeem` (`Idempotency-Key`, `code`) → `201 FriendRequest` | chat.v1.yaml | `game_player` | Cần sửa — `404 FRIEND_CODE_INVALID` thay `410 Gone` (API_DESIGN §7), thêm `409`, `422` |
| `blockUser` / `unblockUser` | `PUT` / `DELETE /api/v1/social/blocks/{user_id}` → `204` | chat.v1.yaml | `game_player` | Cần sửa — thêm `403`, `422` |
| `listBlocks` | `GET /api/v1/social/blocks` → `PlayerRelation[]` | chat.v1.yaml | `game_player` | Có sẵn |
| `muteUser` / `unmuteUser` | `PUT` / `DELETE /api/v1/social/mutes/{user_id}` → `204` | chat.v1.yaml | `game_player` | Cần sửa — thêm `403`, `422` |
| `listMutes` | `GET /api/v1/social/mutes` → `PlayerRelation[]` | chat.v1.yaml | `game_player` | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `cleanupStudentData` / `restoreStudentData` | identity → chat (saga dọn học sinh): hủy lời mời đang chờ | `specs/openapi/modules/chat.internal.v1.yaml` | có sẵn | Có sẵn (hiện thực phần social ở F016/F095) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `presence.friend.updated` | server → client (chỉ bạn bè) | Có sẵn (enum chung — delta F091) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `friend_request.sent` | chat → NTF (F123), AUD | `specs/events/payloads/chat/friend_request.sent.v1.schema.json` | Cần thêm |
| `friend.added` / `friend.removed` | chat → competition (`ref.friendships`, F091), NTF | `friend.added.v1`, `friend.removed.v1` | Cần thêm |
| `user.blocked` / `user.unblocked` | chat → competition (`ref.blocks`, F091/F100/F104), chat (F115), AUD | `user.blocked.v1`, `user.unblocked.v1` | Cần thêm |
| `user.muted` / `user.unmuted` | chat → chat (F115), AUD | `user.muted.v1`, `user.unmuted.v1` | Cần thêm |
| `tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up/restored` | identity → chat (projection) | `specs/events/payloads/identity/` | Có sẵn (áp 2026-11-17, owner F010/F011/F016) |
| `game_profile.updated` | game → chat (`ref.player_profiles`) | `specs/events/payloads/game/game_profile.updated.v1.schema.json` | Cần thêm (owner F070) |
| `ew.rt.presence.<tenant_id>` (NATS core) | gateway → chat | `specs/schemas/realtime/presence-changed.schema.json` | Cần thêm (F091) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- `eduworld-api.v1.yaml` (social): bổ sung response lỗi như bảng §1; `redeemFriendCode` dùng `404 FRIEND_CODE_INVALID` (bỏ `410`); mô tả từ chối lặng lẽ cho `listFriendRequests`; thống nhất `maxLength` `nickname` = 32 cho `PlayerCard`, `PlayerRelation` và `PublicPlayer` AsyncAPI theo DATABASE_DESIGN §1.4 (`varchar(32)`, RB-302).
- Payload schema 7 event chat (`specs/events/payloads/chat/`), cập nhật `specs/events/README.md`.
- Kiểm: `make specs-lint`, `make gen MOD=chat`, `make test-contract`.
- ADR-0078: `sendFriendRequest`, mã bạn bè khai báo `403 SOCIAL_SCOPE_VIOLATION` cho tenant `common`; bootstrap/client-config báo `social.friends_enabled = false` [Proposed] (task T029).
