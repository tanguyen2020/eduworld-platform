# Research: F060 — Khung project Godot: autoload, theme sinh từ token, i18n vi/en, CI, Prism mock

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Vị trí và định dạng cấu hình môi trường

- **Decision**: `res://config/env/<env>.json` (`local`, `dev`, `staging`, `production`, `mock`), khóa: `env`, `api_base_url`, `realtime_url`, `cdn_base_url`, `login_path`, `client_version` (ghi lúc build), `is_mock` (chỉ `mock`), `feature_flags_default`. Chọn file bằng tham số dòng lệnh `--env=<name>` (debug) hoặc biến build (`export` preset ghi `env`). Debug build cho phép override `user://config_override.json`. Cấu hình thiết bị riêng `user://device.cfg` (`shared_device`, `lan_cache_url`).
- **Rationale**: KEYCLOAK_SSO §11 đã dùng đường dẫn này; JSON dễ sinh lúc build và kiểm trong CI; tách rõ cấu hình build và cấu hình thiết bị.
- **Alternatives**: `res://config/<env>.json` (GODOT_ARCHITECTURE §4 — gộp lẫn thư mục); `local.cfg` INI (LOCAL_DEVELOPMENT §4 — khác định dạng với các env khác).

## R2. Mock API cho phát triển client

- **Decision**: Prism `mock` trên spec gộp, chế độ dynamic tắt (trả `examples` cố định), compose profile `mock` cổng `4011`, APISIX không đứng trước. `mock.json` trỏ `http://localhost:4011`, `is_mock: true`; `AppState` hiển thị `MockBadge` (CanvasLayer trên cùng) khi `Config.is_mock`. Preset export phát hành loại `res://config/env/mock.json` bằng `exclude_filter`, CI quét `.pck` kiểm.
- **Rationale**: CODE_PLAN §1.2, §2 cho phép game client chạy trên Prism mock gắn nhãn (R17); trả example cố định giúp test lặp lại được.
- **Alternatives**: Mock viết tay trong GDScript (lệch spec, khó bảo trì); dùng backend thật sớm (chặn lịch Godot).

## R3. Lint kiến trúc lớp và quy ước signal

- **Decision**: Script `tools/godot/lint_layers.gd` (headless) duyệt `scenes/ui/**`, `scenes/world/**`, `scripts/world/**`, `scripts/player/**`: cấm `HTTPRequest`, `HTTPClient`, `WebSocketPeer`, `StreamPeerTCP`, gọi `ApiClient`/`RealtimeClient` trực tiếp; `tools/godot/lint_signals.gd` kiểm tên signal (`snake_case`, động từ quá khứ theo danh sách hậu tố `_ed`/bất quy tắc cho phép) và signal toàn cục chỉ trong `event_bus.gd`.
- **Rationale**: GODOT_ARCHITECTURE §6 "quy tắc review code" cần tự động hóa để R1 được kiểm bằng máy.
- **Alternatives**: Chỉ review thủ công (không có bằng chứng, constitution IX).

## R4. `SecureStorage` theo nền tảng (ADR-0086)

- **Decision**: Interface GDScript `SecureStorage` + `SecureStorageFactory`; adapter native trong plugin `addons/eduworld_platform`: Windows dùng DPAPI `CryptProtectData` scope CurrentUser qua GDExtension (ciphertext ở `user://secure/session.bin`); Android dùng Android Keystore (khóa AES-GCM 256 không xuất được, ciphertext trong `noBackupFilesDir`) qua Android plugin (Godot v2 plugin, Kotlin); Web và `shared_device = true` dùng `MemoryOnly`. iOS Keychain (`kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`) làm ở F133; macOS/Linux không phát hành trong Release 1.0 → `MemoryOnly`.
- **Rationale**: ADR-0086 mục 1–2 yêu cầu kho bảo mật của hệ điều hành; DPAPI/Keystore/Keychain gắn bí mật với user/thiết bị, sao chép file sang máy khác không giải mã được; tách interface giúp F061 test bằng `FakeSecureStorage`.
- **Alternatives**: Mã hóa bằng khóa cố định trong game (khóa lộ khi dịch ngược, R4); `ConfigFile.save_encrypted_pass` (mật khẩu nằm trong build); localStorage trên Web (không đủ an toàn, ADR-0086 chọn chỉ RAM).
