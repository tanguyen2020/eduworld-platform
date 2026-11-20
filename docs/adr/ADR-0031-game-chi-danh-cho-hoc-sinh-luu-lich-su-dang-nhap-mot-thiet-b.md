# ADR-0031 — Game chỉ dành cho học sinh, lưu lịch sử đăng nhập, một thiết bị cho mỗi tài khoản

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md), [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md), [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu: lưu thời gian login khi vào game; chỉ học sinh login vào game, vận hành không thể; một tài khoản chỉ login trên một thiết bị, login thiết bị khác thì tự logout thiết bị còn lại, có thông báo xác nhận trong game.

## Quyết định

1. **Chỉ học sinh vào game.**
   - Backend: `POST /api/v1/auth/session` và `POST /api/v1/game/sessions` với token của client `eduworld-game` (`azp`) chỉ chấp nhận User `user_type = student`, `status = active`, thuộc tenant hợp lệ (ADR-0026). Người dùng khác bị từ chối `403 GAME_ACCESS_STUDENT_ONLY` và ghi audit. API người chơi (`/api/v1/game/*`, `/multiplayer/*`, `/competition/*`) và realtime ticket cũng kiểm tra.
   - Keycloak (lớp bổ sung): client `eduworld-game` chỉ cho role `student` hoàn tất đăng nhập (kiểm chứng trên bản Keycloak triển khai).
   - Nhân sự nhà trường và vận hành xem trải nghiệm game qua preview trong Admin Portal (Godot Web export, dữ liệu mẫu, không tạo game session thật) [Confirmed].
2. **Lưu lịch sử đăng nhập game.**
   - Bảng append-only `game.login_events` (game-service): `login_event_id`, `tenant_id`, `student_id`, `game_session_id`, `device_id`, `platform`, `device_model`, `client_version`, `ip_hash` (không lưu IP thô), `sis_connection_id` (ADR-0046), `login_at` (giờ server), `logout_at`, `end_reason` (`logout | replaced_by_other_device | timeout | account_locked | cleanup | client_crash`).
   - Cập nhật `last_login_at` của User (identity-service, qua event) và `last_game_login_at` của GameProfile.
   - Event `student.game_logged_in` / `student.game_logged_out`; dùng cho báo cáo hoạt động, worker dọn tài khoản (ADR-0032), điều tra an ninh. Thời hạn lưu theo DATA_PRIVACY.
3. **Một thiết bị cho mỗi tài khoản học sinh.**
   - Mỗi bản cài có `device_id` (UUID sinh lần đầu, lưu `user://`, header `X-Device-Id`).
   - Mỗi học sinh có **tối đa 1 game session active** (unique partial index).
   - Thiết bị B đăng nhập khi A còn session: `POST /api/v1/game/sessions` trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE` kèm thông tin thân thiện của A; game B **hiển thị hộp thoại xác nhận**; học sinh đồng ý thì B gọi lại với `takeover = true` (kèm `Idempotency-Key`).
   - Takeover: kết thúc session A (`replaced_by_other_device`); gửi A message `session.session.replaced` rồi đóng WebSocket (close code `4409`); thu hồi Keycloak session của A (theo `sid`); game A hiện thông báo và về màn hình đăng nhập; ghi audit.
   - Trận/quest co-op đang diễn ra: coi như mất kết nối, tiếp tục từ B trong thời gian grace; bài kiểm tra tiếp tục được trên B.
   - Session A không heartbeat quá 45 s thì tự kết thúc (`timeout`), B vào không cần hỏi.
   - Chỉ áp dụng cho game; Admin Portal không giới hạn một thiết bị.

## Hệ quả

(+) Không dùng chung tài khoản nhiều máy, chống gian lận thi đấu, số liệu đăng nhập chính xác. (−) Máy dùng chung phòng tin học cần đăng xuất rõ ràng giữa các ca; UX hộp thoại phải thân thiện cho học sinh nhỏ.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-15 | Ghi `sis_connection_id` vào login events | ADR-0046 |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
