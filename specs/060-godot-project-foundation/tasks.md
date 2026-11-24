# Tasks: F060 — Khung project Godot: autoload, theme sinh từ token, i18n vi/en, CI, Prism mock

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 `project.godot` (tên, renderer theo `renderer_default`, `untyped_declaration = error`, internationalization locale), `.godot-version`, `.gdlintrc` — file: `src/game/project.godot` — ✅ `project.godot` (Godot 4.7, Mobile, `untyped_declaration=2`, locale), `.godot-version`=4.7, `.gdlintrc`
- [x] T002 [P] Cài GdUnit4 (ghim commit) và gdtoolkit; target `make godot-test`, `make godot-lint`, `make godot-export-<target>` — file: `src/game/addons/gdUnit4/plugin.cfg` — ✅ GdUnit4 v6.2.3 (release chính thức, tag commit f2e4f86), gdtoolkit 4.5.0 (CI); `make godot-test`, `godot-lint`, `godot-export-windows|android|check` — `godot-export-windows` gỡ theo ADR-0101 (2026-11-24)
- [x] T003 [P] `export_presets.cfg` Windows + Android (keystore từ secret của GitHub Environment, `exclude_filter` loại `config/env/mock.json`, `tests/*`, `scenes/ui/debug/*`) — file: `src/game/export_presets.cfg` — ✅ `export_presets.cfg` Windows + Android, `exclude_filter` mock/local/dev/staging/tests/tools/debug/xx_PS; keystore qua env `GODOT_ANDROID_KEYSTORE_RELEASE_*` — preset Windows gỡ theo ADR-0101 (2026-11-24), còn preset Android

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 File cấu hình `config/env/{local,dev,staging,production,mock}.json` theo data-model (không secret, không OIDC) — file: `src/game/config/env/local.json` — ✅ `config/env/{local,dev,staging,production,mock}.json`
- [x] T005 [P] Compose profile `mock`: service `api-mock` (Prism, spec gộp, cổng 4011, `--dynamic=false`) — file: `deploy/compose/docker-compose.yml` — ✅ `deploy/compose/docker-compose.yml` service `api-mock` profile `mock` :4011, `make up-mock`; Prism trả 200 `/game/client-config` (local)
- [x] T006 [P] `result.gd` (kiểu kết quả chung) và thư mục test/fixture — file: `src/game/scripts/core/result.gd` — ✅ `scripts/core/result.gd`, `tests/fixtures/`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Project Godot đúng kiến trúc lớp, cấu hình không chứa secret (Priority: P1) 🎯 MVP

**Goal**: Autoload nền, Boot, Config/Log an toàn, lint lớp và signal.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F060-AC-01: chạy `gdlint` + mở project headless với script không typed → lỗi — file: `src/game/tests/unit/core/test_typed_project.gd` — phải FAIL trước khi code — ✅ `test_typed_project.gd` 3/3 (tiến trình con: script không typed → exit≠0)
- [x] T008 [P] [US1] Test F060-AC-02: thứ tự autoload trong `ProjectSettings` đúng §4; `Boot.tscn` chạy headless tới `Splash` — file: `src/game/tests/scene/test_boot.gd` — ✅ `test_boot.gd` 4/4
- [x] T009 [P] [US1] Test F060-AC-03: `Config` đọc đúng file theo `--env`, thiếu khóa → `ConfigError`; `scan_artifact.sh` phát hiện chuỗi cấm (TC-AUTH-010) — file: `src/game/tests/unit/core/test_config.gd` — ✅ `test_config.gd` 8/8 + `scan_artifact_test.sh` 17/17
- [x] T010 [P] [US1] Test F060-AC-04: `Log` che `password`, `access_token`, `refresh_token`, `Authorization`, họ tên trong field — file: `src/game/tests/unit/core/test_logger.gd` — ✅ `test_logger.gd` 4/4
- [x] T011 [P] [US1] Test F060-AC-05, F060-AC-06: fixture script UI có `HTTPRequest` và signal sai tên → lint fail — file: `src/game/tests/unit/core/test_lint_layers.gd` — ✅ `test_lint_layers.gd` 10/10

### Implementation cho User Story 1

