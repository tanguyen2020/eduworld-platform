# Data Model: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§2.12, §3.0, §3.8, §3.8b)

Database `eduworld_chat`, schema `chat`. Mọi bảng T có `tenant_id` (FK `ref.tenants`), `deleted_at`, `deleted_by`, `delete_reason varchar(64)`, `created_at`, `updated_at`, RLS theo `app.tenant_id` (DATABASE_DESIGN §2 quy ước T).

## 1. Entity

### Channel — `chat.channels` (Có sẵn trong DATABASE_DESIGN, tạo ở F115, T)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `channel_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NOT NULL, RLS | |
| `tenant_ids` | `uuid[]` | | chỉ channel liên tenant do platform tạo (F107/F117) |
| `school_id` | uuid | | |
| `channel_type` | `varchar(64)` | CHECK 8 giá trị `world/area/match/party/team/class/study_group/direct` | |
| `is_gameplay` | bool | CHECK theo DATABASE_DESIGN §2.12 | |
| `scope_ref` | jsonb | CHECK `octet_length ≤ 2048` | `{match_id}`, `{party_id}`, `{match_id, team_id}`, `{class_id}`… |
| `policy_id` | uuid | | rule cấp class nếu có |
| `retention_days` | int | | channel bền vững (F120) |
| `last_seq` | bigint | NOT NULL DEFAULT 0 | persister `GREATEST` |
| `status` | `varchar(64)` | CHECK `open/closed` | |
| `opened_at`, `closed_at`, `purge_after` | timestamptz | | `purge_after` do F117 đặt |

**Index**: `(tenant_id, channel_type, status)`; `(purge_after) WHERE is_gameplay AND closed_at IS NOT NULL` (F117 dùng).

### ChannelMember — `chat.channel_members` (Có sẵn, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `channel_id`, `user_id` | uuid | PK `(channel_id, user_id)` | |
| `tenant_id`, `school_id` | uuid | | |
| `role` | `varchar(64)` | CHECK `member/leader` | leader dùng cho pin party (F118) |
| `joined_at`, `left_at` | timestamptz | | lịch sử chỉ từ `joined_at` |
| `muted_until` | timestamptz | | |
| `last_delivered_seq`, `last_read_seq` | bigint | NOT NULL DEFAULT 0, chỉ tăng | F117/F118 cập nhật |
| `unread_mentions` | int | NOT NULL DEFAULT 0 | F118 |
| `show_read_receipts` | bool | DEFAULT true | F118 |
| `notification_level` | `varchar(64)` | CHECK `all/mentions/none` | |

### Message — `chat.messages` (Có sẵn, T, P tháng `created_at`) và `chat.gameplay_messages` (Có sẵn, T, P ngày `created_on`)

Cột đúng DDL DATABASE_DESIGN §3.8: `message_id` (UUIDv7 = `Nats-Msg-Id`), `channel_id`, `seq bigint`, `tenant_id`, `school_id`, `sender_id`, `client_message_id`, `kind varchar(64)` CHECK, `body varchar(1000)`, `body_display varchar(1000)`, `quick_message_id`, `sticker_id`, `gif_id`, `reply_to_message_id`, `quote jsonb`, `forwarded_from jsonb`, `mentions uuid[]` (CHECK ≤ 5), `revision`, `edit_count`, `edited_at`, `recalled_at`, `recalled_by`, `moderation_state varchar(64)` CHECK `allowed/masked/unchecked/held/removed`, `filter_verdict jsonb`, `search_tsv` (generated, `chat.f_unaccent`), `created_at`, `deleted_at`, `deleted_by`. `gameplay_messages` thêm `created_on date`.

**Ràng buộc**: PK `(message_id, created_at)` / `(message_id, created_on)`; UNIQUE `(channel_id, seq, created_at)` / `(channel_id, seq, created_on)`. **Index**: `messages_channel_seq_idx (channel_id, seq DESC) WHERE deleted_at IS NULL`, `messages_sender_idx`. F115 tạo cả GIN `search_tsv` theo DDL (dùng ở F120).

### GameplayPartitionChannel — `chat.gameplay_partition_channels` (Có sẵn, G)

`partition_day date`, `channel_id`, `tenant_id`, `purged_at`; PK `(partition_day, channel_id)`. Persister ghi khi lưu tin gameplay; F117 dùng khi purge.

### ChatPolicy — `chat.chat_policies` (Có sẵn, T*)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `policy_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NULL khi `scope_type = platform` | T* |
| `scope_type` | `varchar(64)` | CHECK `platform/tenant/school/class` | |
| `scope_id`, `school_id` | uuid | UNIQUE `(scope_type, coalesce(scope_id,…)) WHERE deleted_at IS NULL` | |
| `rules` | jsonb | validate theo `chat-policy.schema.json` v5; CHECK `octet_length ≤ 65536` | không có trường `grade_*` |
| `version` | int | optimistic concurrency | |

F115 chỉ đọc + seed cấp platform; ghi qua API ở F120.

## 2. Vòng đời / trạng thái

`Channel.status`: `open` → `closed` (F117/F120 đóng) — gửi vào `closed` trả `CHAT_CHANNEL_CLOSED` (F117). `Message.moderation_state` được đặt một lần khi gửi (`allowed/masked/unchecked/held`); `removed` do moderation (F121).

## 3. Quy tắc kiểm tra dữ liệu

- `seq` cấp trước publish (Valkey), PG unique là chốt chặn cuối ([VALKEY_DESIGN §4.9](../../docs/07-backend/VALKEY_DESIGN.md)).
- Tin `block` không có dòng; tin `held` có dòng `moderation_state = held` (DATABASE_DESIGN §3.8).
- `body` (gốc) không bao giờ trả cho API người dùng khi `masked`/`recalled`/`removed`.
- Role `chat_app` không có `DELETE`/`TRUNCATE` trên mọi bảng `chat.*` (TC-TEN-016).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `chat.message_created` | Persister commit tin (outbox, không nội dung) | `specs/events/payloads/chat/chat.message_created.v1.schema.json` (Mới) |
| JetStream `ew.chat.<tenant_id>.msg.<channel_id>` | Gửi tin (không qua outbox) | `specs/events/payloads/chat/chat-envelope.v1.schema.json` (Mới) |
