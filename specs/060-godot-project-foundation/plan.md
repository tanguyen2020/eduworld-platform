# Implementation Plan: F060 — Khung project Godot: autoload, theme sinh từ token, i18n vi/en, CI, Prism mock

**Branch**: `060-godot-project-foundation` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Tạo `godot/` theo GODOT_ARCHITECTURE §3 với `project.godot` ghim phiên bản, typed GDScript bắt buộc, autoload nền (`Config`, `Log`, `EventBus`, `AppState`, `SceneRouter`, `AudioDirector`, `Settings`) theo thứ tự §4, `ThemeService` áp Theme/density sinh bởi F005, i18n gettext vi/en + `L10n` + `LocaleFormat` + pseudo-locale, `UiGallery` debug, các lint (theme override, lớp, signal, khóa dịch), GdUnit4 và pipeline CI headless (export Windows/Android, smoke boot). Thêm Prism mock profile `mock` và cơ chế nhãn MOCK, chặn mock trong bản phát hành. Thêm lớp `SecureStorage` (interface, factory, adapter Windows DPAPI, Android Keystore, `MemoryOnly`) trong plugin `addons/eduworld_platform` để F061 giữ phiên game theo ADR-0086.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x, ghim minor trong `godot/.godot-version`) |
| Primary Dependencies | GdUnit4, gdtoolkit (`gdlint`, `gdformat`), Prism (`stoplight/prism`) cho mock; GDExtension C++ (Windows DPAPI `Crypt32`) và Android plugin Kotlin (`KeyStore` + `Cipher` AES/GCM) cho `SecureStorage` |
| Storage | `user://settings.cfg`, `user://device.cfg`, `user://device_id` (UUID thiết bị — tạo ở F061); không token trong các file này; bí mật phiên chỉ qua `SecureStorage` (DPAPI → `user://secure/session.bin` ciphertext; Android Keystore → file ciphertext riêng của app; `MemoryOnly`) — ADR-0086 |
| Testing | GdUnit4 headless (unit + scene runner), chụp ảnh `UiGallery` so baseline, script quét artifact |
| Target Platform | Windows 10+ x86_64, Android 9+ arm64 (export từ F060); iOS/Web ở F133 |
| Project Type | Game client |
| Performance Goals | UI theo [PERFORMANCE_BUDGET §9](../../docs/08-game-client/PERFORMANCE_BUDGET.md) (`ui_cpu_budget`); khung không thêm tải nền đáng kể |
| Constraints | AGENTS R1, R2, R3, R4, R14, R17, R32 · ADR-0014, ADR-0047, ADR-0056, ADR-0068, ADR-0086 |
| Scale/Scope | 7 autoload, 2 locale + pseudo, 4 density |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Mock sinh từ spec gộp; không contract mới |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Lớp Service ← Gameplay ← UI, lint lớp; client chỉ gọi APISIX (F061) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không lưu dữ liệu nghiệp vụ trên thiết bị |
| IV Server-authoritative | ✅ | Khung không tính điểm/XP; không answer key |
| V Xác thực, phân quyền | ✅ | Không secret/OIDC trong build (TC-AUTH-010); bí mật phiên chỉ trong kho bảo mật hệ điều hành, `MemoryOnly` trên máy dùng chung (ADR-0086, TC-AUTH-084) |
| VI An toàn trẻ em | ✅ | Log lọc PII; density g1_3 trước bootstrap; không dark pattern |
| VII Một nguồn sự thật, cấu hình | ✅ | Locale theo `supported_locales`; URL qua cấu hình môi trường |
| VIII UI tách logic, token | ✅ | Theme sinh từ token; lint override; UI không gọi mạng |
| IX Test có bằng chứng | ✅ | GdUnit4, ảnh baseline, TC-I18N-004…007; mock gắn nhãn, chặn khỏi bản phát hành |

## Project Structure

### Documentation (feature này)

```text
specs/060-godot-project-foundation/
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
godot/
├── project.godot  .godot-version  export_presets.cfg (không mật khẩu)  .gdlintrc
├── config/env/{local,dev,staging,production,mock}.json
├── scenes/bootstrap/{Boot.tscn,Splash.tscn,ConfigError.tscn}
├── scenes/ui/components/{Button,Card,Dialog,Toast,ProgressBar}.tscn
├── scenes/ui/debug/{UiGallery.tscn,MockBadge.tscn}
├── scripts/core/{config.gd,logger.gd,event_bus.gd,app_state.gd,scene_router.gd,settings.gd,audio_director.gd,
│                 theme_service.gd,l10n.gd,locale_format.gd,result.gd,ui_tokens.gd(sinh F005)}
├── scripts/platform/{secure_storage.gd,secure_storage_factory.gd,
│                     adapters/{dpapi,android_keystore,memory_only}_secure_storage.gd}   # iOS Keychain: F133
├── addons/eduworld_platform/{windows/ (GDExtension DPAPI), android/ (plugin Keystore), plugin.cfg}
├── resources/ui/theme/eduworld_theme*.tres (sinh F005)   resources/audio/bus_layout.tres
├── locale/{messages.pot,vi.po,en.po,xx_PS.po(sinh)}
└── tests/{unit/core/,unit/theme/,scene/,contract/,fixtures/}
tools/godot/{lint_theme_overrides.gd,lint_layers.gd,lint_signals.gd,extract_pot.gd,pseudo_locale.gd,scan_artifact.sh,screenshot_gallery.gd}
deploy/compose/docker-compose.yml  # profile mock: service api-mock (Prism) cổng 4011
.github/workflows/godot.yml
```

