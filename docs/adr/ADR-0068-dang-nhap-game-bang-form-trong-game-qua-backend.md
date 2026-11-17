# ADR-0068 — Đăng nhập game bằng form trong game, xác thực qua backend

> **Trạng thái:** Accepted · một phần superseded by [ADR-0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) · bổ sung bởi ADR-0083 · **Ngày:** 2026-11-15 · **OQ:** — · **Supersedes:** ADR-0007 (cho game; portal giữ luồng trình duyệt) · **Bổ sung:** ADR-0063, ADR-0064, ADR-0065 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) · [STUDENT_GAME_UX_CORE](../05-ux/STUDENT_GAME_UX_CORE.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Bổ sung bởi [ADR-0083](ADR-0083-dang-nhap-game-va-tao-game-session-tach-biet.md):** `/auth/game/login` không tạo game session; client gọi `POST /game/sessions` sau đăng nhập.

> [!WARNING]
> **Một phần được thay bởi ADR-0091.** Không còn hiệu lực: mục 4 "portal giữ đăng nhập qua trang Keycloak". **Còn hiệu lực:** toàn bộ đăng nhập game.

## Bối cảnh

Mỗi lần mở game học sinh phải nhập lại mật khẩu (ADR-0064). Với luồng trình duyệt hệ thống của ADR-0007, trên PC game phải bật trình duyệt mỗi lần, gây bối rối cho học sinh nhỏ. Người duyệt chọn: **ô nhập username/password nằm trong game, gửi qua backend của EduWorld**.

## Phương án

1. Trang Keycloak trong trình duyệt hệ thống (ADR-0007): chuẩn OAuth nhất, trải nghiệm kém khi phải đăng nhập mỗi lần.
2. Game gửi thẳng tới Keycloak (Direct Access Grant từ client công khai): game phải dùng grant không khuyến nghị trên client công khai, khó kiểm soát rate limit.
3. **Form trong game → API đăng nhập của identity-service → Keycloak** (chọn).

## Quyết định

1. **Form đăng nhập vẽ bằng Godot** (S01): username, password (ẩn ký tự, nút hiện/ẩn), nút Đăng nhập, "Nhớ tên đăng nhập" (ADR-0064), "Quên mật khẩu? Nhờ thầy cô đặt lại". Không có trình duyệt, WebView hay chuyển ứng dụng.
2. **API đăng nhập game (identity-service, qua APISIX public, TLS bắt buộc):**
   - `POST /api/v1/auth/game/login` — body `{username, password, device_id, client_version, locale}`; không cần `X-Tenant-Id`. identity-service gọi token endpoint Keycloak bằng **client confidential `eduworld-game-backend`** (bật Direct Access Grants, chỉ identity-service giữ secret, game không biết secret), nhận access/refresh token của học sinh, rồi trong cùng lời gọi thực hiện kiểm tra ADR-0063 (chỉ học sinh, membership tenant, enrollment, login sync ADR-0061, một thiết bị ADR-0031). Kết quả: token + PlayerBootstrap (có `tenant_id` để gắn `X-Tenant-Id`), hoặc `403 TENANT_NOT_SELECTED` kèm `choices[]` và một `login_ticket` ngắn hạn, hoặc lỗi.
   - `POST /api/v1/auth/game/tenant:select` — `{login_ticket, tenant_id}` khi thuộc nhiều tenant (không nhập lại mật khẩu; ticket dùng một lần, 2 phút).
   - `POST /api/v1/auth/game/token:refresh` — làm mới access token bằng refresh token (đi qua backend vì refresh token thuộc client confidential).
   - `POST /api/v1/auth/game/logout` — thu hồi refresh token và phiên Keycloak (gọi khi thoát game).
   - Token vẫn là token Keycloak (ADR-0008); tenant của phiên do backend xác định và kiểm tra qua `X-Tenant-Id` + membership (ADR-0065), token game không cần claim `organization`.
3. **Biện pháp bảo vệ mật khẩu (bắt buộc):**
   - Không log body của route đăng nhập ở APISIX, backend, tracing; mật khẩu không ghi vào bất kỳ đâu, xóa khỏi bộ nhớ sau khi gọi Keycloak (best effort).
   - Rate limit: theo thiết bị 10 lần/phút, theo username 10 lần/phút, toàn cục theo năng lực (ADR-0045/0064); brute-force detection của Keycloak khóa tạm tài khoản sau 5 lần sai (1–15 phút); thông điệp lỗi chung "Tên đăng nhập hoặc mật khẩu chưa đúng" (không tiết lộ tài khoản có tồn tại hay không).
   - Chỉ cho realm role `student` đăng nhập qua client `eduworld-game-backend` (nhân sự bị từ chối `GAME_ACCESS_STUDENT_ONLY`, ADR-0031).
   - Certificate pinning trong game cho tên miền API [Proposed]; chặn đăng nhập từ phiên bản client quá cũ (`min_client_version`).
4. **Portal giữ đăng nhập qua trang Keycloak trên trình duyệt** (ADR-0066/0067) vì là ứng dụng web, có MFA.
5. Hàng chờ đăng nhập (ADR-0045) áp dụng tại API `/auth/game/login`: quá tải → `429` + `Retry-After`, game hiển thị "Đang vào lớp…".

## Hệ quả

(+) Học sinh đăng nhập liền mạch trong game ở mọi nền tảng; backend kiểm soát rate limit, khóa tài khoản, hàng chờ ở một chỗ. (−) Mật khẩu đi qua ứng dụng và backend của EduWorld (trách nhiệm bảo vệ cao hơn); dùng Direct Access Grant trên client confidential (không theo khuyến nghị OAuth 2.1 cho ứng dụng, chấp nhận có kiểm soát); khó thêm MFA/SSO cho học sinh sau này (cần ADR mới).

## Phương án đã loại

Phương án 1 (trải nghiệm kém khi đăng nhập mỗi lần); phương án 2 (client công khai dùng password grant, không kiểm soát được ở backend).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
| 2026-11-16 | Bổ sung bởi ADR-0083 | Technical Architect |
| 2026-11-17 | Một phần superseded by ADR-0091 (portal đăng nhập bằng form riêng qua backend) | Technical Architect |
