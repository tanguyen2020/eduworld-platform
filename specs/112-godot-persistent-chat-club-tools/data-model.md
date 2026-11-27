# Data Model: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — game client không có dữ liệu bền vững và F112 không thêm bảng nào. Channel, tin, thành viên, club, mục tiêu, sự kiện có nguồn sự thật ở module chat (F115–F121, schema `chat`/`social` của `eduworld_chat`). Mọi thứ dưới đây chỉ ở RAM, xóa khi đăng xuất hoặc `session.session.replaced`.

Mô hình trong RAM (typed class, không lưu đĩa):

| Class | Trường chính | Nguồn |
|---|---|---|
| `PersistentChannelTab` | `channel_id`, `channel_type` (`area`/`world`/`team`/`direct`), `label_source` (`area`, `world`, `club:<club_id>`, `friend:<user_id>`, `unknown`), `status` (`open`/`read_only`/`frozen`), `unread_count`, `unread_mentions`, `joined` (bool), `muted` | `Channel` (`listMyChannels`, `openDirectChannel`), `chat.unread.updated`, `chat.channel.membership_changed`; `Club.chat_channel_id` (F111) |
| `ChatHubState` | `tabs[]` (thứ tự: Khu vực, Cả trường, club, Bạn bè), `open_tab`, `current_area_channel_id`, `hud_unread`, `hud_mentions`, `visible` (ẩn khi `blocking_activity`) | Tính từ các nguồn trên |
| `DirectPeer` | `channel_id`, `user_id`, `nickname`, `avatar_thumbnail_url`, `level` (thẻ công khai) | `listFriends` (F110) + `openDirectChannel`; hoặc `sender` của tin của người kia |
| `SearchHit` | `message_id`, `channel_id`, `channel_type`, `seq`, `sender` (thẻ công khai), `created_at`, `excerpt` (văn bản thuần đã bỏ đánh dấu) | `MessageSearchHit` (`searchMessages`) |
| `ClubManageState` | `club` (`Club` + `version`), `members[]` (`ClubMember`: nickname, avatar, `role`), `edit_draft`, `goal_draft`, `event_draft`, `pending_idempotency_key` | `getClub`, `listClubMembers` (đủ mọi trang), form của học sinh |

## 2. Vòng đời / trạng thái

- Tab channel bền vững: `listed` → `joined` (tab mở, `chat.channel.join`) → `left` (đóng/đổi tab, `chat.channel.leave`); `area` cũ → bị thay khi `hub.instance.assigned`; club rời/bị gỡ hoặc lịch sử `404` → gỡ tab và xóa tin.
- Trạng thái gửi của channel: `open` → `read_only` (club khóa/lưu trữ) / `frozen` (DM có block) → `open`; khóa tạm `not_allowed` (lỗi `CHAT_NOT_ALLOWED`, tới lần làm mới kế tiếp) và `curfew` (`CURFEW_ACTIVE`, tới giờ kết thúc giới nghiêm).
- Form club: `editing` → `submitting` (khóa nút, giữ `Idempotency-Key`) → `saved` (toast) | `conflict` (`412`, tải lại) | `blocked` (`451`, giữ form) | `invalid` (`422`) | `forbidden` (`403`, đóng công cụ).

## 3. Quy tắc kiểm tra dữ liệu

- Tab chỉ tạo cho `channel_type` thuộc 4 loại trên; loại lạ bị bỏ qua (ADR-0095). Tab club chỉ khi `chat_channel_id` trùng một club của em; tab DM chỉ hiện tên khi biết `DirectPeer`.
- Độ dài nhập lấy từ contract (`ClubUpdate`, `ClubGoalCreate`, `ClubEventCreate`, `q` của `searchMessages`) và `Channel.max_text_length`; client chỉ chặn để thân thiện, server luôn kiểm lại.
- `excerpt` không bao giờ chứa markup: mọi chuỗi hiển thị là văn bản thuần.
- Không lưu, không log `q`, đoạn trích, nội dung tin hay nội dung form (R14).

## 4. Event / outbox

| Event | Vai trò | Ghi chú |
|---|---|---|
| — | Không áp dụng | Client không phát domain event và không dùng NATS (R25) |
