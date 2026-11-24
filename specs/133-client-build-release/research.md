# Research: F133 — Build và phát hành client Windows, Android, iOS, Web

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Installer Windows và tự cập nhật

- **Decision**: Inno Setup 6 (cài per-machine vào `Program Files`, hỗ trợ cài im lặng cho phần mềm quản lý phòng máy, ghi `device.cfg` vào `%ProgramData%\EduWorld\` theo tham số dòng lệnh `/SHARED=1 /LANCACHE=<url>`); ký Authenticode cả `.exe` game và installer. Tự cập nhật: `UpdateService` đọc manifest phát hành trên CDN, tải installer mới, kiểm chữ ký manifest + SHA-256 + Authenticode rồi chạy cài im lặng.
- **Rationale**: Phòng máy cần cài im lặng/MDM; Inno Setup miễn phí, script trong Git.
- **Alternatives**: MSIX (cần chứng chỉ tin cậy và chính sách sideload ở máy trường); Microsoft Store (ngoài phạm vi Release 1.0).

## R2. Chữ ký manifest phát hành và chỉ mục gói

- **Decision**: RSA-3072, PKCS#1 v1.5 SHA-256, ký bằng `sign-manifest.mjs` (Node `crypto`) trong job gắn GitHub Environment; client kiểm bằng `Crypto.verify` + `CryptoKey` của Godot; chữ ký đặt ở file `.sig` cạnh JSON.
- **Rationale**: Godot `Crypto` hỗ trợ RSA ổn định trên mọi nền tảng export, gồm Web.
- **Alternatives**: Ed25519 (không có trong API `Crypto` của Godot 4); chỉ dựa HTTPS (không chống CDN/cache LAN bị sửa).

## R3. Tự động hóa store

- **Decision**: fastlane `supply` (Play track `internal`, promote thủ công) và `pilot`/`deliver` (TestFlight, App Store) chạy trên runner macOS cho iOS; khóa API store là secret của GitHub Environment `staging`/`production`.
- **Rationale**: Quy trình chuẩn, chạy được từ GitHub Actions (runner macOS cho iOS — ADR-0088).
- **Alternatives**: Upload tay (không tái lập, không có log).

## R4. Hosting Web

- **Decision**: Thư mục `play.<domain>/v/<client_version>/` bất biến (`.wasm`, `.pck`, `.js` nén Brotli, `Cache-Control: public, max-age=31536000, immutable`); `play.<domain>/index.html` `max-age=60` trỏ phiên bản hiện hành; export single-threaded nên không cần header COOP/COEP; `shell.html` kiểm WebGL 2 + UA trước khi tải engine.
- **Rationale**: Khớp DEPLOYMENT §9, §11.2 và PERFORMANCE_BUDGET §14; rollback tức thì.
- **Alternatives**: Bản đa luồng (cần COOP/COEP, rủi ro tương thích nhúng/iframe).

## R5. Kho bảo mật trong bản phát hành (ADR-0086)

- **Decision**: Adapter iOS Keychain viết bằng Swift trong plugin `addons/eduworld_platform/ios` (`SecItemAdd/SecItemCopyMatching/SecItemDelete`, `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false`). Android dùng plugin Keystore của F060; manifest release khai báo `android:dataExtractionRules` (API 31+) và `android:fullBackupContent` (API ≤ 30) loại thư mục phiên. Script `scripts/godot/check-secure-storage.mjs` kiểm artifact: thư viện native đúng nền tảng, manifest Android có quy tắc loại trừ, entitlement iOS không có `keychain-access-groups` lạ hay iCloud Keychain, bản Web không gọi `localStorage`/`indexedDB` cho khóa phiên.
- **Rationale**: Bí mật không được rời thiết bị qua sao lưu đám mây; kiểm tự động trong CI để không phụ thuộc review thủ công.
- **Alternatives**: `allowBackup="false"` toàn app (mất sao lưu cài đặt hợp lệ — chấp nhận làm phương án dự phòng nếu `dataExtractionRules` không đủ); lưu iOS trong file mã hóa (yếu hơn Keychain).
