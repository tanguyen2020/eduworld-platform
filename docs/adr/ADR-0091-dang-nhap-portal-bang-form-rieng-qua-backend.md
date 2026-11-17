# ADR-0091 — Admin Portal đăng nhập bằng form riêng qua backend, không chuyển hướng OIDC

> **Trạng thái:** Accepted · một phần supersedes ADR-0066 (mục 1), ADR-0067 (chọn tenant bằng scope token), ADR-0068 (mục 4) · **Ngày:** 2026-11-17 · **OQ:** — · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Admin Portal (`web/apps/portal`) đang đăng nhập bằng Authorization Code + PKCE: trình duyệt chuyển sang trang Keycloak (theme `eduworld-ops`), MFA có điều kiện cho `platform_staff`, chọn tenant bằng cách xin token mới với scope `organization:{alias}` (ADR-0066, ADR-0067). Technical Architect yêu cầu màn hình đăng nhập do portal tự vẽ, không dùng luồng chuyển hướng OIDC. Game đã có mẫu tương tự: form trong game, module `identity` gọi token endpoint Keycloak bằng client confidential (ADR-0068).

Ràng buộc kỹ thuật của Keycloak: không có API tạo credential OTP; luồng Direct Access Grant trả cùng một lỗi cho sai mật khẩu và thiếu/sai OTP; WebAuthn không dùng được qua Direct Access Grant.

## Quyết định

### 1. Màn hình và API

- Portal có trang `/login` tự vẽ bằng Ant Design (theme từ `design/admin-tokens/`, ADR-0047): ô "Tên đăng nhập hoặc email", ô mật khẩu, ô "Mã xác thực 6 số" (ghi chú: bắt buộc với tài khoản vận hành platform), "Quên mật khẩu?", chọn ngôn ngữ. Không chuyển hướng sang Keycloak, không có bước chọn trường trước đăng nhập.
- API của module `identity`, qua APISIX public, TLS bắt buộc, không cần `X-Tenant-Id`:
  - `POST /api/v1/auth/portal/login` — `{username, password, otp?, device_id, locale}`. `identity` gọi token endpoint Keycloak (Direct Access Grant) bằng client `eduworld-portal`, kèm `totp` khi có.
  - `POST /api/v1/auth/portal/token:refresh` — làm mới access token bằng refresh token trong cookie.
  - `POST /api/v1/auth/portal/logout` — thu hồi refresh token và phiên Keycloak.
  - `POST /api/v1/auth/portal/setup:password` — đổi mật khẩu tạm lần đầu bằng `setup_ticket` (mục 3).
  - `POST /api/v1/auth/portal/password:forgot` và `POST /api/v1/auth/portal/password:reset` — quên mật khẩu (mục 3).
- Một form gửi một lần cả mật khẩu và OTP; lỗi luôn là thông điệp chung "Tên đăng nhập, mật khẩu hoặc mã xác thực chưa đúng" (`401 INVALID_CREDENTIALS`). Không có bước hỏi "tài khoản này có cần OTP không", để không lộ tài khoản nào là nhân sự platform. Trình duyệt được nhớ (không phải bí mật) rằng lần trước đăng nhập có OTP để làm nổi bật ô mã.

### 2. Keycloak

- Client `eduworld-portal` chuyển sang **confidential**, tắt Standard Flow, bật Direct Access Grants; secret chỉ module `identity` giữ. Giữ nguyên client id nên mọi kiểm tra `azp = eduworld-portal` (R20, R23, ADR-0066) không đổi.
- Direct grant flow `eduworld-portal-direct-grant`: kiểm tra username → mật khẩu → chỉ cho realm role `tenant_staff`/`platform_staff` → OTP có điều kiện (bắt buộc với `platform_staff`, tùy chọn với nhân sự tenant đã đăng ký OTP). Mapper AMR/LoA đặt `amr` chứa `otp` và `acr` mức MFA khi bước OTP chạy, để các kiểm tra MFA hiện có giữ nguyên. Spike xác nhận claim này trước khi làm F051/F012 [Proposed].
- MFA chỉ TOTP; WebAuthn không còn trong phạm vi portal.
- Brute-force detection của Keycloak giữ nguyên (khóa tạm sau 5 lần sai).

### 3. Mật khẩu lần đầu, đăng ký OTP, quên mật khẩu

