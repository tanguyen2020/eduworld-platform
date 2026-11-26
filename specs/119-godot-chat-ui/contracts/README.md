# Contracts: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Client gọi qua `ApiClient` (token game, `X-Tenant-Id` của phiên, `Accept-Language`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listMyChannels`, `listChannelMessages` | `GET /api/v1/chat/channels[/{channel_id}/messages]` | chat.v1.yaml | `game_player` | Có sẵn (F115) |
| `listQuickMessages` | `GET /api/v1/chat/quick-messages?channel_type=` | chat.v1.yaml | `game_player` | Có sẵn (F116) |
| `listStickerPacks`, `listGifs` | `GET /api/v1/chat/stickers`, `GET /api/v1/chat/gifs?tag=` | chat.v1.yaml | `game_player` | Có sẵn (F118) |
| `listChannelPins`, `getMessageReceipts` | `GET …/pins`, `GET …/receipts` | chat.v1.yaml | `game_player` | Có sẵn (F118) |
| `editMessage`, `recallMessage`, `forwardMessage`, `pinMessage`, `unpinMessage` | REST fallback | chat.v1.yaml | `game_player` | Có sẵn (F118) |
| `toggleMessageReaction` | `POST /api/v1/chat/messages/{message_id}/reactions` | chat.v1.yaml | `game_player` | Có sẵn (F118); client dùng lệnh WS `chat.reaction.toggle` |
| `createReport` | `POST /api/v1/chat/reports` (`Idempotency-Key`, `ReportCreate`) | chat.v1.yaml | học sinh | Có sẵn (backend F121) |
| `blockUser`, `muteUser` | `PUT /api/v1/social/blocks/{user_id}`, `PUT /api/v1/social/mutes/{user_id}` | chat.v1.yaml | học sinh | Có sẵn (backend F095) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (client không gọi facade nội bộ giữa module) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `chat.channel.join/leave`, `chat.message.send/edit/recall/forward`, `chat.reaction.toggle`, `chat.pin.add/remove`, `chat.typing.start/stop`, `chat.receipt.delivered/read`, `chat.channel.sync`, `chat.history.fetch` | client → server | Có sẵn |
| `chat.message.acked/created/edited/recalled/held/removed`, `chat.reaction.updated`, `chat.pin.updated`, `chat.typing.updated`, `chat.receipt.updated`, `chat.unread.updated`, `chat.mention.created`, `chat.sanction.applied`, `chat.history.loaded`, `system.command.failed` | server → client | Có sẵn |
| `party.state.snapshot.chat_channel_id` | server → client | Có sẵn |
| `chat_channel_ids` trong message trận | server → client | Cần thêm (F105, xem F117) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không dùng NATS (R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- Không có delta riêng của F119; phụ thuộc delta của F116 (`params` quick message, `category_code` trong lỗi), F118 (`toggleMessageReaction`, `CHAT_REACTION_LIMIT`), F117 (`chat_channel_ids` trong message trận).
- Fixture GdUnit4 sinh từ example OpenAPI/AsyncAPI vào `src/game/tests/fixtures/contract/` (`tools/gen_fixtures.mjs`, tiền tố `chat.`).
- **Đã làm 2026-11-27 (chỉ ví dụ, không đổi schema):** ví dụ response `listMyChannels`, `listChannelMessages`, `listQuickMessages`, `listStickerPacks`, `listGifs`, `createReport` trong `specs/openapi/eduworld-api.v1.yaml` (module tách lại); `examples` cho `chat.message.created/acked/held/edited/recalled/removed`, `chat.reaction.updated`, `chat.pin.updated`, `chat.typing.updated`, `chat.receipt.updated`, `chat.unread.updated`, `chat.mention.created`, `chat.sanction.applied`, `chat.history.loaded`, `system.command.acked` (gửi tin) và `system.command.failed` (`CHAT_CONTENT_BLOCKED` pii/từ, `CHAT_TEMPORARILY_UNAVAILABLE`, `CHAT_RATE_LIMITED`, `CHAT_CHANNEL_CLOSED`, `NOT_FOUND`, `CHAT_REACTION_LIMIT`, `CHAT_MUTED`) — nội dung chữ trung tính (R15).
- **Câu hỏi mở cho backend [Proposed]:** `Channel` của `listMyChannels` chưa trả `features` và `max_text_length` (backend chỉ có trong policy hiệu lực); chưa có API danh sách emoji/reaction cho phép và thời hạn sửa/thu hồi cho client; `chat_channel_ids` trong message trận vẫn chờ F105/F109.
