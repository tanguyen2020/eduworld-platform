# Data Model: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — game client không có dữ liệu bền vững: tin, con trỏ, unread chỉ giữ trong RAM (`ChatStore`), nguồn sự thật ở module chat (F115–F118). Cache asset sticker/GIF trong `user://cache/chat_assets/` chỉ là bản sao có thể xóa, khóa theo `asset_key` bất biến.

Mô hình trong RAM (typed class, không lưu đĩa):

| Class | Trường chính | Nguồn |
|---|---|---|
| `ChatMessage` | `message_id`, `client_message_id`, `channel_id`, `seq`, `sender` (`PlayerCard`), `kind`, `body` (đã che), `quick_message_id`, `params`, `sticker_id`, `gif_id`, `reply_to_message_id`, `quote`, `forwarded_from`, `reactions`, `revision`, `edited_at`, `recalled`, `moderation_state`, `local_state` (`pending/sent/delivered/read/failed`) | `Message` (OpenAPI), `ChatMessage` (AsyncAPI) |
| `ChatChannel` | `channel_id`, `channel_type`, `features`, `allowed_kinds`, `status`, `purge_after`, `unread_count`, `unread_mentions`, `last_seq`, `my_read_seq` | `Channel` |
| `ReceiptState` | `member` (`PlayerCard`), `delivered_seq`, `read_seq` | `chat.receipt.updated` |

## 2. Vòng đời / trạng thái

Tin của mình: `pending` (đồng hồ) → `sent` (`chat.message.acked`) → `delivered` → `read`; lỗi tạm thời → `pending` (gửi lại cùng `client_message_id`); lỗi kiểm duyệt → bỏ khỏi danh sách + toast phản hồi.

## 3. Quy tắc kiểm tra dữ liệu

- Sắp theo `seq`; không bao giờ chèn tin chưa `acked` của người khác.
- Giữ tối đa `chat_recent_buffer` tin mỗi channel; tin cũ hơn tải lại qua REST khi kéo lên.

## 4. Event / outbox

Không áp dụng (client không phát domain event).
