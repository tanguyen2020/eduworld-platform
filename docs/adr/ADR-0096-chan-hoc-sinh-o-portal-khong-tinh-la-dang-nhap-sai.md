# ADR-0096 — Chặn học sinh ở Admin Portal không được tính là đăng nhập sai

> **Trạng thái:** Proposed (đã triển khai vì rủi ro khóa tài khoản trẻ em; chờ owner chốt) · một phần supersedes ADR-0066 (mục 1, lớp Keycloak), ADR-0091 (mục 5 "chặn học sinh hai lớp"), ADR-0094 (mục 1 bước "chặn người không có `tenant_staff`/`platform_staff`" trong flow, mục 3 dòng "Chặn học sinh") · **Ngày:** 2026-11-22 · **OQ:** — · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Flow direct grant `eduworld-portal-direct-grant` chặn học sinh **trong Keycloak**: sau bước mật khẩu, sub-flow `eduworld-portal-deny-non-staff` (Condition – User Role `tenant_staff`/`platform_staff` phủ định → `deny-access-authenticator`) trả lỗi, module `identity` ánh xạ thành `403 PORTAL_ACCESS_STAFF_ONLY` (ADR-0066 mục 1, ADR-0091 mục 5, ADR-0094 mục 1).

Lần chạy E2E Portal thứ hai (2026-11-22) thấy hai lần `POST /api/v1/auth/portal/login` liên tiếp của học sinh `hs.an.3a` trả `403 PORTAL_ACCESS_STAFF_ONLY` rồi `401 INVALID_CREDENTIALS`. Đo trên Keycloak 26.4 dùng một lần (realm-as-code của repo, dữ liệu dev) xác nhận nguyên nhân: Keycloak gọi brute-force detection (`AuthenticationProcessor.logFailure`) cho **mọi** authenticator trả `FAILURE`, kể cả `deny-access-authenticator`. Kết quả đo:

| Lần | Kết quả portal | `numFailures` | Tài khoản khóa tạm |
|---|---|---|---|
| 1 | `401` "Tài khoản học sinh không dùng để đăng nhập trang quản trị." | 1 | không |
| 2 (trong 1 giây) | `401` như trên | 2 | **có** (quick login check 1 s → chờ 60 s) |
| 3 | `401 invalid_grant` "Invalid user credentials" | 2 | có |
| Đăng nhập game ngay sau đó, mật khẩu đúng | `401 invalid_grant` | — | — |

Brute-force đếm theo **tài khoản** cho cả realm (KEYCLOAK_SSO §10), nên một em học sinh bấm nhầm vào cổng quản trị vài lần bị khóa tạm khỏi **game** (60 giây, tăng dần tới 15 phút sau 5 lần). Đây là rủi ro trực tiếp với trẻ em và trái mục tiêu của brute-force detection (chỉ chống đoán mật khẩu): lần thử của em đã **đúng** mật khẩu.

Keycloak 26.4 không có authenticator có sẵn nào từ chối mà không đánh dấu `FAILURE`; không có client policy chặn token theo role người dùng; đặt bước chặn trước bước mật khẩu thì lộ loại tài khoản khi chưa biết mật khẩu (trái ADR-0091 mục 1) và vẫn bị đếm. Viết SPI Java riêng thêm một artefact build/vận hành cho Keycloak mà dự án chưa có.

## Quyết định

