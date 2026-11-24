# Implementation Plan: F133 — Build và phát hành client Windows, Android, iOS, Web

**Branch**: `133-client-build-release` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Hoàn thiện workflow GitHub Actions `godot.yml` ([ADR-0088](../../docs/adr/ADR-0088-ci-cd-bang-github-actions.md)) theo GODOT_ARCHITECTURE §11: export 4 nền tảng trong image Godot ghim phiên bản (Windows/Android/Web trên runner Linux, iOS trên runner macOS), sinh `res://config/<env>.json` từ biến/secret của GitHub Environment, build gói PCK vùng + chỉ mục gói ký RSA-SHA256 lên `ew-game-bundles`, cổng dung lượng, quét artifact, smoke headless; ký và phát hành (fastlane cho Play/TestFlight, Inno Setup + Authenticode cho Windows, CDN versioned cho Web) với bước promote thủ công; client thêm `update_service` (manifest phát hành, 426), `content_downloader` (gói cài sẵn → LAN → CDN, SHA-256), `prefetch_scheduler`, `device_config`; adapter `SecureStorage` iOS Keychain và kiểm tra cấu hình kho bảo mật theo nền tảng trong build (Android loại file phiên khỏi backup, iOS không iCloud Keychain, Web chỉ `MemoryOnly` — ADR-0086); bộ cài trường; quy trình chứng nhận device matrix có báo cáo.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x, static typing), Bash, Node 22 (`scripts/godot/*.mjs`), Ruby (fastlane), Inno Setup script |
| Primary Dependencies | Godot export templates (ghim), GdUnit4, fastlane (`supply`, `pilot`), `signtool`/osslsigncode (Authenticode), `apksigner`, Inno Setup 6, Brotli, Playwright (smoke Web) |
| Storage | `ew-game-bundles` (floci, ADR-0071) qua CDN; cache trên thiết bị `user://cache/` |
| Testing | GdUnit4 headless (unit/scene), smoke headless client, Playwright cho Web, thiết bị lab (TC-PERF client) |
| Target Platform | Windows 10+ x86_64, Android 9+ arm64-v8a, iOS 15+ arm64, Web (Chrome/Edge, WebGL 2) |
| Project Type | Game client + workflow phát hành |
| Performance Goals | [PERFORMANCE_BUDGET §3, §11, §16](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| Constraints | ADR-0013, ADR-0057, ADR-0068, ADR-0086, AGENTS R3, R4, R31 |
| Scale/Scope | 4 nền tảng, ~8 vùng PCK, lab §2.1 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta: `TelemetryEvent` thêm `content_download`; schema mới `client-release-manifest`, `bundle-index` ở Phase 2; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public; tầng service/state/UI theo GODOT_ARCHITECTURE §6; không sửa backend |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Artifact/bộ cài/chỉ mục gói không có dữ liệu tenant; cache theo hash |
| IV Server-authoritative | ✅ | Không chứa answer key/câu hỏi đề (quét artifact F133-AC-03) |
| V Xác thực, phân quyền | ✅ | Không secret trong build; ký chỉ ở job gắn GitHub Environment (secret ký chỉ cấp cho environment), tag `client/v*` có ruleset bảo vệ; phát hành cần Release Manager; refresh token chỉ trong kho bảo mật hệ điều hành, không sao lưu ra đám mây (ADR-0086) |
| VI An toàn trẻ em | ✅ | Telemetry không PII; tải trước không đăng nhập học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Ngưỡng từ PERFORMANCE_BUDGET/CONFIG_DEFAULTS (`build_size`…); URL môi trường từ CI |
| VIII UI tách logic, token | ✅ | Màn "Cần cập nhật"/tiến độ tải chỉ gọi `update_service`/`content_downloader`; style từ `UiTokens`/theme sinh |
| IX Test có bằng chứng | ✅ | GdUnit4 cho service mới, smoke, báo cáo lab |

## Project Structure

### Documentation (feature này)

```text
specs/133-client-build-release/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/game/
├── export_presets.cfg                          # 4 preset, không secret
├── config/{dev,staging,production}.json        # SINH bởi scripts/godot/gen-config.mjs (không sửa tay)
├── scripts/core/{device_config.gd,update_service.gd,manifest_verifier.gd}
├── scripts/networking/{content_downloader.gd,prefetch_scheduler.gd,bandwidth_probe.gd}
├── scenes/bootstrap/{VersionCheck.tscn,UpdateRequired.tscn}   # UI chỉ gọi update_service
├── web/shell.html                                           # kiểm WebGL 2/trình duyệt trước khi tải engine
├── platform/{android,ios}/fastlane/Fastfile
├── platform/android/res/xml/data_extraction_rules.xml   # loại thư mục phiên khỏi cloud backup + device transfer (ADR-0086)
├── addons/eduworld_platform/ios/ (Swift: IosKeychainSecureStorage) · scripts/platform/adapters/ios_keychain_secure_storage.gd
└── tests/unit/{test_device_config.gd,test_content_downloader.gd,test_manifest_verifier.gd,test_update_service.gd,test_prefetch_scheduler.gd}
scripts/godot/{export.sh,gen-config.mjs,check-build-size.mjs,scan-artifact.sh,build-bundles.sh,sign-manifest.mjs,smoke.sh,check-secure-storage.mjs}
deploy/installer/windows/eduworld.iss  deploy/installer/school-kit/build-kit.sh
deploy/cdn/play/{headers.yaml,README.md}        # Brotli, Cache-Control, index cuối
.github/workflows/godot.yml                     # job (ký hiệu `godot / <job>`): godot / export-*, godot / size-check, godot / scan-artifact, godot / bundles, godot / smoke, godot / sign-*, godot / publish-*, godot / n-1-smoke
docs/12-project/perf-reports/client-<YYYY-MM-DD>.md
```

**Structure Decision**: Theo cây `src/game/` ở [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md) (`scripts/core`, `scripts/networking`, `scenes/bootstrap`); workflow theo DEPLOYMENT §10.2 và ADR-0088 (`godot.yml`; job ký hiệu `godot / <job>` như giao diện GitHub Actions). Logic tải/cập nhật là `RefCounted`/autoload service, scene chỉ hiển thị (R1).

## Phase 0: Research

→ [research.md](research.md): R1 installer Windows + tự cập nhật, R2 chữ ký manifest, R3 tự động hóa store, R4 hosting Web, R5 kho bảo mật trong bản phát hành (ADR-0086).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client, `RefCounted`) | `ManifestVerifier.verify(json, sig, pubkey) -> Result`; `DeviceConfig.parse(text) -> DeviceConfig` (kiểm `lan_cache_url`) | Không phụ thuộc Node/scene |
| application (client service) | `UpdateService` (so `client_version` với `ClientConfig`/manifest, xử lý 426); `ContentDownloader` (thứ tự nguồn preinstalled → LAN → CDN, SHA-256, telemetry `content_download`); `PrefetchScheduler` (`shared_device`, `offhours_prefetch_window`, chế độ dữ liệu thấp) | Signal: `update_required`, `bundle_downloaded`, `prefetch_completed` |
| infrastructure | `ApiClient` (F061), `HTTPRequest` pool, file `user://cache/` LRU | Không gọi API cần token khi prefetch |
| presentation | `VersionCheck.tscn`, `UpdateRequired.tscn`, `src/game/web/shell.html` | Chuỗi qua file dịch |
| infrastructure (nền tảng) | `IosKeychainSecureStorage` (plugin iOS, hiện thực interface `SecureStorage` của F060) | `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false` |
| CI | `export.sh`, `build-bundles.sh`, `sign-manifest.mjs`, `check-build-size.mjs`, `scan-artifact.sh`, `smoke.sh`, `check-secure-storage.mjs` (adapter đúng nền tảng, manifest Android loại backup, entitlement iOS không iCloud Keychain, Web không ghi localStorage/IndexedDB), fastlane, Inno Setup | Ký chỉ ở job gắn GitHub Environment |

