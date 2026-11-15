# ADR-0064 — Không tự đăng nhập khi mở lại game

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Supersedes một phần:** ADR-0045 (mục 3: phiên dài + refresh token lưu trên thiết bị), ADR-0007 (lưu refresh token trong secure storage) · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Người duyệt yêu cầu: **mở game lần sau phải nhập lại mật khẩu, không được tự vào game** vì lý do bảo mật (máy dùng chung ở trường, thiết bị gia đình nhiều người dùng).

## Quyết định

1. **Mỗi lần khởi động game là một lần đăng nhập mới bằng username + password** (ADR-0063). Không có "Ghi nhớ đăng nhập", không tự đăng nhập.
2. **Token chỉ nằm trong bộ nhớ (RAM) của tiến trình game.** Không ghi refresh token, access token hay mật khẩu xuống đĩa, secure storage, Keychain/Keystore hoặc `user://`. Client chỉ được nhớ **username** (tùy chọn, mặc định bật, có thể tắt; tắt sẵn trên thiết bị phòng máy đã cấu hình).
3. **Trong lúc game đang mở:** dùng refresh token trong bộ nhớ để gia hạn access token (5 phút) và để kết nối lại khi rớt mạng, không bắt nhập lại mật khẩu.
4. **Phiên Keycloak của client `eduworld-game` [Proposed]:** SSO session idle 30 phút, tối đa 10 giờ; không bật "Remember me"; không dùng offline token. Khi game thoát bình thường → logout (end-session + revoke). Game bị tắt đột ngột → phiên tự hết hạn theo idle; vì token không lưu xuống đĩa nên lần mở sau vẫn phải nhập mật khẩu.
5. **Mobile:** ứng dụng chuyển nền quá 30 phút hoặc bị hệ điều hành đóng → mở lại phải đăng nhập lại [Proposed].
6. **Năng lực đăng nhập (thay ADR-0045 mục 3):** mọi lượt vào game đều băm mật khẩu ở Keycloak. Mục tiêu tải nâng lên **đỉnh 100 lượt đăng nhập mật khẩu/giây**, bão đăng nhập 5.000 lượt trong 10 phút [Proposed]. Sizing Keycloak tính lại trong DEPLOYMENT (ADR-0045 mục 5–6 vẫn áp dụng: tham số băm được benchmark, hàng chờ đăng nhập, rate limit theo tenant/thiết bị, scale theo lịch học).
7. Portal (giáo viên, vận hành) giữ chính sách phiên của ADR-0027/KEYCLOAK_SSO (idle 30 phút, tối đa 10 giờ, MFA theo role).

## Hệ quả

(+) Không ai mở máy dùng chung mà vào được tài khoản của học sinh khác; giảm rủi ro lộ token trên thiết bị. (−) Học sinh phải nhập mật khẩu mỗi lần chơi; tải Keycloak giờ cao điểm tăng (cần thêm CPU); giáo viên cần hỗ trợ đặt lại mật khẩu nhiều hơn.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
