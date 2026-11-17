# Research: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

**Plan**: [plan.md](plan.md)

## R1. Giữ token khi trả `TENANT_NOT_SELECTED`

- **Decision**: Sau password grant thành công, nếu học sinh thuộc nhiều tenant, module identity mã hóa token set (AES-GCM, khóa từ secret manager) và lưu Valkey `ew:identity:login_ticket:<sha256(ticket)>` cùng `user_id`, `device_id`, `choices[]`, TTL `game_login_ticket_ttl`; `GETDEL` khi chọn để bảo đảm dùng một lần.
- **Rationale**: Không bắt nhập lại mật khẩu (ADR-0063), không băm mật khẩu lần hai (ADR-0045); dữ liệu tạm hợp ADR-0004.
- **Alternatives**: Lưu mật khẩu để gọi lại Keycloak — bị loại (vi phạm FR-AUTH-003). Trả token ngay kèm danh sách tenant — bị loại (client có token khi chưa có tenant, trái ADR-0065).

## R2. Nguồn `min_client_version`

- **Decision**: Cấu hình triển khai dùng chung `EW_GAME_MIN_CLIENT_VERSION` đọc bởi module identity (`gameLogin`) và module game (`getClientConfig`, F014) [Proposed]; đăng ký tên `min_client_version` trong CONFIG_DEFAULTS §16.
- **Rationale**: Kiểm `426` trước khi gọi Keycloak mà không gọi module game đồng bộ (R30).
- **Alternatives**: identity gọi `getClientConfig` — bị loại (R30). Projection từ event của module game — chưa có event; có thể chuyển sau.

## R3. Game session tách khỏi đăng nhập

- **Decision** (đã chốt bởi ADR-0083): `gameLogin` → `createGameSession` (F014) → realtime ticket. Bỏ `game_session_id`, `takeover`, `409`/`other_device` khỏi contract đăng nhập (đã áp dụng vào spec gộp); `PlayerBootstrap.game_profile` thành tùy chọn.
- **Rationale**: ADR-0031, ADR-0045 (SLO tách hai lời gọi), AGENTS R29/R30, DATABASE_DESIGN §3.12, TC-AUTH-015/016 đều dùng `POST /game/sessions`.
- **Alternatives**: Login tạo luôn session (spec cũ, đề xuất D15 ban đầu) — bị loại trong ADR-0083 vì cần identity gọi đồng bộ module game trên đường đăng nhập (R30).

## R4. Giữ phiên game bằng refresh token: rotation, dùng lại, gắn thiết bị (ADR-0086)

- **Decision**: module identity bọc refresh của Keycloak bằng aggregate `GameAuthSession` (`identity.game_auth_sessions`): lưu `sid`, `device_id`, SHA-256 của refresh token hiện hành và ≤ 20 hash đã xoay. Phân loại trước khi gọi Keycloak: hash hiện hành + đúng thiết bị → refresh; hash đã xoay → `REFRESH_TOKEN_REUSED` + thu hồi phiên; sai thiết bị → `REFRESH_TOKEN_DEVICE_MISMATCH`; không thấy/đã thu hồi → `REFRESH_TOKEN_INVALID`. Keycloak vẫn bật `revokeRefreshToken=true`, reuse 0 làm lớp thứ hai.
- **Rationale**: So khớp theo hash token đã cấp nên không thể giả token để thu hồi phiên người khác (khác với dựa vào claim `jti`/`sid` chưa xác minh — khóa ký refresh token là khóa HMAC của realm, service không có). Lỗi tách mã giúp client và audit phân biệt. Không lưu token thô (F013-AC-15).
- **Alternatives**: Dựa vào thông điệp lỗi Keycloak ("Stale token") để nhận diện reuse — bị loại (phụ thuộc chuỗi lỗi, không ổn định giữa phiên bản). Offline token — bị loại (ADR-0086). Thu hồi phiên khi sai thiết bị — bị loại (kẻ có token bị đánh cắp có thể cố tình gọi sai thiết bị để đăng xuất nạn nhân; ADR-0086 chỉ yêu cầu từ chối).
