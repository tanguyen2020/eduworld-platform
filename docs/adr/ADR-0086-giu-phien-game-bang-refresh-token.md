# ADR-0086 — Giữ phiên game bằng refresh token: còn hạn thì không phải đăng nhập lại

> **Trạng thái:** Accepted · **Ngày:** 2026-11-16 · **OQ:** — · **Thay một phần:** ADR-0064 (mục 1, 2, 4, 5) · **Bổ sung:** ADR-0068, ADR-0083 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) · [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

> [!NOTE]
> Bổ sung bởi ADR-0078 mục 5: Windows mặc định `shared_device = true`; mọi nền tảng hiện màn "Tiếp tục là <nickname>?" khi tự vào lại bằng refresh token, nút "Không phải tôi" đăng xuất và xóa token.

## Bối cảnh

[ADR-0064](ADR-0064-khong-tu-dang-nhap-khi-mo-lai-game.md) bắt học sinh nhập mật khẩu mỗi lần mở game và chỉ giữ token trong RAM. Technical Architect quyết định: đăng nhập phải có refresh token; **nếu token còn hạn thì không cần đăng nhập lại**. Điều này giảm tải băm mật khẩu đầu tiết và giúp học sinh nhỏ tuổi (xem [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) CLS-02), nhưng phải giữ an toàn trên máy dùng chung ở phòng máy.

## Quyết định

1. **Lưu refresh token an toàn trên thiết bị cá nhân.** Sau đăng nhập thành công, game client lưu refresh token trong kho bảo mật của hệ điều hành: Windows DPAPI (theo user), Android Keystore, iOS Keychain. Không lưu mật khẩu; access token chỉ ở RAM. **Web** không lưu refresh token bền vững (chỉ RAM) [Confirmed] vì storage trình duyệt không đủ an toàn.
2. **Máy dùng chung không lưu.** Thiết bị cấu hình `shared_device = true` (phòng máy, `device.cfg`) giữ hành vi cũ: token chỉ trong RAM, mở lại game phải đăng nhập; thoát game thì logout.
3. **Khởi động game:** nếu có refresh token → gọi `POST /auth/game/token:refresh` kèm `device_id`. Thành công → bỏ qua màn đăng nhập, tạo game session (`POST /game/sessions`, ADR-0083) và vào game. Thất bại (hết hạn, bị thu hồi, sai thiết bị) → xóa token khỏi kho bảo mật và hiện form đăng nhập.
4. **Thời hạn [Confirmed]:** access token 5 phút; phiên Keycloak của client `eduworld-game-backend` idle 7 ngày, tối đa 30 ngày (tham số `game_session_sso`). Không dùng offline token. Hết hạn → đăng nhập lại.
5. **Bảo vệ refresh token:**
   - Xoay vòng (rotation) mỗi lần dùng; phát hiện dùng lại token cũ → thu hồi cả phiên.
   - Gắn với `device_id`: identity-service từ chối refresh từ thiết bị khác.
   - Thu hồi khi: đăng xuất chủ động, takeover sang thiết bị khác (quy tắc một thiết bị, ADR-0031), đổi/đặt lại mật khẩu, khóa tài khoản hoặc xử phạt, thu hồi quyền truy cập, dọn học sinh (ADR-0085).
6. **Đăng xuất chủ động** trong game: gọi `:end` game session, `POST /auth/game/logout` (thu hồi phiên Keycloak) và xóa token khỏi kho bảo mật.
7. **Portal (nhân sự):** giữ chính sách phiên hiện hành (idle 30 phút, tối đa 10 giờ, MFA theo role); dùng refresh token xoay vòng trong bộ nhớ và gia hạn im lặng qua phiên Keycloak — tải lại trang không phải đăng nhập lại khi phiên còn hạn.
8. ADR-0064 mục 3 (gia hạn bằng refresh token trong lúc chơi), mục 6 (năng lực đăng nhập) và mục 7 vẫn hiệu lực.

## Hệ quả

(+) Học sinh không phải nhập mật khẩu mỗi lần mở game trên thiết bị cá nhân; giảm tải Keycloak đầu tiết. (−) Thêm lưu trữ bảo mật theo nền tảng, xử lý xoay vòng và thu hồi; rủi ro khi thiết bị cá nhân bị người khác dùng (giảm bằng thời hạn và đăng xuất chủ động). Sửa KEYCLOAK_SSO, GAME_NETWORKING, GODOT_ARCHITECTURE, STUDENT_GAME_UX_CORE (S01), SECURITY_ARCHITECTURE, PRD, TEST_STRATEGY, CONFIG_DEFAULTS và feature F013, F060, F061.

## Phương án đã loại

- Giữ ADR-0064 (nhập mật khẩu mỗi lần): bất tiện cho học sinh nhỏ, tải băm mật khẩu cao.
- Offline token Keycloak: thời hạn quá dài, khó thu hồi theo thiết bị.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-16 | Tạo ADR | Technical Architect |
| 2026-11-17 | Ghi chú bổ sung từ ADR-0078 mục 5 (không đổi ý nghĩa quyết định) | Technical Architect |
