# ADR-0094 — Bỏ MFA (TOTP) cho đăng nhập Admin Portal

> **Trạng thái:** Accepted (rủi ro được chấp nhận) · một phần supersedes ADR-0091 (mục 1 ô mã OTP, mục 2 OTP có điều kiện và mapper AMR/LoA, mục 3 đăng ký OTP, mục 4 step-up `MFA_REQUIRED`), ADR-0009, ADR-0022, ADR-0027 mục 3(c), ADR-0029, ADR-0063 mục 5, ADR-0064 mục 7, ADR-0066 mục 1, ADR-0067 mục 1, ADR-0068 mục 4, ADR-0072 (MFA của `safety_moderator`), ADR-0086 mục 7, ADR-0090 (phần MFA) · **Ngày:** 2026-11-21 · **OQ:** — · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

ADR-0091 đặt đăng nhập Admin Portal bằng form riêng qua module `identity`, kèm mã TOTP 6 số bắt buộc với realm role `platform_staff` (mọi role platform: `system_admin`, `safety_moderator`, Content Admin/Reviewer, Game Designer, Live-ops, Competition Admin) và tùy chọn với nhân sự tenant đã đăng ký OTP. Backend dựa vào claim `amr` chứa `otp` (hoặc `acr` mức MFA) và `auth_time` trong 8 giờ để mở route platform (`/api/v1/admin/*`, `/api/v1/game-admin/*`, `/api/v1/sis/*`) và `CrossTenantScope`; thiếu thì trả `403 MFA_REQUIRED`.

Ngày 2026-11-21 owner dự án quyết định **bỏ hoàn toàn xác thực hai lớp** khỏi đăng nhập Admin Portal, **cho mọi tài khoản kể cả role platform**: đăng nhập chỉ còn username/email + mật khẩu. Owner đã được cảnh báo rõ trước khi chốt: khi mật khẩu của một tài khoản platform bị lộ (phishing, dùng lại mật khẩu, máy nhiễm mã độc), người nắm mật khẩu có toàn quyền của tài khoản đó, trong đó System Admin đọc được dữ liệu học sinh (là trẻ em) của **mọi trường** và Safety Moderator đọc được tin nhắn liên quan case của mọi tenant. Owner vẫn chọn bỏ MFA.

## Quyết định

### 1. Đăng nhập

- Form `/login` của portal chỉ có "Tên đăng nhập hoặc email" và mật khẩu. `POST /api/v1/auth/portal/login` nhận `{username, password, device_id, locale}`; contract không còn trường `otp`.
- Direct grant flow `eduworld-portal-direct-grant`: username → mật khẩu → chặn người không có realm role `tenant_staff`/`platform_staff`. Không còn bước OTP nào (kể cả OTP "khi đã cấu hình"). Tài khoản đã từng đăng ký OTP vẫn đăng nhập bằng mật khẩu; credential OTP cũ được để nguyên trong Keycloak nhưng không flow nào đọc.
- Bỏ client scope `eduworld-amr-loa` (mapper `amr`, `acr` theo LoA) và mọi authenticator config `otp`: không còn thành phần nào dùng `amr`/`acr`.
- Bỏ đăng ký OTP: không còn bước `configure_otp`, không gửi email `execute-actions-email` `CONFIGURE_TOTP`, bỏ trường `otp_setup_email_sent`. Required action `CONFIGURE_TOTP` bị tắt ở mức realm và gỡ khỏi mọi người dùng còn tồn đọng (script `post-apply.mjs`, chạy sau mỗi lần apply realm), để không ai bị Keycloak chặn với "Account is not fully set up"; nếu realm chưa được apply lại, module `identity` tự gỡ `CONFIGURE_TOTP` còn sót qua Admin API ngay lúc đăng nhập (sau khi Keycloak đã xác nhận mật khẩu, audit `auth.portal_legacy_otp_action_removed`) rồi thử lại một lần. Tài khoản platform mới chỉ nhận email `UPDATE_PASSWORD`.
- Thông điệp lỗi chung còn "Tên đăng nhập hoặc mật khẩu chưa đúng" (`401 INVALID_CREDENTIALS`).

