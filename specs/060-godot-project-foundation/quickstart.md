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
| 2 | `godot --headless --path godot res://scenes/bootstrap/Boot.tscn -- --env=local --quit-after 120` | Exit 0; log thứ tự autoload đúng §4 | F060-AC-02 |
| 3 | Thêm khóa `oidc_issuer` vào `local.json`; export Windows; `tools/godot/scan_artifact.sh` | Fail | F060-AC-03 |
| 4 | `Log.info("x", {"password": "p", "access_token": "t"})`; đọc ring buffer | Giá trị bị che | F060-AC-04 |
| 5 | Thêm `HTTPRequest.new()` vào script UI | `lint_layers.gd` fail | F060-AC-05 |
| 6 | Thêm `signal quest_update` vào `event_bus.gd` | `lint_signals.gd` fail | F060-AC-06 |
| 7 | Thêm `theme_override_colors/font_color` vào `.tscn` | `lint_theme_overrides.gd` fail | F060-AC-07 |
| 8 | Mở `UiGallery`, đặt `AppState.grade_band` = `g6_7` rồi `g1_3` | Theme đổi ngay; trước bootstrap là `g1_3` | F060-AC-08 |
| 9 | Đổi cỡ chữ, tương phản cao, giảm chuyển động; khởi động lại | Giữ nguyên; `settings.cfg` không có token | F060-AC-09 |
| 10 | CI chụp `UiGallery` 4 density × 3 locale | Khớp baseline | F060-AC-10 |
| 11 | Thêm thư mục `godot/addons/antd/` thử | CI fail | F060-AC-11 |
| 12 | `L10n.error_text("TENANT_MISMATCH")`, mã lạ | Bản dịch / `ui.error.generic` + telemetry hàng đợi | F060-AC-12 |
| 13 | Thêm `tr("ui.boot.new")` không thêm vào `.po`; CI | Fail | F060-AC-13 |
| 14 | Đổi ngôn ngữ trong `UiGallery` | Cập nhật ngay, không restart | F060-AC-14 |
| 15 | Đặt locale `fr-FR` trong `settings.cfg` | Bị bỏ qua, dùng `vi-VN` | F060-AC-15 |
| 16 | Unit test `LocaleFormat` | Đúng bảng GODOT_ARCHITECTURE §10.1 | F060-AC-16 |
| 17 | Chạy `UiGallery` locale `xx-PS` | Không chuỗi hardcode, không tràn | F060-AC-17 |
| 18 | Render chuỗi "Tiếng Việt: ằẳẵặ ỹỷỵ ữựừ" | Không glyph thiếu | F060-AC-18 |
| 19 | PR sửa `godot/scripts/core/settings.gd` | Workflow `godot.yml` chạy đúng thứ tự §11, có artifact Windows/Android | F060-AC-19 |
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
make godot-test          # godot --headless --path godot -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests
godot --headless --path godot -s res://../tools/godot/lint_theme_overrides.gd
bash tools/godot/scan_artifact.sh build/windows/EduWorld.exe
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), ảnh `UiGallery` (4 density × vi/en/xx-PS), log `scan_artifact.sh`, ảnh màn có nhãn MOCK, kết quả test `SecureStorage` trên runner Windows và thiết bị/emulator Android.
