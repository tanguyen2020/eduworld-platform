# Research: F004 — Keycloak realm as code và APISIX gateway (route, `tenant-header`, gateway nội bộ)

> **ADR-0094 (2026-11-21, owner bỏ MFA/TOTP cho đăng nhập Admin Portal):** mọi nội dung về OTP/TOTP, mapper AMR/LoA, `acr`/`amr`, `MFA_REQUIRED`, step-up, `configure_otp`, `otp_setup_email_sent`, guard `platform_mfa` trong file này là **lịch sử, không còn hiệu lực**. Hiện hành: [spec.md — Clarifications, Session 2026-11-21](spec.md) và [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md).

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Hiện thực kiểm tra `X-Tenant-Id` ở gateway

- **Decision**: Lua plugin riêng `ew-tenant-header` (`deploy/apisix/plugins/ew-tenant-header.lua`, priority trước `limit-count`), cấu hình `mode: required | optional`, trả Problem Details JSON (`type`, `title`, `status`, `code`, `trace_id` lấy từ `traceparent`/`request-id`), set `ctx.var.ew_tenant_id` để `limit-count` (`key_type: var`) và logger/OTel dùng.
- **Rationale**: Dùng lại trên mọi route bằng tên plugin, test được bằng `busted`/test-nginx, kiểm schema cấu hình; tránh lặp mã Lua trong `serverless-pre-function` ở hàng trăm route. API_DESIGN §10 và DEPLOYMENT §5 để ngỏ hai lựa chọn [Confirmed].
- **Alternatives**: `serverless-pre-function` (khó test, khó review, lặp); kiểm hoàn toàn ở backend (mất khóa rate limit theo tenant và không chặn sớm).

## R2. Giới hạn đăng nhập theo username mà không ghi body

- **Decision**: Lua plugin `ew-login-key` chỉ trên `/auth/game/login` và `/auth/game/tenant:select`: đọc body (≤ 4 KB), parse JSON, lấy `username`, chuẩn hóa (trim, lowercase NFC), băm HMAC-SHA256 với khóa từ env `EW_APISIX_LOGIN_KEY_SECRET`, đặt `ctx.var.ew_login_user_hash`; không lưu body vào `ctx` cho logger. `limit-count` thứ hai dùng `key = ew_login_user_hash`; `limit-count` thứ ba dùng `X-Device-Id`; ngưỡng toàn cục dùng `limit-count` `key_type: constant` trả `429 LOGIN_QUEUE_BUSY` + `Retry-After` có jitter.
- **Rationale**: ADR-0068 yêu cầu giới hạn theo username và không log mật khẩu; băm khóa tránh lộ username trong Valkey/log.
- **Alternatives**: Giới hạn username chỉ trong module identity (vẫn làm ở F013 như lớp hai, nhưng gateway giảm tải Keycloak sớm hơn); theo IP (cấm, R30).

## R3. Hành vi khi Valkey (bộ đếm) lỗi

- **Decision**: `limit-count` policy `redis` với `allow_degradation: true` cho route thường; route đăng nhập dùng thêm `limit-conn` (bộ nhớ cục bộ) làm trần an toàn; alert khi degradation.
- **Rationale**: Không chặn toàn bộ học sinh khi Valkey lỗi; bảo vệ Keycloak bằng trần cục bộ; quyền vẫn do backend kiểm.
- **Alternatives**: Fail-closed (gây mất dịch vụ toàn cục khi Valkey lỗi).

## R5. Spike direct grant cho portal (ADR-0091) — đã chạy 2026-11-18 trên Keycloak 26.4 (compose), script `deploy/keycloak/scripts/spike-direct-grant.mjs`

**Kết quả đo**:

