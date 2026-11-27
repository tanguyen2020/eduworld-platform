# Data Model: F133 — Build và phát hành client Web, Android, iOS

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

> Bổ sung ADR-0086: bản ghi phiên `eduworld.game.session` do F061 ghi qua `SecureStorage` (F060); F133 chỉ đảm bảo vị trí lưu theo nền tảng trong bản phát hành (không có bản Windows — ADR-0101) — Android: `noBackupFilesDir` (khóa Keystore), loại khỏi backup; iOS: Keychain `ThisDeviceOnly`, không iCloud; Web: chỉ RAM. Không có bảng server.

Không áp dụng — F133 không tạo/sửa bảng database. Dữ liệu bền vững là file phân phối trên CDN/object storage và file cấu hình trên thiết bị; định dạng dưới đây là hợp đồng giữa CI và client: [`client-release-manifest.schema.json`](../../specs/schemas/client-release-manifest.schema.json), [`bundle-index.schema.json`](../../specs/schemas/bundle-index.schema.json) (T005, ví dụ trong `specs/examples/`). Chữ ký tách rời `<file>.sig` = base64 RSA-SHA256 PKCS#1 v1.5 trên đúng các byte JSON (khóa ≥ 3072 bit, research R2).

### Manifest phát hành — `play.<domain>/releases/<platform>/<channel>.json` (+ `.sig`)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `schema_version` | const `1` | Thêm khi làm T005 (giống `school-kit-index`) |
| `platform` | enum `android`, `ios`, `web` | Nền tảng phát hành (ADR-0101); không có manifest Windows |
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
| `schema_version` | const `1` | |
| `client_version` | string | SemVer, maxLength 32; client chỉ nhận chỉ mục của đúng phiên bản của nó |
| `generated_at` | date-time | |
| `bundles[]` | `{bundle_id (≤128, `^[a-z0-9][a-z0-9._-]{0,127}# Data Model: F133 — Build và phát hành client Web, Android, iOS

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

> Bổ sung ADR-0086: bản ghi phiên `eduworld.game.session` do F061 ghi qua `SecureStorage` (F060); F133 chỉ đảm bảo vị trí lưu theo nền tảng trong bản phát hành (không có bản Windows — ADR-0101) — Android: `noBackupFilesDir` (khóa Keystore), loại khỏi backup; iOS: Keychain `ThisDeviceOnly`, không iCloud; Web: chỉ RAM. Không có bảng server.

Không áp dụng — F133 không tạo/sửa bảng database. Dữ liệu bền vững là file phân phối trên CDN/object storage và file cấu hình trên thiết bị; định dạng dưới đây là hợp đồng giữa CI và client: [`client-release-manifest.schema.json`](../../specs/schemas/client-release-manifest.schema.json), [`bundle-index.schema.json`](../../specs/schemas/bundle-index.schema.json) (T005, ví dụ trong `specs/examples/`). Chữ ký tách rời `<file>.sig` = base64 RSA-SHA256 PKCS#1 v1.5 trên đúng các byte JSON (khóa ≥ 3072 bit, research R2).

### Manifest phát hành — `play.<domain>/releases/<platform>/<channel>.json` (+ `.sig`)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `schema_version` | const `1` | Thêm khi làm T005 (giống `school-kit-index`) |
| `platform` | enum `android`, `ios`, `web` | Nền tảng phát hành (ADR-0101); không có manifest Windows |
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
), version (int ≥ 1), url (https ≤2048, `…/bundles/<sha256>.pck`, không query), sha256 (hex 64), size_bytes, grade_bands[] (enum g1_3…g8_9)}` | ≤ 500 mục |

### Cấu hình thiết bị — `device.cfg` (ConfigFile Godot; Android managed config/MDM; bản Web không có `device.cfg` — ADR-0101)

| Khóa | Kiểu | Ràng buộc |
|---|---|---|
| `shared_device` | bool | mặc định `false` |
| `lan_cache_url` | string | `http(s)://`, ≤ 2048; sai → bỏ qua (F133-AC-18) |
| `low_data_mode` | bool | mặc định `false` |
| `preinstalled_bundles_dir` | string | ≤ 512, `res://`/`user://`/đường dẫn tuyệt đối, không `..`; mặc định `res://preinstalled` (gói APK trường đặt PCK ở `assets/preinstalled/<sha256>.pck`) |
| `grade_bands` | array | Tùy chọn, tập con g1_3…g8_9: khối máy tải trước (rỗng = mọi khối); thêm ở T036 |

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
| — | Không phát event NATS; telemetry `content_download` (`source`, `bytes` — không `bundle_id`) và `client_error` `bundle_index_invalid` qua `postTelemetry` (có từ F131/F080) | `TelemetryEvent` |
