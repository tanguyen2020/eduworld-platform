# Contracts: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (tách ra `specs/openapi/modules/chat.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listMyChannels` | `GET /api/v1/chat/channels` | chat.v1.yaml | `game_player` (chỉ channel mình là member) | Có sẵn |
| `listChannelMessages` | `GET /api/v1/chat/channels/{channel_id}/messages?before_seq=&after_seq=&limit=` | chat.v1.yaml | `game_player` + member | Có sẵn |
| `sendChannelMessage` | `POST /api/v1/chat/channels/{channel_id}/messages` (`Idempotency-Key`, body `MessageCreate`; `201`/`403`/`429`/`451`) | chat.v1.yaml | `game_player` + member + policy | Cần sửa (thêm `404`, `409`, `428`, `503`) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (đường gửi không gọi facade module khác — R30, R29) | — | — | — |

## 3. Realtime (AsyncAPI)

Spec nguồn: `specs/asyncapi/eduworld-realtime.v1.yaml`; gateway định tuyến `chat.*` → NATS `ew.cmd.chat.<type>` (F090).

| Message | Hướng | Trạng thái |
|---|---|---|
| `chat.message.send` | client → server | Có sẵn |
| `chat.message.acked` | server → người gửi (sau JetStream ack) | Có sẵn |
| `chat.message.created` | server → member (fan-out) | Có sẵn |
| `chat.message.held` | server → người gửi | Có sẵn |
| `chat.channel.join` / `chat.channel.leave` | client → server | Có sẵn |
| `chat.history.fetch` → `chat.history.loaded` | client ↔ server | Có sẵn |
| `chat.sync` → `chat.history.loaded` | client ↔ server | Có sẵn |
| `system.command.failed` (`CHAT_TEMPORARILY_UNAVAILABLE`, `CHAT_NOT_ALLOWED`, `CHAT_KIND_NOT_ALLOWED`, `CHAT_RATE_LIMITED`, `NOT_FOUND`, `VALIDATION_FAILED`) | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| JetStream `ew.chat.<tenant_id>.msg.<channel_id>` (stream `CHAT_MESSAGES`) | module chat → `persister` (F115), `notifier`/`moderation`/`stats` (F118/F121/F117) | `specs/events/payloads/chat/chat-envelope.v1.schema.json` | Cần thêm |
| `CHAT_DLQ.persister` | persister → vận hành | dùng lại envelope | Cần thêm (khai báo stream trong EVENT_DESIGN §4.10 đã có) |
| `chat.message_created` | persister (outbox) → AUD, analytics | `specs/events/payloads/chat/chat.message_created.v1.schema.json` | Cần thêm |
| `audit.recorded` | module chat → identity | `specs/events/envelope.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/chat-policy.schema.json` (v5) | Validate seed và rule đọc từ `chat.chat_policies` | Có sẵn |

## 6. Delta cần thực hiện

- `specs/events/payloads/chat/` (thư mục mới): `chat-envelope.v1.schema.json` (đúng envelope [EVENT_DESIGN §4.10](../../../docs/07-backend/EVENT_DESIGN.md), mọi string có `maxLength`), `chat.message_created.v1.schema.json` (`message_id`, `channel_id`, `seq`, `sender_id`, `kind` — không nội dung).
- `eduworld-api.v1.yaml` `sendChannelMessage`: thêm response `404` (không phải member), `409` (`CHAT_CHANNEL_CLOSED`), `428` (`IDEMPOTENCY_KEY_REQUIRED`), `503` (`CHAT_TEMPORARILY_UNAVAILABLE`); giữ `MessageCreate.client_message_id` (tên duy nhất, RB-508); `MessageCreate.quick_message_id` là `Uuid` trong khi `Message.quick_message_id` và AsyncAPI là string 64 → thống nhất string 64.
- `eduworld-realtime.v1.yaml`: đổi `client_msg_id` → `client_message_id` trong `chat.message.send` và `chat.message.acked` (RB-508, CHAT_SYSTEM §5).
- `listChannelMessages`/`listMyChannels`: thêm response `403` (`GAME_ACCESS_STUDENT_ONLY`, `TENANT_MISMATCH`).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`, `make test-contract`.
- ADR-0078: `sendChannelMessage` và AsyncAPI `chat.message.send` khai báo `CURFEW_ACTIVE` (đã thêm vào `ProblemCode` và `ErrorCode`); `CHAT_KIND_NOT_ALLOWED` khi preset `quick_message_emote_only` (task T041).