| Câu hỏi | Kết quả | Hệ quả |
|---|---|---|
| (a) `amr`/`acr` | `amr` đúng khi mỗi execution có authenticator config **riêng** với `default.reference.value` (`pwd`, `otp`) + mapper `oidc-amr-mapper`: không OTP → `["pwd"]`, có OTP → `["pwd","otp"]`. `acr` **luôn `1`** trong direct grant (LoA chỉ tính ở browser flow; mapper `oidc-acr-mapper` không nâng mức) | MFA của token portal xác định bằng `amr` chứa `otp`: `httpx` nhận `acr ∈ EW_OIDC_PLATFORM_REQUIRED_ACR` **hoặc** `amr ∩ EW_OIDC_PLATFORM_REQUIRED_AMR` (mặc định `otp`), cùng `auth_time` ≤ `EW_OIDC_PLATFORM_MAX_AUTH_AGE` (KEYCLOAK_SSO §3.3 đã nêu cả hai kiểm tra). Không cần ADR mới |
| (b) required action | Mật khẩu đúng + `UPDATE_PASSWORD`/`CONFIGURE_TOTP` → `400 invalid_grant` "Account is not fully set up"; sai mật khẩu → `401 invalid_grant` "Invalid user credentials" | Phân biệt được qua status + `error_description` → `403 ACCOUNT_SETUP_REQUIRED` |
| (c) OTP chưa đăng ký | `direct-grant-validate-otp` REQUIRED với người **chưa** có OTP → **500** (`AuthenticationFlowException`), **không** tự thêm `CONFIGURE_TOTP` | Flow chỉ chạy OTP khi người dùng đã cấu hình (`conditional-user-configured`), cho mọi nhân sự; tài khoản `platform_staff` tạo mới mang sẵn required action `CONFIGURE_TOTP` (provisioning, F012) → (b). Nếu thiếu, token chỉ có `amr ["pwd"]` → guard `platform_mfa` trả `403 MFA_REQUIRED` (fail-safe) và F012 coi là `configure_otp` |
| (d) chặn học sinh / chặn nhân sự ở game | `deny-access-authenticator` trong direct grant trả **401 `text/html`** (trang lỗi theme, chứa thông điệp `denyErrorMessage`: `portalStudentDenied`/`gameStaffDenied`), không có JSON `error` | Phân biệt được với sai mật khẩu (401 JSON `invalid_grant`): module identity ánh xạ 401 + `Content-Type: text/html` → `403 PORTAL_ACCESS_STAFF_ONLY` / `GAME_ACCESS_STUDENT_ONLY` (và vẫn kiểm `user_type` ở backend) |
| (e) action-token `CONFIGURE_TOTP` | Chưa đo (cần SMTP mailpit + Admin API `execute-actions-email`, thuộc F012) | Mở ở F012 |
| `auth_time` | Token direct grant **không có** `auth_time` (note `AUTH_TIME` không được đặt, kể cả với mapper của scope `basic`) | `httpx` dùng `iat` khi thiếu `auth_time`; giới hạn `EW_OIDC_PLATFORM_MAX_AUTH_AGE` tính từ lúc nhập mật khẩu do module identity cưỡng chế khi refresh token portal của nhân sự platform (F012) |
| Khác | Tham số OTP của direct grant là `otp` (không phải `totp`); dùng lại cùng mã TOTP trong 30 s → 401 (`otpPolicyCodeReusable=false`); User Profile mặc định của Keycloak 26 bắt buộc email/họ/tên → mọi người dùng thiếu bị "Account is not fully set up": realm khai báo User Profile (component `declarative-user-profile`) với email/họ/tên **không** bắt buộc | — |

**Câu hỏi ban đầu:**

- (a) token direct grant của `eduworld-portal` có `amr` (`pwd`, `otp`) và `acr` theo LoA khi sub-flow Conditional OTP chạy/không chạy (mapper "Authentication Method Reference (AMR)" + reference value trên authenticator; ánh xạ ACR→LoA `1` mật khẩu, `2` MFA); (b) lỗi Keycloak trả khi mật khẩu đúng nhưng còn required action (`invalid_grant` "Account is not fully set up") và có phân biệt được với sai mật khẩu không; (c) conditional OTP có tự thêm required action `CONFIGURE_TOTP` cho `platform_staff` chưa đăng ký OTP; (d) thông điệp/`error` khi "Allow access only for role" chặn `student` để module identity ánh xạ `403 PORTAL_ACCESS_STAFF_ONLY`; (e) `execute-actions-email` với `CONFIGURE_TOTP` mở trang action-token theme `eduworld-ops` không cần browser login flow của client.
- **Cách làm**: Keycloak 26.x trong compose, realm dev, 4 tài khoản (`ad.truong.a` không OTP, `sa.admin` có OTP, `sa.moi` chưa OTP + mật khẩu tạm, `hs.an.3a`); ghi token đã giải mã và response lỗi vào mục này.
- **Kết quả mong đợi** (giả định của ADR-0091): (a) có; (b) phân biệt được qua `error_description`; (c) có; (d) phân biệt được; (e) có. Nếu (a) không đạt → dừng, mở câu hỏi + ADR bổ sung (spec Edge Cases).
- **Chặn**: task đăng nhập portal của F012 và F051 (trang `/login`) — xem phụ thuộc trong [tasks.md](tasks.md) (T039).

## R4. Áp realm theo môi trường

- **Decision**: `adorsys/keycloak-config-cli` với `import.var-substitution.enabled=true`; file `realm-eduworld.json` (chung, không user/secret) + `realm-eduworld.dev.json` (chỉ local: Organization, user demo, secret `change-me-local`); CI chạy hai lần để kiểm idempotent.
- **Rationale**: KEYCLOAK_SSO §11, DEPLOYMENT §6 đã chọn công cụ; var substitution cho `${env.KC_…}`.
- **Alternatives**: Terraform provider Keycloak (thêm công cụ, không có trong tài liệu).
