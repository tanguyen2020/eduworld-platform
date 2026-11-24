# Data Model: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F061 là game client, không có bảng hay dữ liệu bền vững phía server; `identity.*` và `game.sessions`/`game.login_events` thuộc F013/F014.

Dữ liệu phía client (để coder không đoán):

| Dữ liệu | Nơi giữ | Kiểu GDScript | Vòng đời |
|---|---|---|---|
| `access_token`, `refresh_token`, `expires_at_msec`, `refresh_expires_at_msec` | `TokenStore` (RAM) | `String`, `int` | Ghi khi login/tenant:select/resume/refresh; xóa khi Đăng xuất, `session_expired`, takeover bị thay, "Ở lại máy kia", `createGameSession` lỗi, `TENANT_MISMATCH`, thoát tiến trình |
| Bản ghi phiên `eduworld.game.session` = JSON `{refresh_token, tenant_id, saved_at}` | `SessionStore` → `SecureStorage` (F060): Keystore/Keychain trên thiết bị cá nhân (Android/iOS; không còn DPAPI — ADR-0101); `MemoryOnly` trên Web/`shared_device` | `Dictionary` (`String`, UUID `String`, ISO-8601 `String`) | Ghi đè sau mỗi token mới; **giữ** khi đóng cửa sổ trên thiết bị cá nhân; xóa khi Đăng xuất, `401 REFRESH_TOKEN_*`, `403` tài khoản/tenant, 4409/heartbeat `409`, `ACCOUNT_NOT_PROVISIONED`, bản ghi hỏng (ADR-0086). Không chứa access token, mật khẩu, bootstrap |
| `tenant_id`, bootstrap | `AppState` (RAM) | `String`, `Dictionary` (parse `PlayerBootstrap`) | Như `TokenStore`; `tenant_id` thêm trong bản ghi phiên |
| `game_session_id` | `AppState` (RAM) | `String` | Từ `GameSession.game_session_id` (response `201` của `createGameSession`, ADR-0083); xóa khi `endGameSession`/bị thay |
| `Idempotency-Key` của `createGameSession` | `GameSessionService` (RAM) | UUIDv7 `String` (ADR-0089) | Một key cho mỗi lời gọi logic (giữ khi retry mạng); sinh key mới cho lời gọi `takeover: true` |
| `login_ticket`, `choices` | `AuthService` (RAM) | `String`, `Array[Dictionary]` | Xóa sau `selectGameLoginTenant` hoặc khi về form |
| Bộ đệm mật khẩu | `PasswordBuffer` (RAM) | `PackedByteArray` | Chỉ trong lúc `gameLogin` đang chạy (kể cả hàng chờ); `wipe()` ghi `0` rồi `clear()` ở mọi phản hồi cuối; không giữ cho takeover ([research R1](research.md)) |
| `device_id` | `user://device_id` (Web: localStorage) | UUID `String` | Sinh lần đầu cài (ADR-0031) |
| `last_username`, `remember_username` | `user://settings.cfg` mục `[login]` | `String` (≤ `maxLength` của `GameLoginRequest.username`), `bool` | Chỉ khi bật và không `shared_device` |
| `shared_device` | `device.cfg` (do trường cấu hình) | `bool` | Chỉ đọc; `true` → `SecureStorage` `MemoryOnly` |

Không có tệp `user://tenant.cfg`; không lưu mật khẩu, access token, `login_ticket` ở bất kỳ đâu ngoài RAM; refresh token chỉ ngoài RAM trong `SecureStorage` (ADR-0086).

Vòng đời bản ghi phiên:

```text
(không có) --login/tenant:select 200--> Stored --resume/renew 200--> Stored (token mới, ghi đè)
Stored --đóng cửa sổ (thiết bị cá nhân)--> Stored
Stored --Đăng xuất | 401 REFRESH_TOKEN_* | 403 | 4409/heartbeat 409 | bản ghi hỏng--> (không có)
```
