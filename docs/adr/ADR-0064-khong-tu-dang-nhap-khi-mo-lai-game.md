# ADR-0064 — Không tự đăng nhập khi mở lại game

> **Trạng thái:** Accepted · một phần superseded by ADR-0086 · một phần superseded by [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Supersedes một phần:** ADR-0045 (mục 3: phiên dài + refresh token lưu trên thiết bị), ADR-0007 (lưu refresh token trong secure storage) · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0086](ADR-0086-giu-phien-game-bang-refresh-token.md):** không còn bắt nhập mật khẩu mỗi lần mở game; refresh token được lưu trong kho bảo mật của thiết bị cá nhân (mục 1, 2, 4, 5). **Còn hiệu lực:** mục 3, 6, 7; máy dùng chung vẫn chỉ giữ token trong RAM.

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)** (owner bỏ MFA/TOTP cho đăng nhập Admin Portal, 2026-11-21). Không còn hiệu lực: "MFA theo role" ở mục 7. **Còn hiệu lực:** chính sách phiên portal idle 30 phút, tối đa 10 giờ (phiên `platform_staff` tối đa 8 giờ từ lúc nhập mật khẩu, ADR-0094 mục 2).

## Bối cảnh

Người duyệt yêu cầu: **mở game lần sau phải nhập lại mật khẩu, không được tự vào game** vì lý do bảo mật (máy dùng chung ở trường, thiết bị gia đình nhiều người dùng).

## Quyết định

1. **Mỗi lần khởi động game là một lần đăng nhập mới bằng username + password** (ADR-0063). Không có "Ghi nhớ đăng nhập", không tự đăng nhập.
2. **Token chỉ nằm trong bộ nhớ (RAM) của tiến trình game.** Không ghi refresh token, access token hay mật khẩu xuống đĩa, secure storage, Keychain/Keystore hoặc `user://`. Client chỉ được nhớ **username** (tùy chọn, mặc định bật, có thể tắt; tắt sẵn trên thiết bị phòng máy đã cấu hình).
3. **Trong lúc game đang mở:** dùng refresh token trong bộ nhớ để gia hạn access token (5 phút) và để kết nối lại khi rớt mạng, không bắt nhập lại mật khẩu.
4. **Phiên Keycloak của client `eduworld-game` [Confirmed]:** SSO session idle 30 phút, tối đa 10 giờ; không bật "Remember me"; không dùng offline token. Khi game thoát bình thường → logout (end-session + revoke). Game bị tắt đột ngột → phiên tự hết hạn theo idle; vì token không lưu xuống đĩa nên lần mở sau vẫn phải nhập mật khẩu.
5. **Mobile:** ứng dụng chuyển nền quá 30 phút hoặc bị hệ điều hành đóng → mở lại phải đăng nhập lại [Confirmed].
6. **Năng lực đăng nhập (thay ADR-0045 mục 3):** mọi lượt vào game đều băm mật khẩu ở Keycloak. Mục tiêu tải nâng lên **đỉnh 100 lượt đăng nhập mật khẩu/giây**, bão đăng nhập 5.000 lượt trong 10 phút [Confirmed]. Sizing Keycloak tính lại trong DEPLOYMENT (ADR-0045 mục 5–6 vẫn áp dụng: tham số băm được benchmark, hàng chờ đăng nhập, rate limit theo tenant/thiết bị, scale theo lịch học).
7. Portal (nhân sự nhà trường, vận hành) giữ chính sách phiên của ADR-0027/KEYCLOAK_SSO (idle 30 phút, tối đa 10 giờ, MFA theo role).

## Hệ quả

(+) Không ai mở máy dùng chung mà vào được tài khoản của học sinh khác; giảm rủi ro lộ token trên thiết bị. (−) Học sinh phải nhập mật khẩu mỗi lần chơi; tải Keycloak giờ cao điểm tăng (cần thêm CPU); nhà trường cần hỗ trợ đặt lại mật khẩu nhiều hơn.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
| 2026-11-16 | Một phần superseded by ADR-0086 | Technical Architect |
| 2026-11-21 | Một phần superseded by ADR-0094 (bỏ MFA/TOTP cho đăng nhập Admin Portal) | Owner dự án |
