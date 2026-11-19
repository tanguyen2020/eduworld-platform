# Quickstart: F060 — Khung project Godot: autoload, theme sinh từ token, i18n vi/en, CI, Prism mock

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F005 (F001 cho compose/CI)
- Lệnh: `make design-tokens`, `make up` + `docker compose --profile mock up -d api-mock`, `make godot-test`, `make godot-export-windows`, `make godot-export-android` ([LOCAL_DEVELOPMENT §6.3](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: chưa cần (đăng nhập ở F061). Khi F061 xong: `hs.an.3a` (khối 3 → density `g1_3`), `hs.dung.7a` (khối 7 → `g6_7`).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Thêm `var x = 1` (không type) vào script; chạy lint | Fail `untyped_declaration` | F060-AC-01 |
| 2 | `godot --headless --path src/game res://scenes/bootstrap/Boot.tscn -- --env=local --quit-after 120` | Exit 0; log thứ tự autoload đúng §4 | F060-AC-02 |
| 3 | Thêm khóa `oidc_issuer` vào `local.json`; export Windows; `src/game/tools/scan_artifact.sh` | Fail | F060-AC-03 |
| 4 | `Log.info("x", {"password": "p", "access_token": "t"})`; đọc ring buffer | Giá trị bị che | F060-AC-04 |
| 5 | Thêm `HTTPRequest.new()` vào script UI | `lint_layers.gd` fail | F060-AC-05 |
| 6 | Thêm `signal quest_update` vào `event_bus.gd` | `lint_signals.gd` fail | F060-AC-06 |
| 7 | Thêm `theme_override_colors/font_color` vào `.tscn` | `lint_theme_overrides.gd` fail | F060-AC-07 |
| 8 | Mở `UiGallery`, đặt `AppState.grade_band` = `g6_7` rồi `g1_3` | Theme đổi ngay; trước bootstrap là `g1_3` | F060-AC-08 |
| 9 | Đổi cỡ chữ, tương phản cao, giảm chuyển động; khởi động lại | Giữ nguyên; `settings.cfg` không có token | F060-AC-09 |
| 10 | CI chụp `UiGallery` 4 density × 3 locale | Khớp baseline | F060-AC-10 |
| 11 | Thêm thư mục `src/game/addons/antd/` thử | CI fail | F060-AC-11 |
| 12 | `L10n.error_text("TENANT_MISMATCH")`, mã lạ | Bản dịch / `ui.error.generic` + telemetry hàng đợi | F060-AC-12 |
| 13 | Thêm `tr("ui.boot.new")` không thêm vào `.po`; CI | Fail | F060-AC-13 |
| 14 | Đổi ngôn ngữ trong `UiGallery` | Cập nhật ngay, không restart | F060-AC-14 |
| 15 | Đặt locale `fr-FR` trong `settings.cfg` | Bị bỏ qua, dùng `vi-VN` | F060-AC-15 |
| 16 | Unit test `LocaleFormat` | Đúng bảng GODOT_ARCHITECTURE §10.1 | F060-AC-16 |
| 17 | Chạy `UiGallery` locale `xx-PS` | Không chuỗi hardcode, không tràn | F060-AC-17 |
| 18 | Render chuỗi "Tiếng Việt: ằẳẵặ ỹỷỵ ữựừ" | Không glyph thiếu | F060-AC-18 |
| 19 | PR sửa `src/game/scripts/core/settings.gd` | Workflow `godot.yml` chạy đúng thứ tự §11, có artifact Windows/Android | F060-AC-19 |
| 20 | Boot `--env=mock` với `api-mock` chạy | Nhãn "MOCK" luôn hiện | F060-AC-20 |
| 21 | Thêm `mock.json` vào preset release; CI | Fail | F060-AC-21 |
| 22 | Chạy contract test fixture | Parse không lỗi | F060-AC-22 |
| 23 | Trên Windows và Android: `SecureStorageFactory.create(false)` → `save("t", "abc")` → `load("t")` | Đúng adapter DPAPI/Keystore; trả `abc`; Web → `MemoryOnly` | F060-AC-23 |
| 24 | Đặt `shared_device = true` trong `device.cfg`; `save` rồi thoát, mở lại, `load`; quét `user://`, kho hệ điều hành | `MemoryOnly`; không còn bản ghi (TC-AUTH-084) | F060-AC-24 |
| 25 | `save` chuỗi mẫu duy nhất; `grep` chuỗi trong `user://`, `noBackupFilesDir`, ring buffer `Log` | Không tìm thấy dạng rõ | F060-AC-25 |
| 26 | Ghi đè file `user://secure/t.bin` bằng byte ngẫu nhiên / chạy dưới user Windows khác; `load("t")` | Trả rỗng + mã lỗi, không crash; `erase` xóa được | F060-AC-26 |
| 27 | Xóa `user://device.cfg` trên Windows, chạy game và đăng nhập | `shared_device = true`, không có bản ghi trong DPAPI sau khi thoát | F060-AC-27 |

## 3. Lệnh test tự động

```bash
make design-tokens-check
make godot-test          # godot --headless --path src/game -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests
godot --headless --path src/game -s res://tools/lint_theme_overrides.gd
bash src/game/tools/scan_artifact.sh build/windows/EduWorld.exe
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), ảnh `UiGallery` (4 density × vi/en/xx-PS), log `scan_artifact.sh`, ảnh màn có nhãn MOCK, kết quả test `SecureStorage` trên runner Windows và thiết bị/emulator Android.

## Bằng chứng (2026-11-20, local — Windows 11, Godot 4.7.2 stable, GNU make qua WSL, Node 20.19)

Lệnh gate (nối `&&`): `make godot-lint GDLINT=<bỏ qua>` → `make godot-test` → `make godot-export-check` → `make godot-smoke` → `make design-tokens-check` → `node scripts/specs/check-features.mjs` → `check-cross-features` → `check-workflows` → `test-godot-rules.sh` → `test-path-filters.sh` → `test-design-rules.sh` → `gen_fixtures.mjs --check`: **exit 0**. Commit code `1f2c75a`.

| Tiêu chí | Bằng chứng tự động | Kết quả |
|---|---|---|
| F060-AC-01 | `test_typed_project.gd` (tiến trình con: `var count = 1` → exit ≠ 0, có type → 0); `tools/check_typed.gd` 66 file | 3/3 pass; OK |
| F060-AC-02 | `test_boot.gd`; `make godot-smoke` (log `boot_completed` thứ tự `Config,Log,EventBus,AppState,SceneRouter,AudioDirector,Settings` → `splash_shown`) | 4/4 pass; exit 0 |
| F060-AC-03 | `test_config.gd`; `scan_artifact_test.sh` (secret, OIDC, URL token Keycloak, PCK release thật) | 8/8; 17/17 |
| F060-AC-04 | `test_logger.gd` | 4/4 |
| F060-AC-05, AC-06 | `test_lint_layers.gd`; `lint_layers.gd` 15 file, `lint_signals.gd` 40 file | 10/10; OK |
| F060-AC-07 | `test_lint_theme_overrides.gd`; `lint_theme_overrides.gd` 51 file, allowlist rỗng | 6/6; OK |
| F060-AC-08 | `test_theme_service.gd` (g1_3 trước bootstrap, đổi band → Theme cửa sổ + `EventBus.density_changed`) | 7/7 |
| F060-AC-09 | `test_settings.gd` | 4/4 |
| F060-AC-10 | `test_ui_gallery_snapshot.gd` (4 density × 3 locale, tương phản cao); `tools/screenshot_gallery.gd` (Vulkan Mobile, Intel Iris Xe): 13 ảnh, 0 khác baseline `tests/scene/baselines/windows/` | 4/4; 0 differing |
| F060-AC-11 | `tools/check_no_admin_ui.sh` (project OK; fixture `addons/antd` + import `src/cms/frontend` → exit 1) | OK |
| F060-AC-12 | `test_l10n.gd` | 4/4 |
| F060-AC-13 | `test_extract_pot.gd`; `extract_pot.gd --check` (78 khóa, đủ vi/en, không chuỗi hardcode) | 6/6; OK |
| F060-AC-14 | `test_locale_switch.gd` (đổi vi→en < 1 s, `locale_changed`, màn đang mở cập nhật) | 1/1 |
| F060-AC-15 | `test_locale_default.gd` (`fr-FR` bị bỏ, mặc định `vi-VN`, hồ sơ → tenant) | 6/6 |
| F060-AC-16 | `test_locale_format.gd` (`1.234,5`/`1,234.5`, `15/11/2026`/`Nov 15, 2026`, `14:30`/`2:30 PM`, `3 phút 20 giây`/`3 min 20 s`) | 5/5 |
| F060-AC-17 | `test_pseudo_locale.gd` (UiGallery, Splash, ConfigError ở `xx-PS`: không chuỗi chưa dịch, không tràn); `pseudo_locale.gd --check` | 2/2; OK |
| F060-AC-18 | `test_font_glyphs.gd` (Be Vietnam Pro Regular/Bold, Baloo 2 Bold) | 2/2 |
| F060-AC-19 | `scripts/ci/godot-rules.test.mjs`; `check-workflows` 10 workflow 0 lỗi; path filters 22/22; design rules 4/4 | 5/5 |
| F060-AC-20 | `test_mock_badge.gd`; Prism `api-mock` (docker, `--dynamic=false`) trả `200` cho `GET /game/client-config`, boot `--env=mock` → `splash_shown` (container đã dừng) | 2/2 |
| F060-AC-21 | `scan_artifact_test.sh` (PCK preset "Windows Desktop" không có `local.json`, UiGallery, tests, pseudo-locale, gdUnit4; PCK có `mock.json` → fail); `make godot-export-check` | 17/17; OK |
| F060-AC-22 | `test_fixtures_parse.gd` (ClientConfig, Problem, 10 tài liệu `specs/examples`, mã lỗi ∈ `ProblemCode`); `gen_fixtures.mjs --check` | 5/5; OK |
| F060-AC-23 | `test_secure_storage_factory.gd` (factory theo nền tảng, round-trip MemoryOnly + file cipher) | 7/7 |
| F060-AC-24 | `test_secure_storage_shared_device.gd` | 3/3 |
| F060-AC-25 | `test_secure_storage_at_rest.gd` (quét toàn bộ `user://` + ring buffer `Log`) | 2/2 |
| F060-AC-26 | `test_secure_storage_errors.gd` | 5/5 |
| F060-AC-27 | `test_device_config_defaults.gd` | 6/6 |

Tổng GdUnit4: **111 test, 0 lỗi, 0 fail, 0 orphan**. `design-tokens-check`: 188/188 cặp WCAG AA, 23 file sinh khớp. `check-features`: 93 feature, 0 lỗi.

**Chưa có bằng chứng local (ghi rõ, [Proposed]):**
- `gdlint` (gdtoolkit 4.5.0) chỉ chạy trong CI — cài pip trên máy dev bị từ chối quyền.
- GDExtension DPAPI và AAR Android Keystore chưa được build/chạy local (không có MSVC/scons, JDK/Android SDK); job `platform-windows` chạy bộ test `SecureStorage` với DPAPI thật, test trên emulator Android chờ lane thiết bị F133. Khi chưa có plugin, factory trả `MemoryOnly` (đã test).
- Export Windows/Android đầy đủ cần export template (CI tải + kiểm SHA-512); local chỉ export PCK release (`make godot-export-check`).
- Baseline ảnh Linux (CI, xvfb + Compatibility) sẽ được tạo ở lượt CI đầu, cần người duyệt.
- Workflow `godot.yml` mới chưa có lượt chạy GitHub Actions tại thời điểm merge.
