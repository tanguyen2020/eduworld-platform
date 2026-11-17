# Research: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Takeover không giữ mật khẩu (ADR-0083)

- **Bối cảnh**: Bản trước của contract cho `gameLogin` tạo game session và trả `409` không kèm ticket, buộc client giữ mật khẩu để gọi lại. ADR-0083 tách hai bước: `gameLogin` chỉ trả token + bootstrap; `409 ACTIVE_SESSION_ON_OTHER_DEVICE` chỉ đến từ `createGameSession`, lúc client đã có token. AC-UX-0103 yêu cầu "Tiếp tục ở đây" vào game không nhập lại mật khẩu.
- **Decision**: `PasswordBuffer` (`PackedByteArray`) chỉ sống trong lúc request `gameLogin` đang chạy, kể cả vòng thử lại `429 LOGIN_QUEUE_BUSY`; mọi phản hồi cuối (200, 401, 403, 422, 426, hủy, lỗi mạng) ghi `0` rồi `clear()` ngay. Takeover: `GameSessionService` gọi lại `createGameSession` với `takeover: true`, cùng access token, `Idempotency-Key` mới (body khác → key cũ bị `409 IDEMPOTENCY_KEY_REUSED`). "Ở lại máy kia" gọi `gameLogout` để thu hồi token vừa cấp.
- **Rationale**: Không giữ mật khẩu cho takeover (ADR-0083 Hệ quả); UX không đổi.
- **Alternatives**: Giữ mật khẩu trong RAM khi hộp xác nhận mở (đề xuất cũ) — bị loại sau ADR-0083; `login_ticket` cho 409 — không cần vì 409 không còn ở đăng nhập.

## R2. Gọi HTTP trên mọi nền tảng

- **Decision**: `ApiClient` dùng pool `HTTPRequest` (tối đa 4 song song, GAME_NETWORKING §2.4); Web export dùng cùng `HTTPRequest` (Godot ánh xạ sang `fetch`), CORS chỉ origin game.
- **Rationale**: Một đường mã, test được bằng `FakeHttp` thay `HttpTransport`.
- **Alternatives**: `HTTPClient` thủ công — nhiều mã hơn, không cần thiết.

## R3. Đồng hồ hết hạn token

- **Decision**: Hạn access token tính từ `expires_in` theo đồng hồ đơn điệu `Time.get_ticks_msec()`; refresh khi còn dưới 60 giây hoặc gặp `401` (GAME_NETWORKING §3).
- **Rationale**: Không phụ thuộc giờ hệ thống bị chỉnh.

## R4. Giữ phiên và tự vào lại (ADR-0086)

- **Bối cảnh**: ADR-0086 thay ADR-0064 mục 1, 2, 4, 5: trên thiết bị cá nhân refresh token được lưu trong kho bảo mật của hệ điều hành; mở lại game khi phiên còn hạn không phải đăng nhập lại. Web và `shared_device` giữ hành vi chỉ RAM.
- **Decision**: `SessionStore` (bọc `SecureStorage` của F060) giữ một bản ghi `eduworld.game.session` = `{refresh_token, tenant_id, saved_at}`. Khởi động: có bản ghi → `refreshGameToken` `mode: "resume"` (+ `device_id`, `client_version`, `X-Tenant-Id` đã lưu) → `200` dùng `bootstrap` trong response, không cần gọi thêm API bootstrap. Phân loại kết quả: `401 REFRESH_TOKEN_*`/`403` = thất bại xác định → xóa bản ghi, form; mạng/5xx/429 = tạm thời → giữ bản ghi, thử lại; `426` → ForceUpdate. Refresh single-flight, ghi đè bản ghi ngay sau mỗi token mới. Đóng cửa sổ trên thiết bị cá nhân chỉ `endGameSession`; nút Đăng xuất mới gọi `gameLogout` và xóa bản ghi.
- **Rationale**: Rotation phía server coi token cũ dùng lại là dấu hiệu bị đánh cắp và thu hồi cả phiên — client phải không bao giờ gửi song song hay gửi lại token cũ; `tenant_id` cần cho header `X-Tenant-Id` bắt buộc của `refreshGameToken`; giữ bản ghi khi lỗi mạng tránh bắt học sinh nhập mật khẩu chỉ vì Wi-Fi trường chập chờn.
- **Alternatives**: Gọi `gameLogout` khi đóng cửa sổ (hành vi ADR-0064 — trái ADR-0086 trên thiết bị cá nhân); lưu cả access token (không cần, access 5 phút); checkbox "Ghi nhớ đăng nhập" do học sinh chọn (ADR-0086 quyết định theo loại thiết bị, không thêm lựa chọn).