- [x] T012 [US1] `config.gd` (đọc env JSON, override debug, `device.cfg`, `is_mock`) — file: `src/game/scripts/core/config.gd` — ✅ `scripts/core/config.gd`
- [x] T013 [P] [US1] `logger.gd` (structured, ring buffer 500, redaction) — file: `src/game/scripts/core/logger.gd` — ✅ `scripts/core/logger.gd`
- [x] T014 [P] [US1] `event_bus.gd` (danh sách signal cố định trong plan), `app_state.gd`, `scene_router.gd` (load threaded, màn chờ), `audio_director.gd` (bus + âm lượng) — file: `src/game/scripts/core/event_bus.gd` — ✅ `event_bus.gd`, `app_state.gd`, `scene_router.gd`, `audio_director.gd` + `resources/audio/bus_layout.tres`
- [x] T015 [US1] Khai báo autoload theo thứ tự §4; scene `Boot`, `Splash`, `ConfigError` — file: `src/game/scenes/bootstrap/Boot.tscn` — ✅ autoload theo §4; `scenes/bootstrap/{Boot,Splash,ConfigError}.tscn`
- [x] T016 [P] [US1] `lint_layers.gd`, `lint_signals.gd` (research R3), `scan_artifact.sh` — file: `src/game/tools/lint_layers.gd` — ✅ `tools/lint_layers.gd`, `lint_signals.gd`, `check_typed.gd`, `scan_artifact.sh` (+ `tools/lib/*`)

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Giao diện theo Theme sinh từ token, mật độ theo khối (Priority: P1)

**Goal**: ThemeService, Settings, UiGallery, lint override.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T017 [P] [US2] Test F060-AC-07: fixture `.tscn` có override màu/font → `lint_theme_overrides.gd` fail; allowlist qua (TC-UI-003) — file: `src/game/tests/unit/theme/test_lint_theme_overrides.gd` — phải FAIL trước khi code — ✅ `test_lint_theme_overrides.gd` 6/6
- [x] T018 [P] [US2] Test F060-AC-08: density mặc định `g1_3`; đổi `grade_band` → Theme đổi, signal `density_changed` — file: `src/game/tests/unit/theme/test_theme_service.gd` — ✅ `test_theme_service.gd` 7/7
- [x] T019 [P] [US2] Test F060-AC-09: `Settings` lưu/đọc `settings.cfg`, không ghi khóa ngoài danh sách — file: `src/game/tests/unit/core/test_settings.gd` — ✅ `test_settings.gd` 4/4
- [x] T020 [P] [US2] Test F060-AC-10: `screenshot_gallery.gd` chụp 4 density × 3 locale, so baseline (TC-UI-001 phía Godot) — file: `src/game/tests/scene/test_ui_gallery_snapshot.gd` — ✅ `test_ui_gallery_snapshot.gd` 4/4 (headless) + `tools/screenshot_gallery.gd` 13 ảnh, 0 khác baseline Windows; baseline Linux tạo ở CI lần đầu [Confirmed]
- [x] T021 [P] [US2] Test F060-AC-11: quét `src/game/` không có addon/thư mục Admin UI, không import `src/cms/frontend/` (TC-UI-010) — file: `src/game/tools/check_no_admin_ui.sh` — ✅ `tools/check_no_admin_ui.sh` (OK trên project, fail với fixture `addons/antd`)

### Implementation cho User Story 2

- [x] T022 [US2] `theme_service.gd` (nạp `eduworld_theme_<band>.tres`, font scale, tương phản cao) — file: `src/game/scripts/core/theme_service.gd` — ✅ `scripts/core/theme_service.gd`
- [x] T023 [P] [US2] `settings.gd` (ConfigFile `user://settings.cfg`, signal thay đổi) — file: `src/game/scripts/core/settings.gd` — ✅ `scripts/core/settings.gd`
- [x] T024 [P] [US2] Component nền (Button, Card, Dialog, Toast, ProgressBar) chỉ dùng Theme; `UiGallery.tscn` debug — file: `src/game/scenes/ui/debug/UiGallery.tscn` — ✅ `scenes/ui/components/{Button,Card,Dialog,Toast,ProgressBar}.tscn`, `scenes/ui/debug/UiGallery.tscn`
- [x] T025 [P] [US2] `lint_theme_overrides.gd` (allowlist số/ký hiệu) — file: `src/game/tools/lint_theme_overrides.gd` — ✅ `tools/lint_theme_overrides.gd` + `tools/theme_override_allowlist.txt` (rỗng)

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Song ngữ vi/en và chịu chuỗi dài (Priority: P1)