### Tích hợp và phụ thuộc

- F060 (`SecureStorage` interface + adapter Windows/Android/`MemoryOnly`), F062 (`QualityManager`, benchmark headless), F061 (`ApiClient`, đăng nhập form, tự vào lại/đăng xuất, 426), F014 (`getClientConfig`, `min_client_version`), F130 (bucket, CDN origin, secret ký), F131 (`ClientCrashRateHigh`), F080 (ingest `content_download`).

### Bảo mật, tenant, idempotency, audit

- Khóa ký manifest (RSA-3072) chỉ trong secret manager/secret của GitHub Environment; khóa công khai nhúng qua `gen-config.mjs`.
- Upload `ew-game-bundles` theo khóa `bundles/<sha256>.pck` (ghi lại cùng khóa là no-op); chỉ mục/manifest theo `client_version`.
- Mỗi lần promote ghi `actor`, `channel`, `version` vào log workflow (artifact giữ theo chính sách retention của GitHub Actions).
- Kho bảo mật (ADR-0086): Android `data_extraction_rules.xml` + `fullBackupContent` loại thư mục phiên (plugin F060 dùng `noBackupFilesDir`, cấu hình là lớp bảo vệ thứ hai); iOS không thêm capability Keychain Sharing; Windows DPAPI CurrentUser (installer per-user hoặc per-machine đều được, bí mật theo user Windows).

### Quan sát

- Telemetry `content_download`, `load_time`, `crash`, `perf_sample` qua `postTelemetry`; dashboard Client (F131/F080); báo cáo dung lượng + chứng nhận lab mỗi release.

### Rủi ro ADR Proposed

- ADR-0079 (Windows + Android trước) nếu Accepted: US3 phần iOS/Web và job iOS/Web chuyển phase sau. ADR-0075: chỉ đổi endpoint S3 upload.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