- Keycloak trả "Account is not fully set up" khi tài khoản còn required action, sau khi mật khẩu đã đúng. Khi đó `identity` trả `403 ACCOUNT_SETUP_REQUIRED` kèm `setup_ticket` (dùng một lần, 10 phút, gắn `user_id`, lưu Valkey, không chứa mật khẩu) và danh sách bước còn thiếu.
- **Đổi mật khẩu tạm:** màn hình riêng của portal; `identity` đặt mật khẩu mới qua Keycloak Admin API (client `svc-identity`, áp password policy của realm) và gỡ required action `UPDATE_PASSWORD`; người dùng đăng nhập lại.
- **Đăng ký OTP:** Keycloak không có API tạo OTP, nên đây là **ngoại lệ duy nhất dùng trang Keycloak**: `identity` gọi Admin API `execute-actions-email` với `CONFIGURE_TOTP`; người dùng mở link trong email tới trang action-token của Keycloak (theme `eduworld-ops`), quét QR, rồi quay lại `/login`. Không phải luồng đăng nhập OIDC.
- **Quên mật khẩu:** portal tự xử lý. `password:forgot` luôn trả `202` (không lộ tài khoản có tồn tại); nếu có tài khoản nhân sự khớp, `identity` gửi email chứa link `https://portal.<domain>/reset-password?token=…` (token ngẫu nhiên, lưu hash, dùng một lần, hết hạn theo `portal_password_reset_ttl`); `password:reset` đặt mật khẩu qua Admin API. Học sinh không dùng luồng này.

### 4. Token và phiên trên trình duyệt

- Access token trả trong body, portal chỉ giữ trong bộ nhớ. Refresh token đặt trong cookie `HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/portal`, không đọc được bằng JavaScript. `token:refresh` và `logout` đòi thêm header `X-EW-CSRF` khớp cookie double-submit.
- Tải lại trang: portal gọi `token:refresh` để khôi phục phiên. Phiên giữ giới hạn hiện có: idle 30 phút, tối đa 10 giờ; route platform chỉ nhận `auth_time` trong 8 giờ, quá hạn trả `403 MFA_REQUIRED` và portal mở hộp đăng nhập lại (mật khẩu + OTP).
- **Chọn và đổi tenant không cần token mới:** tenant của phiên do `X-Tenant-Id` + kiểm tra membership quyết định (ADR-0065), giống game. Bỏ scope `organization:{alias}` cho portal.

### 5. Bảo vệ

Áp toàn bộ biện pháp của ADR-0068 mục 3 cho route `/auth/portal/*`: không log body, xóa mật khẩu khỏi bộ nhớ sau khi gọi Keycloak; rate limit theo username, theo `device_id` và toàn cục (không giới hạn theo một IP, R30); chặn học sinh hai lớp (flow Keycloak + `403 PORTAL_ACCESS_STAFF_ONLY`); audit đăng nhập thành công/thất bại.

## Hệ quả

(+) Giao diện đăng nhập thống nhất với portal, không rời trang; chọn tenant đơn giản hơn; dùng chung mẫu với game.

(−) Mật khẩu và OTP đi qua backend của EduWorld. Direct Access Grant bị OAuth 2.0 Security BCP khuyến cáo tránh, và không có MFA chống phishing (WebAuthn). Đăng ký OTP vẫn phải qua một trang Keycloak mở từ email. Phải viết thêm màn đổi mật khẩu, quên mật khẩu, CSRF cho cookie refresh. Phải sửa KEYCLOAK_SSO, ADMIN_PORTAL, SECURITY_ARCHITECTURE, API_DESIGN/OpenAPI, CONFIG_DEFAULTS, PRD (FR-AUTH-029/030), TEST_STRATEGY và F004, F012, F051.

## Phương án đã loại

- **Giữ OIDC, chỉ tùy biến theme Keycloak giống portal:** vẫn chuyển hướng sang trang Keycloak, không đáp ứng yêu cầu.
- **`identity` tự lưu và kiểm tra secret OTP:** trái R30 (chỉ Keycloak xác thực credential), phải tự bảo vệ secret, và token Keycloak không có `amr` để các kiểm tra MFA dựa vào.
- **Hỏi trước tài khoản có cần OTP:** lộ danh sách tài khoản nhân sự platform.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-17 | Tạo ADR (Accepted) | Technical Architect |