### 2. Backend

- Guard của route platform (`/admin/*`, `/game-admin/*`) chỉ còn: token `azp = eduworld-portal`, `user_type = staff` (guard `portal_staff`) và Principal có role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED`, ghi `authz.denied`). Bỏ kiểm tra `acr`/`amr`/`auth_time` trong `pkg/httpx`, bỏ trường `MFA` của Principal và `ACR`/`AMR`/`AuthTime` của `authz.Claims` (API công khai của `pkg/authz` đổi theo ADR này — ADR-0089 §3), guard `platform_mfa` đổi tên thành `platform_role`, bỏ biến `EW_OIDC_PLATFORM_REQUIRED_AMR` và `EW_OIDC_PLATFORM_REQUIRED_ACR`.
- `CrossTenantScope` (`tenant_ids`) giữ mọi điều kiện còn lại: `platform.cross_tenant.read`, client portal, không có tenant của phiên, `reason` thuộc danh sách đóng, audit `cross_tenant.accessed` khi dùng (ADR-0027 §6, ADR-0089).
- Bỏ mã lỗi `MFA_REQUIRED` khỏi catalog Problem Details và mọi contract; portal bỏ hộp "đăng nhập lại" (step-up).
- Giữ **giới hạn tuổi phiên của nhân sự platform** như một kiểm soát độc lập với MFA: module `identity` nhớ thời điểm nhập mật khẩu của phiên (Valkey, theo `sid`) và từ chối `token:refresh` (`401 REFRESH_TOKEN_INVALID`) của phiên có realm role `platform_staff` khi đã quá `EW_OIDC_PLATFORM_MAX_AUTH_AGE` (mặc định 8 giờ) hoặc không còn bản ghi phiên; người dùng nhập lại mật khẩu. Phiên nhân sự tenant theo client session của `eduworld-portal` (idle 30 phút, tối đa 10 giờ).

### 2b. Công cụ vận hành

- Grafana, GlitchTip, Prometheus, Alertmanager, Loki, Tempo cũng **không** dùng MFA (owner chốt 2026-11-21): chỉ truy cập qua network `admin` (VPN) và đăng nhập Keycloak bằng tài khoản có role vận hành platform; tài khoản tenant/học sinh bị từ chối; truy vấn Loki production vẫn ghi nhật ký truy cập ([F131](../../specs/131-observability-alerting-runbooks/spec.md) F131-AC-07).

### 3. Kiểm soát bù trừ (đã có và được giữ)

| Kiểm soát | Hiện trạng đã kiểm chứng trong repo |
|---|---|
| Route platform chỉ qua gateway nội bộ | `deploy/apisix/routes/routes.yaml` route `platform-internal`: `/api/v1/admin/*`, `/api/v1/sis/*`, `/api/v1/game-admin/*` chỉ trên gateway `internal` với allowlist CIDR `EW_ADMIN_CIDRS` (VPN). Gateway public không publish các route này. |
| Authorization nghiệp vụ trong Go | `authz.Require` đầu mỗi use case, permission platform-only không cấp cho role tenant (R20, `pkg/authz/catalog.go`), guard `portal_staff` + `platform_role` theo nhóm route, membership kiểm tra chéo `X-Tenant-Id` (ADR-0065). Không đổi. |
| Phiên ngắn | Access token 5 phút; client session `eduworld-portal` idle 30 phút, tối đa 10 giờ; phiên `platform_staff` tối đa 8 giờ từ lúc nhập mật khẩu (mục 2); refresh token xoay vòng, một lần dùng, trong cookie `HttpOnly; Secure; SameSite=Strict` + CSRF double-submit. |
| Chống dò mật khẩu | Keycloak brute-force detection: khóa tạm sau 5 lần sai, chờ tăng dần 60 giây đến tối đa 15 phút, không khóa vĩnh viễn. Module `identity` giới hạn đăng nhập 10 lần/phút theo username, 10 lần/phút theo `device_id`, 20 lần/giây toàn cục; APISIX có profile `portal-login` theo username/thiết bị/toàn cục. Không giới hạn theo một IP (R30). |
| Audit | Mọi lần đăng nhập portal thành công/thất bại (`auth.portal_login`, không ghi mật khẩu, username chỉ ở dạng hash), đăng xuất, đổi/quên mật khẩu; mọi lần đọc liên tenant (`cross_tenant.accessed` + metric `eduworld_tenant_filter_bypass_total`); mọi lần bị guard từ chối (`authz.denied`). |
| Chặn học sinh | Hai lớp: flow Keycloak chặn người không có `tenant_staff`/`platform_staff`; module `identity` trả `403 PORTAL_ACCESS_STAFF_ONLY`. |

**Kiểm soát chưa có (không được coi là bù trừ):** password policy của realm hiện chỉ là `length(6) and notUsername and passwordHistory(3)` và áp cho cả học sinh, nên **không** phải mật khẩu mạnh; tăng độ mạnh cho riêng nhân sự cần quyết định riêng. Chưa có cảnh báo đăng nhập bất thường (thiết bị/vị trí mới) cho tài khoản platform.

## Hệ quả

(+) Đăng nhập portal đơn giản, không cần ứng dụng xác thực, không còn trang action-token đăng ký OTP; không ai bị khóa ngoài vì mất điện thoại hay chưa đăng ký OTP. Bớt mã: step-up `MFA_REQUIRED`, hộp đăng nhập lại, email `CONFIGURE_TOTP`, mapper AMR/LoA.

(−) **Rủi ro được chấp nhận:** một mật khẩu platform bị lộ là đủ để chiếm tài khoản. Với System Admin, kẻ tấn công đọc được dữ liệu học sinh của mọi trường (đọc liên tenant bằng `tenant_ids`), ghi dữ liệu của bất kỳ tenant nào, đổi cấu hình kiểm duyệt chat, gán role platform. Với Safety Moderator: đọc tin nhắn liên quan case của mọi tenant. Gateway nội bộ chỉ giảm một phần rủi ro: đăng nhập (`/auth/portal/login`) và một số route tenant đọc được `tenant_ids` (`/cms/*`, `/lms/*`, `/tenant/analytics/*`, `/catalog/schools`) vẫn ở gateway public, nên người có mật khẩu System Admin từ Internet vẫn đọc được dữ liệu liên tenant qua các route đó. Không đạt khuyến nghị MFA cho tài khoản đặc quyền của OWASP ASVS (V2) và các khung bảo vệ dữ liệu trẻ em; cần nêu rõ trong hồ sơ bảo mật và hợp đồng với trường.

(−) Việc phải làm theo: sửa KEYCLOAK_SSO, SECURITY_ARCHITECTURE, ADMIN_PORTAL, API_DESIGN, PRODUCT_REQUIREMENTS (FR-AUTH-029/030), CONFIG_DEFAULTS, LOCAL_DEVELOPMENT, TEST_STRATEGY, CHILD_SAFETY, AGENTS R20, constitution; contract OpenAPI; realm-as-code; F004, F012, F051 và các spec nhắc MFA.

**Muốn bật lại MFA** thì cần ADR mới (khôi phục bước OTP có điều kiện trong flow, mapper AMR, guard `amr`); credential OTP cũ còn trong Keycloak nên người đã đăng ký không phải đăng ký lại.

## Phương án đã loại

- **Trang đăng nhập riêng cho vận hành platform (giữ OTP cho platform, bỏ cho tenant):** owner muốn một cách đăng nhập cho mọi tài khoản, không giữ OTP cho ai.
- **Chỉ bỏ MFA ở môi trường local:** owner muốn bỏ ở mọi môi trường, không chỉ cho phát triển.
- **Giữ MFA cho riêng System Admin và Safety Moderator:** owner từ chối, cùng lý do như trên.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-21 | Tạo ADR (Accepted, rủi ro được chấp nhận sau cảnh báo) | Owner dự án |
| 2026-11-21 | Mở rộng cho công cụ vận hành (Grafana, GlitchTip…): không MFA, chỉ VPN + role vận hành | Owner dự án |