**Structure Decision**: Theo GODOT_ARCHITECTURE §3, §4; chỉ tạo thư mục/autoload mà F060 hiện thực, các mục còn lại do F061–F064 thêm đúng vị trí.

## Phase 0: Research

→ [research.md](research.md): R1 cấu hình môi trường; R2 mock API; R3 lint lớp và signal; R4 `SecureStorage` theo nền tảng (ADR-0086).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| Service (autoload/RefCounted) | `Config`, `Log`, `EventBus`, `AppState`, `Settings`, `ThemeService` (RefCounted, giữ bởi `AppState`), `L10n`, `LocaleFormat` (static), `SecureStorage` (RefCounted, tạo bởi `SecureStorageFactory`) | Không gọi mạng ở F060 |
| Gameplay | — | Chưa có |
| UI | `Boot`, `Splash`, `ConfigError`, `UiGallery`, `MockBadge`, component nền | Chỉ đọc `AppState`, gọi method public của service |

**EventBus** (danh sách cố định ban đầu): `session_expired()`, `network_state_changed(state: int)`, `notification_received(payload: Dictionary)`, `zone_changed(area_id: String)`, `locale_changed(locale: String)`, `density_changed(band: String)`, `ui_requested(screen_id: StringName, args: Dictionary)`.

**AppState** khung: `player: Dictionary` (rỗng tới F061), `grade_band: String` (mặc định `g1_3`), `school_policies`, `feature_flags`, `connectivity`; signal `player_changed`, `grade_band_changed`.

**ThemeService**: nạp `eduworld_theme_<band>.tres`, đặt `get_tree().root.theme`; nghe `grade_band_changed`, `Settings.font_scale_changed`, `Settings.high_contrast_changed`.

**SecureStorage** ([GODOT_ARCHITECTURE §5.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md#52-lưu-phiên-bằng-securestorage-adr-0086)): interface `is_available() -> bool`, `save(key: String, data: PackedByteArray) -> Error`, `load(key: String) -> PackedByteArray`, `erase(key: String) -> Error`. `SecureStorageFactory.create(shared_device: bool) -> SecureStorage`: `shared_device` hoặc `OS.has_feature("web")` → `MemoryOnlySecureStorage`; `Windows` → `DpapiSecureStorage` (GDExtension gọi `CryptProtectData`/`CryptUnprotectData` không cờ `CRYPTPROTECT_LOCAL_MACHINE`, ghi tạm + đổi tên nguyên tử); `Android` → `AndroidKeystoreSecureStorage` (plugin Kotlin: khóa alias `eduworld.session`, AES/GCM/NoPadding 256, IV ngẫu nhiên, ciphertext trong `context.noBackupFilesDir`); nền tảng khác → `MemoryOnly` (iOS ở F133). Adapter không log giá trị.

**L10n/LocaleFormat**: `L10n.pick(text: Dictionary) -> String` (fallback theo CONTENT_MODEL §8), `L10n.error_text(code: String) -> String` (`ui.error.<CODE>` hoặc `ui.error.generic`); `LocaleFormat.number/date/time/duration(value, locale)`.

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F005 (Theme, `UiTokens`) · F001 (CI, compose) · Tiêu thụ: F061 (thêm `AuthService`, `ApiClient` vào autoload), F062, F063, F092.

### Bảo mật, tenant, idempotency, audit

- Không secret trong `config/env/*.json`, `export_presets.cfg` (mật khẩu keystore từ secret của GitHub Environment); `scan_artifact.sh` quét `.pck`/APK/EXE tìm chuỗi `secret`, `/protocol/openid-connect/token`, `oidc_`, `is_mock`.
- Không mật khẩu/access token trên đĩa; refresh token chỉ qua `SecureStorage` (ADR-0086). F060 cung cấp lớp lưu trữ; nội dung bản ghi (`eduworld.game.session`) do F061 ghi. `scan_artifact.sh` không cần quét token (không có trong build).

### Quan sát

- `Log`: ring buffer 500 dòng, mức theo env; telemetry `i18n.missing_key` được xếp hàng cho F061 gửi qua `/game/telemetry` (khung chỉ ghi vào `AppState.pending_telemetry`).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
