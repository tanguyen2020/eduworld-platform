# Contracts: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listMyChannels` | `GET /api/v1/chat/channels` (`Channel.status`, `purge_after`, `unread_count`) | chat.v1.yaml | `game_player` | Có sẵn |
| `listChannelMessages` | `GET /api/v1/chat/channels/{channel_id}/messages` (sau purge → `404`) | chat.v1.yaml | `game_player` + member | Có sẵn |
| `sendChannelMessage` | `POST /api/v1/chat/channels/{channel_id}/messages` (`409 CHAT_CHANNEL_CLOSED`) | chat.v1.yaml | `game_player` + member | Cần sửa (khai báo `409`, F115 T005) |
| `getChatRetentionSettings` / `putChatRetentionSettings` | `GET/PUT /api/v1/admin/settings/chat-retention` (`422 RETENTION_BELOW_MINIMUM`) | identity.v1.yaml | `platform.settings.write` | Có sẵn (identity, ngoài phạm vi — chỉ tiêu thụ kết quả) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `getTenant` | chat → identity (dự phòng lấy `settings.gameplay_chat_retention_days`) | identity.internal.v1.yaml | gồm `chat` | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `chat.receipt.read` | client → server | Có sẵn |
| `chat.unread.updated` | server → mọi thiết bị của người dùng | Có sẵn |
| `chat.sync`, `chat.history.loaded` | client ↔ server (resume/đổi thiết bị) | Có sẵn |
| `system.command.failed` `CHAT_CHANNEL_CLOSED`, `CHAT_MUTED` | server → client | Có sẵn |
| `party.state.snapshot.chat_channel_id` | server → thành viên party (competition) | Có sẵn |
| Payload trận (`battle.*`) mang `chat_channel_ids {match, team}` | server → người chơi (competition) | Cần thêm (F105) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `party.created`, `party.member_joined`, `party.member_left`, `party.member_kicked`, `party.disbanded` | competition → chat | `specs/events/payloads/competition/party.*.v1.schema.json` | Cần thêm (F100/F101 sở hữu) |
| `match.started` (thêm `participants[] {student_id, team_id?, school_id}`, `mode`) | competition → chat | `specs/events/payloads/competition/match.started.v1.schema.json` | Cần thêm + Cần sửa payload EVENT_DESIGN §4.8 (F105 sở hữu) |
| `match.completed`, `match.aborted` | competition → chat | `specs/events/payloads/competition/match.completed.v1.schema.json`, `match.aborted.v1.schema.json` | Cần thêm (F105) |
| `tenant.updated`, `platform_settings.changed` (key `chat_retention`) | identity → chat | `specs/events/payloads/identity/` | Có sẵn (áp 2026-11-17; owner F010 cho `tenant.updated`, owner F015 cho `platform_settings.changed`) |
| `chat.channel_opened`, `chat.channel_closed` | chat → RT, worker `chat-channel-closer` | `specs/events/payloads/chat/chat.channel_opened.v1.schema.json`, `chat.channel_closed.v1.schema.json` | Cần thêm |
| `chat.gameplay_purged` | job purge → AUD, MON | `specs/events/payloads/chat/chat.gameplay_purged.v1.schema.json` | Cần thêm |
| `chat.daily_stats` | job rollup → analytics (module game) | `specs/events/payloads/chat/chat.daily_stats.v1.schema.json` | Cần thêm |
| JetStream `ew.chat.<tenant_id>.cursor.<channel_id>` | module chat → persister | `specs/events/payloads/chat/chat-cursor.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/chat-policy.schema.json` v5 | Ràng buộc always-on (`match`/`team`/`party`) ở mọi cấp | Có sẵn (TC-CHAT-018 ghi "v4" — sửa TEST_STRATEGY) |

## 6. Delta cần thực hiện

- Event payload chat mới (§4): `chat.channel_opened`, `chat.channel_closed`, `chat.gameplay_purged`, `chat.daily_stats`, `chat-cursor`.
- Phối hợp competition (F101/F105): `match.started` thêm `participants[]` + `mode`; tạo schema payload `party.*`, `match.*`; thêm `chat_channel_ids` vào message trận AsyncAPI; dùng `src/backend/pkg/chatid` để điền `chat_channel_id`.
- Phối hợp realtime (F090): message fan-out nội bộ `ew:chat:fanout:<channel_id>` mang `exclude_user_ids` (gateway không giao cho các user này); khai báo định dạng ở REALTIME_ARCHITECTURE §5.0.
- `ProblemCode`/API: `sendChannelMessage` khai báo `409 CHAT_CHANNEL_CLOSED` (đã nằm trong delta F115).
- Kiểm: `make specs-lint`, `make test-contract` (consumer contract với module competition).
- ADR-0078: `chat-policy.schema.json` cho phép thu hẹp `allowed_kinds` của `match`/`team`/`party` chỉ qua preset `quick_message_emote_only` (task T039).
