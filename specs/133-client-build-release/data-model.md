# Data Model: F133 — Build và phát hành client Windows, Android, iOS, Web

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

> Bổ sung ADR-0086: bản ghi phiên `eduworld.game.session` do F061 ghi qua `SecureStorage` (F060); F133 chỉ đảm bảo vị trí lưu theo nền tảng trong bản phát hành — Windows: `user://secure/` (DPAPI CurrentUser); Android: `noBackupFilesDir` (khóa Keystore), loại khỏi backup; iOS: Keychain `ThisDeviceOnly`, không iCloud; Web: chỉ RAM. Không có bảng server.

Không áp dụng — F133 không tạo/sửa bảng database. Dữ liệu bền vững là file phân phối trên CDN/object storage và file cấu hình trên thiết bị; định dạng dưới đây là hợp đồng giữa CI và client (schema JSON "Cần thêm", xem [contracts](contracts/README.md)).

### Manifest phát hành — `play.<domain>/releases/<platform>/<channel>.json` (+ `.sig`)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `platform` | enum `windows`, `android`, `ios`, `web` | |
| `channel` | enum `staging`, `production` | |
| `client_version` | string | SemVer, maxLength 32 |
| `build` | integer | `github.run_number` của workflow `godot.yml` |
| `url` | string uri | maxLength 2048 |
| `sha256` | string | pattern hex 64 |
| `size_bytes` | integer | ≥ 0 |
| `min_os` | string | maxLength 32 |
| `released_at` | date-time | |

### Chỉ mục gói — `ew-game-bundles/index/<client_version>.json` (+ `.sig`)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `client_version` | string | maxLength 32 |
| `bundles[]` | `{bundle_id (≤128), version (int), url (uri ≤2048), sha256 (hex 64), size_bytes, grade_bands[] (enum g1_3…g8_9)}` | ≤ 500 mục |

### Cấu hình thiết bị — `device.cfg` (ConfigFile Godot; Windows `%ProgramData%\EduWorld\`, Android managed config/MDM)

| Khóa | Kiểu | Ràng buộc |
|---|---|---|
| `shared_device` | bool | mặc định `false` |
| `lan_cache_url` | string | `http(s)://`, ≤ 2048; sai → bỏ qua (F133-AC-18) |
| `low_data_mode` | bool | mặc định `false` |
| `preinstalled_bundles_dir` | string | ≤ 512, do bộ cài trường ghi |

### Cache thiết bị — `user://cache/bundles/<sha256>.pck`, `user://cache/media/<sha256>/…`

Khóa theo hash (bất biến); xóa LRU theo vùng khi vượt trần `build_size`; không xóa thư mục gói cài sẵn.

## 2. Vòng đời / trạng thái

Bản phát hành: `built` → `scanned` → `smoke_passed` → `signed` → `internal/staging` → (approval) `production` | `halted` (crash rate) | `superseded`.

## 3. Quy tắc kiểm tra dữ liệu

- Mọi gói/media tải về phải khớp SHA-256 trước khi nạp (`ProjectSettings.load_resource_pack`).
- Manifest/chỉ mục sai chữ ký bị bỏ qua toàn bộ.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Không phát event NATS; telemetry `content_download` qua `postTelemetry` (Cần thêm) | `TelemetryEvent` |
