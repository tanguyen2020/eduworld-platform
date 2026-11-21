# Contracts: F118 — Bộ tính năng nhắn tin đầy đủ (sửa/thu hồi, reply/forward, mention, receipts) và sticker/GIF

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/chat.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `editMessage` | `PATCH /api/v1/chat/messages/{message_id}` (`If-Match`; `409`, `412`, `451`, `503`) | chat.v1.yaml | `game_player` + người gửi | Có sẵn |
| `recallMessage` | `POST /api/v1/chat/messages/{message_id}:recall` | chat.v1.yaml | `game_player` + người gửi | Có sẵn |
| `forwardMessage` | `POST /api/v1/chat/messages/{message_id}:forward` (`Idempotency-Key`, `ForwardRequest`) | chat.v1.yaml | `game_player` + member nguồn và đích | Cần sửa (thêm `428`) |
| `pinMessage` / `unpinMessage` | `PUT`/`DELETE /api/v1/chat/channels/{channel_id}/pins/{message_id}` | chat.v1.yaml | `game_player` + quyền pin theo loại channel | Có sẵn |
| `listChannelPins` | `GET /api/v1/chat/channels/{channel_id}/pins` | chat.v1.yaml | `game_player` + member | Có sẵn |
| `getMessageReceipts` | `GET /api/v1/chat/channels/{channel_id}/messages/{message_id}/receipts` | chat.v1.yaml | `game_player` + member | Có sẵn |
| `listStickerPacks` | `GET /api/v1/chat/stickers` | chat.v1.yaml | `game_player` | Có sẵn |
| `listGifs` | `GET /api/v1/chat/gifs?tag=` | chat.v1.yaml | `game_player` | Có sẵn |
| `toggleMessageReaction` | `POST /api/v1/chat/messages/{message_id}/reactions` (REST fallback của `chat.reaction.toggle`, API_DESIGN §9.9) | chat.v1.yaml | `game_player` + member | Cần thêm |
| `listGameConfigs_chat`, `createGameConfig_chat`, `getGameConfigVersion_chat`, `updateGameConfigDraft_chat`, `transitionGameConfigVersion_chat`, `listGameConfigVersions_chat`, `diffGameConfigVersions_chat`, `publishGameConfigVersion_chat`, `rollbackGameConfig_chat`, `unpublishGameConfig_chat`, `listGameConfigReleases_chat` | `/api/v1/game-admin/{config_type}…` với `config_type ∈ {sticker-packs, gifs}` | chat.v1.yaml | `game.chat_assets.write` (gateway nội bộ, role platform) | Có sẵn |
| Presign upload asset chat | `POST /api/v1/game-admin/{config_type}/{config_id}/assets:presign` (đề xuất) | chat.v1.yaml | `game.chat_assets.write` | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `executeTenantCloneStep` | identity → chat (gán/clone sticker pack, GIF cho tenant) | chat.internal.v1.yaml | `identity` | Có sẵn (F045) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `chat.message.edit` / `chat.message.edited` | client → server / server → member | Có sẵn |
| `chat.message.recall` / `chat.message.recalled` | client → server / server → member | Có sẵn |
| `chat.message.forward` | client → server | Có sẵn |
| `chat.reaction.toggle` / `chat.reaction.updated` | client ↔ server | Có sẵn (payload `reactions` khác CHAT_SYSTEM `counts, my_reactions` → giữ AsyncAPI) |
| `chat.pin.add` / `chat.pin.remove` / `chat.pin.updated` | client ↔ server | Có sẵn |
| `chat.typing.start` / `chat.typing.stop` / `chat.typing.updated` | client ↔ server | Có sẵn |
| `chat.receipt.delivered` / `chat.receipt.read` / `chat.receipt.updated` | client ↔ server | Có sẵn |
| `chat.mention.created`, `chat.unread.updated` | server → người dùng | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| JetStream `ew.chat.<tenant_id>.evt.<channel_id>` | module chat → persister, moderation, stats | `specs/events/payloads/chat/chat-evt.v1.schema.json` | Cần thêm |
| `chat.message_edited` / `_recalled` / `_forwarded` | persister → AUD, MOD | `specs/events/payloads/chat/chat.message_edited.v1.schema.json` (+ `recalled`, `forwarded`) | Cần thêm |
| `chat.message_pinned` / `_unpinned` | persister → AUD | `specs/events/payloads/chat/chat.message_pinned.v1.schema.json` (+ `unpinned`) | Cần thêm |
| `content.published` (`sticker_pack`, `gif`) | chat → snapshot/CDN worker, AUD | `specs/events/payloads/chat/content.published.v1.schema.json` | Cần thêm |
| `party.leader_transferred` | competition → chat (role leader) | `specs/events/payloads/competition/party.leader_transferred.v1.schema.json` | Cần thêm (owner F100) |
| `item.granted` (mở khóa sticker theo vật phẩm) | game (F075) → chat (`ref.sticker_unlocks`) | `specs/events/payloads/game/item.granted.v1.schema.json` | Cần thêm (owner F075) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/chat-policy.schema.json` (`features`) | Bật/tắt tính năng theo loại channel | Có sẵn |

## 6. Delta cần thực hiện

- OpenAPI: thêm `toggleMessageReaction` (`POST /chat/messages/{message_id}/reactions`, body `{reaction}` maxLength 64, `Idempotency-Key`); thêm `ProblemCode` `CHAT_REACTION_LIMIT` (422) cho vượt `chat_reactions_per_user` (AsyncAPI `ErrorCode` cũng thêm); `forwardMessage` thêm `428`; `ForwardRequest.target_channel_ids` `maxItems` khớp `chat_forward_targets_max`.
- OpenAPI: endpoint presign upload asset chat cho Game Studio (chưa có trong `chat.v1.yaml`).
- Event payload chat (§4) và phối hợp F100/F075 cho `party.leader_transferred`, event mở khóa vật phẩm.
- API_DESIGN §5: `CHAT_MENTION_LIMIT` mô tả cả "mention người không phải thành viên/đã block" → sửa theo CHAT_SYSTEM §7.7 (loại khỏi `mentions`).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`, `make test-contract`.
