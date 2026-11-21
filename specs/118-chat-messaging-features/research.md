# Research: F118 — Bộ tính năng nhắn tin đầy đủ (sửa/thu hồi, reply/forward, mention, receipts) và sticker/GIF

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Thứ tự áp sự kiện thay đổi tin

- **Decision**: Mọi sự kiện `evt` mang `message_id` + `revision` mới (edit: `revision + 1`; recall: `revision + 1`; reaction/pin không đổi revision, idempotent theo khóa `(message_id, user_id, emoji_code)` / `(channel_id, message_id)`). Persister áp edit/recall chỉ khi `revision > revision hiện tại` trong PG; bản cũ đến muộn bị bỏ, tin chưa có trong PG thì retry (NAK có delay) tối đa `max_deliver`.
- **Rationale**: EVENT_DESIGN §4.10; persister ghi lô không đảm bảo thứ tự giữa `msg` và `evt`.
- **Alternatives**: khóa hàng PG trên đường sửa — loại (R25: không ghi PG từ realtime).

## R2. Receipts không lưu theo tin × người

- **Decision**: Lưu con trỏ `last_delivered_seq`, `last_read_seq` mỗi thành viên (gộp ở `ew:chat:cursor_buf`, flush ≤ 1 lần/2 s qua JetStream `cursor`). `getMessageReceipts(seq)` tính: Read nếu `last_read_seq ≥ seq`, Delivered nếu `last_delivered_seq ≥ seq`; channel > 8 thành viên (club — `team` cố định; `class`/`study_group` đã bỏ theo ADR-0095) chỉ trả số đếm; ẩn thành viên có quan hệ chặn với người gọi.
- **Rationale**: CHAT_SYSTEM §8.1; tránh bảng bùng nổ dòng.

## R3. Version sticker pack/GIF

- **Decision**: Dùng chung khung game-admin (`ContentStatus`, `GameConfigVersion`, publish/rollback/unpublish) như module game, nhưng lưu trong `chat.chat_asset_versions` (Mới) với `config_type ∈ {sticker-packs, gifs}`; bảng `sticker_packs`/`gif_assets` giữ trạng thái hiện hành (`status`). Asset trên object storage theo khóa bất biến `media/<sha256>/…` (R31, ADR-0044); tin cũ tham chiếu `sticker_id`/`gif_id` bất biến.
- **Rationale**: contract `*_chat` đã có (`listGameConfigs_chat`, `publishGameConfigVersion_chat`…); ADR-0043 yêu cầu version bất biến sau publish.
- **Alternatives**: chỉ `status` không version — loại vì contract có diff/rollback/releases.