**Goal**: Gettext, L10n, LocaleFormat, pseudo-locale, font.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T026 [P] [US3] Test F060-AC-12: `L10n.error_text` ánh xạ `ui.error.<CODE>`, thiếu → `ui.error.generic` + hàng đợi telemetry `i18n.missing_key` — file: `src/game/tests/unit/core/test_l10n.gd` — phải FAIL trước khi code — ✅ `test_l10n.gd` 4/4
- [x] T027 [P] [US3] Test F060-AC-13: `extract_pot.gd` + so khóa `vi.po`/`en.po`, thiếu → exit ≠ 0 (TC-I18N-005) — file: `src/game/tests/unit/core/test_extract_pot.gd` — ✅ `test_extract_pot.gd` 6/6
- [x] T028 [P] [US3] Scene test F060-AC-14: đổi locale → `locale_changed`, Label cập nhật không restart (TC-I18N-004 phần khung) — file: `src/game/tests/scene/test_locale_switch.gd` — ✅ `test_locale_switch.gd` 1/1 (< 1 s)
- [x] T029 [P] [US3] Test F060-AC-15: locale ngoài `supported_locales` bị bỏ qua; mặc định `vi-VN` — file: `src/game/tests/unit/core/test_locale_default.gd` — ✅ `test_locale_default.gd` 6/6
- [x] T030 [P] [US3] Test F060-AC-16: `LocaleFormat` số/ngày/giờ/thời lượng theo bảng §10.1 (TC-I18N-007) — file: `src/game/tests/unit/core/test_locale_format.gd` — ✅ `test_locale_format.gd` 5/5
- [x] T031 [P] [US3] Test F060-AC-17: chụp `xx-PS`, kiểm Label không chứa chuỗi chưa dịch và không tràn ngoài ellipsis + tooltip (TC-I18N-006 phần khung) — file: `src/game/tests/scene/test_pseudo_locale.gd` — ✅ `test_pseudo_locale.gd` 2/2
- [x] T032 [P] [US3] Test F060-AC-18: font Theme có glyph cho bộ ký tự tiếng Việt kiểm — file: `src/game/tests/unit/theme/test_font_glyphs.gd` — ✅ `test_font_glyphs.gd` 2/2

### Implementation cho User Story 3

- [x] T033 [US3] `l10n.gd` (`pick`, `error_text`), khóa dịch nền `ui.boot.*`, `ui.config_error.*`, `ui.error.*` cho mọi `ProblemCode` mà game xử lý (GODOT_ARCHITECTURE §9) — file: `src/game/scripts/core/l10n.gd` — ✅ `scripts/core/l10n.gd` (25 khóa `ui.error.*` của §9 + generic)
- [x] T034 [P] [US3] `locale_format.gd` — file: `src/game/scripts/core/locale_format.gd` — ✅ `scripts/core/locale_format.gd` (mẫu `ui.format.*` trong .po)
- [x] T035 [P] [US3] `extract_pot.gd`, `pseudo_locale.gd` (sinh `xx_PS.po` +40%), `vi.po`, `en.po` — file: `src/game/tools/extract_pot.gd` — ✅ `tools/extract_pot.gd`, `tools/pseudo_locale.gd`, `locale/{messages.pot,vi.po,en.po,xx_PS.po}` (78 khóa)

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — CI headless và mock API có nhãn (Priority: P2)

**Goal**: Workflow `godot.yml`, MOCK badge, chặn mock khỏi release, contract fixture.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T036 [P] [US4] Test F060-AC-19: path filter và thứ tự job của workflow `godot.yml` — file: `scripts/ci/test-godot-rules.sh` — phải FAIL trước khi code — ✅ `scripts/ci/godot-rules.test.mjs` 5/5 (`test-godot-rules.sh`, trong `make ci-local`)
- [x] T037 [P] [US4] Scene test F060-AC-20: `Config.is_mock` → `MockBadge` hiện trên mọi scene qua `SceneRouter` — file: `src/game/tests/scene/test_mock_badge.gd` — ✅ `test_mock_badge.gd` 2/2
- [x] T038 [P] [US4] Test F060-AC-21: `scan_artifact.sh` fail khi `.pck` release chứa `mock.json`/`is_mock`/`:4011` — file: `src/game/tools/tests/scan_artifact_test.sh` — ✅ `tools/tests/scan_artifact_test.sh` 17/17 (gồm export PCK release thật)
- [x] T039 [P] [US4] Test F060-AC-22: parse fixture JSON (example `ClientConfig`, `Problem`, `specs/examples/*.json`) thành model — file: `src/game/tests/contract/test_fixtures_parse.gd` — ✅ `tests/contract/test_fixtures_parse.gd` 5/5 (+ `scripts/core/models/{api_problem,client_config}.gd`)

