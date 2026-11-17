# Data Model: F118 — Bộ tính năng nhắn tin đầy đủ (sửa/thu hồi, reply/forward, mention, receipts) và sticker/GIF

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§2.12, §3.8, §3.8a)

Database `eduworld_chat`. Cột tính năng trên `chat.messages`/`chat.gameplay_messages` (`reply_to_message_id`, `quote`, `forwarded_from`, `mentions`, `revision`, `edit_count`, `edited_at`, `recalled_at`, `recalled_by`) và `chat.channel_members` (`last_delivered_seq`, `last_read_seq`, `unread_mentions`, `role`) đã tạo ở F115.

## 1. Entity

### MessageRevision — `chat.message_revisions` (Có sẵn, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id` | uuid | NOT NULL, RLS | |
| `message_id`, `revision` | uuid, int | PK `(message_id, revision)` | bản **cũ** trước mỗi lần sửa |
| `body`, `body_display` | `varchar(1000)` | | bằng chứng; không trả API người dùng |
| `decision` | `varchar(64)` | NOT NULL | quyết định kiểm duyệt của bản đó |
| `edited_at` | timestamptz | NOT NULL | |

### MessageReaction — `chat.message_reactions` (Có sẵn, T)

`tenant_id`, `channel_id`, `message_id`, `user_id`, `emoji_code varchar(64)`, `created_at`, `deleted_at` (bỏ reaction = soft-delete). PK `(message_id, user_id, emoji_code)`; ≤ `chat_reactions_per_user` active/người/tin (use case).

### PinnedMessage — `chat.pinned_messages` (Có sẵn, T)

`tenant_id`, `channel_id`, `message_id`, `pinned_by`, `pinned_at`, `deleted_at`, `deleted_by`. PK `(channel_id, message_id)`; ≤ `chat_pins_max` active/channel (use case + trigger đếm `WHERE deleted_at IS NULL`).

### StickerPack / Sticker — `chat.sticker_packs`, `chat.stickers` (Có sẵn, G platform)

Theo DDL DATABASE_DESIGN §3.8a: pack `pack_id`, `owner_scope = 'platform'`, `code varchar(64)` (UNIQUE partial), `name_vi`, `name_en varchar(128)`, `unlock_rule jsonb`, `status` (`draft/in_review/published/archived`); sticker `sticker_id`, `pack_id`, `asset_key varchar(512)`, `alt_text_vi varchar(1000)`, `sort_order`. **Thêm cột (Mới)**: `stickers.alt_text_en varchar(1000)` (song ngữ, ADR-0056), `sticker_packs.current_version_id uuid`.

### GifAsset / GifTag — `chat.gif_assets`, `chat.gif_tags` (Có sẵn, G platform)

`gif_id`, `asset_key`, `preview_key varchar(512)`, `width`, `height` (≤ 480), `status`, `reviewed_by`, `reviewed_at`, `created_by` (**Thêm cột**, để kiểm người duyệt ≠ người upload); tag `(gif_id, tag varchar(64), locale varchar(64))` PK; index `tag`.

### ChatAssetVersion — `chat.chat_asset_versions` (Mới, G append-only)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `version_id` | UUIDv7 | PK | |
| `config_type` | `varchar(64)` | CHECK `sticker-packs/gifs` | |
| `config_id` | uuid | NOT NULL | `pack_id` / `gif_id` |
| `version` | int | UNIQUE `(config_type, config_id, version)` | |
| `status` | `varchar(64)` | theo `ContentStatus` | |
| `payload` | jsonb | CHECK `octet_length ≤ 65536` | snapshot pack/sticker/tag |
| `created_by`, `created_at`, `published_by`, `published_at` | | | bất biến sau publish (R31) |

### StickerUnlock — `ref.sticker_unlocks` (Mới, projection)

`student_id`, `tenant_id`, `pack_id`, `unlocked_at`, `source_version bigint`; PK `(student_id, pack_id)`. Nguồn: event mở khóa của module game (F075).

### Valkey (tạm thời, không là nguồn sự thật)

`ew:chat:typing:<channel_id>` (HASH, TTL 5 s), `ew:chat:cursor_buf:<channel_id>:<user_id>`, `ew:chat:mentions:<user_id>`, `ew:chat:unread:<user_id>` — theo VALKEY_DESIGN §3.

## 2. Vòng đời / trạng thái

- Tin: `active` → `edited` (revision++) → `recalled` (placeholder; nội dung giữ đến hết retention/purge).
- Sticker pack/GIF: `draft` → `in_review` → `published` → `archived`; rollback đổi `current_version_id`.

## 3. Quy tắc kiểm tra dữ liệu

- Sửa chỉ `kind = text`, trong `chat_edit_window`; thu hồi trong `chat_recall_window`; không reaction/forward tin đã thu hồi.
- `quote.excerpt` ≤ 200 ký tự cắt từ `body_display`; reply preview ≤ 80 ký tự.
- Forward ≤ `chat_forward_targets_max` đích, cùng `tenant_id`.
- Sticker webp/png ≤ 512 KB; GIF ≤ 2 MB, ≤ 480 px; GIF publish cần `reviewed_by ≠ created_by`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| JetStream `ew.chat.<tenant_id>.evt.<channel_id>` (`edit`, `recall`, `reaction_set/unset`, `pin/unpin`) | Mọi thay đổi tin | `specs/events/payloads/chat/chat-evt.v1.schema.json` (Mới) |
| `chat.message_edited` / `chat.message_recalled` / `chat.message_forwarded` | Persister commit (không nội dung) | `specs/events/payloads/chat/chat.message_edited.v1.schema.json`, `chat.message_recalled.v1.schema.json`, `chat.message_forwarded.v1.schema.json` (Mới) |
| `chat.message_pinned` / `chat.message_unpinned` | Persister commit | `specs/events/payloads/chat/chat.message_pinned.v1.schema.json`, `chat.message_unpinned.v1.schema.json` (Mới) |
| `content.published` (`item_type = sticker_pack/gif`) | Publish asset | `specs/events/payloads/chat/content.published.v1.schema.json` (Mới, theo EVENT_DESIGN §4.2) |
