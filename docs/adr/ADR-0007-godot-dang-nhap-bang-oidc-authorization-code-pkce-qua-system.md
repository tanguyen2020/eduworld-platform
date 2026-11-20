# ADR-0007 — Godot đăng nhập bằng OIDC Authorization Code + PKCE qua system browser

> **Trạng thái:** Superseded by [ADR-0068](ADR-0068-dang-nhap-game-bang-form-trong-game-qua-backend.md) (game) và [ADR-0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) (portal) · **Ngày:** 2026-11-15 · **OQ:** OQ-04 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md), [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Đã được thay toàn phần.** Phần game thay bởi ADR-0068 (2026-11-15): game dùng form đăng nhập trong game gửi qua backend (client confidential, Direct Access Grant ở module identity), không mở trình duyệt; giữ phiên theo ADR-0086. Phần Admin Portal thay bởi [ADR-0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) (2026-11-17): portal đăng nhập bằng form riêng qua backend, không chuyển hướng OIDC. Không còn phần nào của ADR này có hiệu lực; nội dung dưới đây giữ để truy vết.

## Bối cảnh

Godot là public client; không được nhúng client secret, không lưu mật khẩu học sinh.

## Quyết định

Client `eduworld-game` là public client, PKCE `S256`, không client secret. Redirect theo nền tảng:

| Nền tảng | Redirect | Ghi chú |
|---|---|---|
| Windows/macOS/Linux | `http://127.0.0.1:{random_port}/callback` (loopback, RFC 8252) | Godot mở `OS.shell_open()` và lắng nghe bằng `TCPServer` |
| Android/iOS | `vn.eduworld.game:/oauth2/callback` (private-use URI scheme) | Android intent filter; iOS `ASWebAuthenticationSession` qua plugin |
| Web | `https://play.<domain>/auth/callback` | Redirect cùng tab |

- Không dùng embedded WebView để nhập mật khẩu.
- Máy dùng chung trong lớp có thể dùng **Device Authorization Grant** và các cách đăng nhập nhanh của ADR-0045 (không thuộc Release 1.0 theo ADR-0063; OQ-04 đã trả lời).
- Refresh token lưu bằng storage an toàn của nền tảng (Android Keystore/iOS Keychain; desktop: file mã hóa trong `user://` gắn máy — rủi ro ghi trong SECURITY_ARCHITECTURE). Logout gọi end-session endpoint + revoke refresh token.

## Hệ quả

(+) Đúng chuẩn OAuth cho native app. (−) Cần plugin nền tảng cho Android/iOS.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-20 | Superseded toàn phần: phần game thay bởi ADR-0068, phần portal (Authorization Code + PKCE) thay bởi ADR-0091; không chuyển Accepted | Owner |