### Implementation cho User Story 4

- [x] T040 [US4] `MockBadge.tscn` (CanvasLayer trên cùng, chuỗi dịch `ui.debug.mock_badge`) gắn bởi `AppState` khi `is_mock` — file: `src/game/scenes/ui/debug/MockBadge.tscn` — ✅ `scenes/ui/debug/MockBadge.tscn` (CanvasLayer 128) gắn bởi `AppState.start_ui`
- [x] T041 [US4] Script sinh fixture từ `examples` trong spec gộp và `specs/examples` — file: `src/game/tools/gen_fixtures.mjs` — ✅ `tools/gen_fixtures.mjs` (+ `--check` trong `make godot-lint`)
- [x] T042 [US4] Workflow `godot.yml` (job tuần tự, action ghim SHA): design-tokens-check → gdlint + typed + lint theme/lớp/signal + check khóa dịch → GdUnit4 → export Windows/Android → scan artifact → smoke boot (`--env=mock`) — file: `.github/workflows/godot.yml` — ✅ `.github/workflows/godot.yml` (action ghim SHA, Godot qua `scripts/ci/setup-godot.sh` kiểm SHA-512); chưa có lượt chạy GitHub Actions tại thời điểm merge — bước export Windows và job `platform-windows` gỡ theo ADR-0101 (2026-11-24)

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Kho bảo mật theo nền tảng để giữ phiên game (Priority: P1)

**Goal**: Interface `SecureStorage`, factory, adapter Android Keystore, `MemoryOnly` (ADR-0086); adapter Windows DPAPI gỡ theo ADR-0101 (2026-11-24).

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [x] T046 [P] [US5] Test F060-AC-23: factory chọn đúng adapter theo `OS.get_name()`/`OS.has_feature("web")`; round-trip `save`/`load`/`erase` với mọi adapter (TC-AUTH-082 phần lưu trữ) — file: `src/game/tests/unit/platform/test_secure_storage_factory.gd` — phải FAIL trước khi code — ✅ `test_secure_storage_factory.gd` 7/7
- [x] T047 [P] [US5] Test F060-AC-24: `shared_device = true` → `MemoryOnly`; sau khi tạo lại tiến trình không còn bản ghi (TC-AUTH-084) — file: `src/game/tests/unit/platform/test_secure_storage_shared_device.gd` — ✅ `test_secure_storage_shared_device.gd` 3/3
- [x] T048 [P] [US5] Test F060-AC-25: quét `user://`, `noBackupFilesDir`, ring buffer `Log` sau khi `save` chuỗi mẫu → không có dạng rõ (chạy trên runner Windows và emulator Android) — file: `src/game/tests/integration/platform/test_secure_storage_at_rest.gd` — ✅ `test_secure_storage_at_rest.gd` 2/2 (cipher test; DPAPI thật chạy ở job `platform-windows`; emulator Android chờ F133) — phần DPAPI/runner Windows gỡ theo ADR-0101 (2026-11-24)
- [x] T049 [P] [US5] Test F060-AC-26: bản ghi hỏng / user Windows khác / khóa Keystore bị xóa → `load` rỗng + mã lỗi, `erase` xóa; `save` lỗi → log `secure_storage_write_failed` không kèm giá trị — file: `src/game/tests/unit/platform/test_secure_storage_errors.gd` — ✅ `test_secure_storage_errors.gd` 5/5 — ca "user Windows khác"/DPAPI gỡ theo ADR-0101 (2026-11-24)
- [x] T054 [P] [US5] Unit test F060-AC-27: Windows thiếu `device.cfg`/thiếu khóa → `shared_device = true` + `MemoryOnly`; `shared_device = false` → DPAPI; Android thiếu khóa → `false` — file: `src/game/tests/unit/test_device_config_defaults.gd` — phải FAIL trước khi code — ✅ `tests/unit/test_device_config_defaults.gd` 6/6 — ca Windows/DPAPI đổi thành nền tảng `other` → `MemoryOnly` theo ADR-0101 (2026-11-24)

