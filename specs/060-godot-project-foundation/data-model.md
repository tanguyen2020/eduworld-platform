# Data Model: F060 — Khung project Godot: autoload, theme sinh từ token, i18n vi/en, CI, Prism mock

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F060 không có dữ liệu bền vững phía server. Trên thiết bị chỉ có file cấu hình/cài đặt (không dữ liệu nghiệp vụ, không token) và bản ghi trong kho bảo mật `SecureStorage` do F061 ghi (ADR-0086).

File để tham chiếu khi code:

| File | Khóa | Kiểu | Ghi chú |
|---|---|---|---|
| `res://config/env/<env>.json` | `env` | String | `local`/`dev`/`staging`/`production`/`mock` |
| | `api_base_url`, `realtime_url`, `cdn_base_url` | String (URL) | Không hardcode trong script |
| | `login_path` | String | `/api/v1/auth/game/login` (ADR-0068) |
| | `client_version` | String SemVer | Ghi lúc build |
| | `is_mock` | bool | Chỉ có trong `mock.json` |
| `user://settings.cfg` (`ConfigFile`) | `[ui] locale`, `font_scale` (0/1/2), `high_contrast`, `reduce_motion` | String/int/bool | `locale` ∈ `supported_locales` |
| | `[audio] music`, `sfx`, `voice` | float 0–1 | |
| `user://device.cfg` | `shared_device`, `lan_cache_url` | bool / String | Theo tham số `shared_device_idle_logout`, `lan_cache_url`; `shared_device = true` → `SecureStorage` `MemoryOnly` |
| `SecureStorage` (khóa do F061 đặt, ví dụ `eduworld.game.session`) | giá trị `PackedByteArray` | bytes | Windows: DPAPI CurrentUser → `user://secure/<key>.bin` (ciphertext); Android: AES-GCM với khóa Keystore → `noBackupFilesDir/<key>.bin`; `MemoryOnly`: chỉ RAM. Không bao giờ ghi rõ (ADR-0086) |
| `res://locale/*.po` | `msgid` = khóa `ui.<screen>.<element>[.<state>]` | gettext | Khóa bất biến |