1. Flow `eduworld-portal-direct-grant` chỉ còn **Username Validation → Password** (không OTP theo ADR-0094, không bước chặn role). Realm không còn sub-flow `eduworld-portal-deny-non-staff` và các authenticator config `ew-portal-not-tenant-staff`, `ew-portal-not-platform-staff`, `ew-portal-deny`. `check-realm.mjs` và `post-apply.mjs` chặn việc đưa bước từ chối trở lại flow portal.
2. Quyết định "chỉ nhân sự" nằm ở module `identity` **sau** khi Keycloak cấp token: use case `portalLogin` tra User EduWorld theo `(issuer, subject)`; không phải `staff` có role portal hiệu lực → `403 PORTAL_ACCESS_STAFF_ONLY`, thu hồi ngay phiên Keycloak vừa tạo (logout bằng refresh token) và audit `auth.portal_login` kết quả `denied`. Token không bao giờ tới trình duyệt.
3. Vẫn **hai lớp độc lập** với cấu hình Keycloak: (a) `portalLogin` của module `identity`; (b) mọi route portal kiểm `azp = eduworld-portal` + `user_type = staff` (guard `portal_staff`, `pkg/httpx`). Lớp Keycloak bỏ vì chính nó gây khóa tài khoản; ADR-0066 vốn đặt yêu cầu "không tin Keycloak" nên hai lớp backend đáp ứng đủ.
4. Brute-force giữ nguyên (`failureFactor` 5, chờ tăng dần 60 s → 15 phút, quick login check 1 s, không khóa vĩnh viễn): mật khẩu sai ở portal **vẫn** bị đếm và khóa tạm tài khoản (kể cả tài khoản học sinh). Lỗi trả người dùng không đổi: sai mật khẩu/khóa tạm → `401 INVALID_CREDENTIALS`; `PORTAL_ACCESS_STAFF_ONLY` chỉ sau mật khẩu đúng (ADR-0091 mục 1, không dò tài khoản).
5. Flow game `eduworld-game-direct-grant` giữ bước chặn người không có role `student` (ADR-0031, ADR-0068): học sinh không bao giờ chạm bước đó. Nhân sự thử vào game vẫn bị đếm một lần sai (khóa tạm portal tối đa theo brute-force) — [Proposed] chấp nhận, xem Hệ quả.

## Hệ quả

(+) Học sinh vào nhầm cổng quản trị bao nhiêu lần cũng không bị khóa game; thông điệp "Tài khoản học sinh không dùng được cổng quản trị" luôn hiện đúng thay vì chuyển thành "sai mật khẩu" ở lần thứ hai.
(+) Không lộ thêm thông tin tài khoản: thứ tự kiểm tra và mã lỗi giữ như ADR-0091.
(−) Keycloak tạo một phiên ngắn cho client `eduworld-portal` của học sinh (sự kiện `LOGIN` trong event log Keycloak) rồi module `identity` thu hồi; khi thu hồi lỗi (Keycloak tạm hỏng) phiên tự hết theo idle 30 phút và token không bao giờ rời backend. Lần đăng nhập đúng mật khẩu này đặt lại bộ đếm brute-force của tài khoản như mọi lần đăng nhập thành công (chỉ người biết mật khẩu làm được).
(−) Phải apply lại realm (keycloak-config-cli xóa sub-flow cũ cùng execution) ở mọi môi trường; trước khi apply, module `identity` vẫn ánh xạ lỗi từ chối của flow cũ thành `403 PORTAL_ACCESS_STAFF_ONLY`.
(−) Đối xứng ở game (nhân sự thử vào game bị đếm một lần sai) chưa xử lý: không ảnh hưởng trẻ em; nếu cần sẽ làm theo cùng mẫu bằng ADR mới.

## Phương án đã loại

- Giữ bước chặn trong Keycloak và nâng `failureFactor`/tắt quick login check: làm yếu chống đoán mật khẩu cho mọi tài khoản, vẫn khóa học sinh sau vài lần.
- Bước chặn đặt trước bước mật khẩu: lộ tài khoản học sinh khi chưa biết mật khẩu (ADR-0091) và Keycloak vẫn đếm `FAILURE`.
- Authenticator SPI tự viết ("deny không đếm brute-force"): thêm build Java, ký và nâng cấp theo từng phiên bản Keycloak, không cần thiết khi backend đã có kiểm tra tương đương.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-22 | Tạo ADR sau lần chạy E2E Portal thứ hai (rủi ro khóa tài khoản game của học sinh), đo xác nhận trên Keycloak 26.4 | E2E Portal v2, [F012 Clarifications](../../specs/012-authz-portal-session/spec.md) |