### Implementation cho User Story 5

- [x] T050 [US5] Interface `secure_storage.gd`, `memory_only_secure_storage.gd`, `secure_storage_factory.gd` (đọc `Config.shared_device`) — file: `src/game/scripts/platform/secure_storage.gd` — ✅ `scripts/platform/{secure_storage,secure_storage_factory}.gd`, `adapters/{memory_only,file_cipher}_secure_storage.gd`
- [x] T051 [P] [US5] GDExtension Windows: `CryptProtectData`/`CryptUnprotectData` scope CurrentUser, ghi tạm + đổi tên nguyên tử vào `user://secure/` — file: `src/game/addons/eduworld_platform/windows/dpapi_secure_storage.cpp` — ✅ `addons/eduworld_platform/windows/` (godot-cpp `godot-4.5-stable`, build ở CI; chưa build/chạy local — không có toolchain C++) — gỡ theo ADR-0101 (2026-11-24)
- [x] T052 [P] [US5] Android plugin (Kotlin): khóa AES-GCM 256 trong Android Keystore alias `eduworld.session`, ciphertext trong `noBackupFilesDir` — file: `src/game/addons/eduworld_platform/android/src/main/kotlin/vn/eduworld/platform/KeystoreSecureStorage.kt` — ✅ `addons/eduworld_platform/android/` (Kotlin, AES-GCM 256, `noBackupFilesDir`; build ở CI; chưa build local — không có JDK/SDK)
- [x] T053 [US5] Adapter GDScript gọi plugin (`dpapi_secure_storage.gd`, `android_keystore_secure_storage.gd`); thêm plugin vào preset export Windows/Android; `FakeSecureStorage` cho test F061 — file: `src/game/scripts/platform/adapters/dpapi_secure_storage.gd` — ✅ `adapters/{dpapi,android_keystore}_secure_storage.gd`, export plugin `addons/eduworld_platform/export_plugin.gd`, `tests/fixtures/fake_secure_storage.gd` — `dpapi_secure_storage.gd` và preset Windows gỡ theo ADR-0101 (2026-11-24)
- [x] T055 [US5] Mặc định `shared_device` theo nền tảng trong `Config` (đọc `shared_device_default`) — file: `src/game/scripts/core/config.gd` — ✅ `config.gd` `load_device_config` + project setting `eduworld/device/shared_device_default/<platform>`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được; F061 dùng được `SecureStorage`.

## Phase N: Polish & Cross-Cutting

- [x] T043 [P] Sửa GODOT_ARCHITECTURE §4 (bỏ `oidc_issuer`/`oidc_client_id`, đường dẫn `res://config/env/<env>.json`), §11 (bỏ intent filter `oauth2/callback`, redirect OIDC Web — D16) và LOCAL_DEVELOPMENT §4 (thay `local.cfg`), §3.1 (profile `mock`, cổng 4011) — file: `docs/08-game-client/GODOT_ARCHITECTURE.md` — ✅ GODOT_ARCHITECTURE §3 (cây thư mục), §4/§11 đã sạch OIDC (D16); LOCAL_DEVELOPMENT §3.1 (`api-mock`), §6.3, §7
- [x] T044 [P] CHANGELOG mục F060 — file: `CHANGELOG.md` — ✅ CHANGELOG mục F060
- [x] T045 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/060-godot-project-foundation/quickstart.md` — ✅ quickstart.md mục Bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3, US5 (song song) → US4 → Polish.
- Đầu ra F005 phải có trước T018, T020, T022; T015 trước mọi scene test.
- F061 bắt đầu khi US1 + US3 xong (dùng `Config`, `Log`, `L10n`, `EventBus`); phần giữ phiên của F061 (tự vào lại, đăng xuất) cần US5.

## Definition of Done

- [x] Mọi tiêu chí F060-AC-* có test pass (ghi mã test/TC) — xem quickstart.md mục Bằng chứng; phần native Keystore chạy ở CI/F133 (DPAPI gỡ theo ADR-0101)
- [x] `make godot-lint` (trừ gdlint: chỉ CI), `make godot-test` (111/111 + 17/17), `make godot-export-check`, `make godot-smoke`, `make design-tokens-check` xanh (không có thay đổi Go)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi (CONFIG_DEFAULTS không đổi giá trị)
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
