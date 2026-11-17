# Research: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Id channel gameplay tất định

- **Decision**: `pkg/chatid.ChannelID(kind, scopeID, teamID)` = UUIDv5(namespace cố định của EduWorld, `"<kind>:<scope_id>[:<team_id>]"`). module competition điền `Party.chat_channel_id` và id channel trận/đội trong payload realtime bằng cùng hàm; module chat tạo channel với đúng id khi tiêu thụ event (upsert).
- **Rationale**: Client có `chat_channel_id` ngay khi nhận state party/trận mà không cần gọi đồng bộ module chat (R29, R30); idempotent tự nhiên khi event giao lại.
- **Alternatives**: competition gọi facade `chat` để mở channel — loại (R29: phản ứng phụ "mở channel chat" phải dùng event, API_DESIGN §12); module chat sinh UUIDv7 rồi phát ngược `chat.channel_opened` để competition lưu — loại vì client phải chờ vòng event thứ hai.
- **Hệ quả**: Nếu client `chat.channel.join` trước khi module chat xử lý event, nhận `NOT_FOUND` → client thử lại với backoff `client_reconnect_backoff` (F119); SC-001 đo độ trễ này.

## R2. Ẩn tin theo block/mute khi fan-out

- **Decision**: Khi publish `ew:chat:fanout:<channel_id>`, module chat đính `exclude_user_ids` = thành viên có quan hệ block hai chiều với người gửi ∪ thành viên đã mute người gửi (đọc `social.blocks`, `social.mutes` cùng database, cache Valkey theo user). `realtime` bỏ giao cho các user đó. Lịch sử (`listChannelMessages`, `chat.sync`) lọc cùng quy tắc ở query.
- **Rationale**: CHILD_SAFETY §6: kiểm soát an toàn thực thi ở server; gateway không có logic nghiệp vụ, chỉ áp danh sách đã tính.
- **Alternatives**: client tự ẩn — loại (không phải lớp bảo vệ); fan-out riêng từng người — loại (tải Valkey tăng theo số thành viên).
- **Phụ thuộc**: định dạng message fan-out nội bộ cần thống nhất với F090 (contracts §6).

## R3. Fail-closed khi chưa có module moderation

- **Decision**: Port `ModerationReadiness.Ready(ctx)` kiểm migration của F121 (bảng `chat.reports`, `chat.moderation_cases` tồn tại và có version tối thiểu). Chưa sẵn sàng → job ghi `gameplay_purge_runs.status = blocked`, metric `eduworld_chat_purge_blocked{reason="moderation_unavailable"}` = 1, alert, không xóa. Sanction (`chat.sanctions`, F116) đã có từ M4a nên luôn được kiểm.
- **Rationale**: Mất bằng chứng không thể khôi phục; giữ dữ liệu lâu hơn hạn vài tuần (M4b → M5) là rủi ro thấp hơn và có cảnh báo.
- **Alternatives**: F117 tự tạo bảng report/case tối giản — loại (trùng sở hữu với F121); purge bỏ qua kiểm tra — loại (vi phạm ADR-0033 mục 3).

## R4. Nguồn `gameplay_chat_retention_days`

- **Decision**: Consumer identity (`tenant.updated`, `platform_settings.changed` key `chat_retention`) cập nhật `ref.tenants.settings_subset.gameplay_chat_retention_days` (giá trị hiệu lực); thiếu thì gọi `getTenant` (facade `identity`, `chat` có trong `x-allowed-callers`). `purge_after` chốt lúc đóng channel; đổi cài đặt sau đó không áp hồi tố.
- **Rationale**: identity sở hữu cài đặt (API_DESIGN `/admin/settings/chat-retention`); projection theo DATABASE_DESIGN §2.17.
