# Quickstart: F114 — Student UX pass 6: phong cách viền hoạt hình phân tầng và màn Cài đặt mới

> Kiểm phong cách viền phân tầng qua token và màn Cài đặt S15. Không cần backend (fixture + fake).

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile`; script: `npm ci --prefix scripts`.
- Máy có GPU cho bước chụp ảnh (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Token và Theme** (F114-AC-01): `make design-tokens-check` (generated khớp nguồn, tương phản AA mọi cặp mới, semver `0.17.0` + `design/CHANGELOG.md`).
2. **Tầng 1/2/3, tương phản cao** (F114-AC-02…AC-04, AC-06): `make godot-test` — `tests/unit/core/test_outline_style.gd` (đọc StyleBox của Theme theo khối; tầng 3 so fixture `tokens@0.16.0`).
3. **Màn Cài đặt** (F114-AC-05, AC-07…AC-14): `make godot-test` — `tests/scene/settings/test_settings_panel.gd`; chơi thử: HUD → Cài đặt → chạm từng tab, −/+ âm lượng, đổi cỡ chữ/tương phản cao/giảm chuyển động, Đăng xuất → Hủy, Esc.
4. **Ma trận khối** (F114-AC-14, AC-15): `tests/scene/test_grade_band_matrix.gd` (mọi màn × 4 khối × 1280×720 / 1024×600 / 844×390, thêm 7 khung nhìn Cài đặt và cỡ chữ +2).
5. **Ảnh trước/sau** (F114-AC-16): `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --out=<dir> --tag=after --screens=settings,settings_graphics,settings_sound,settings_access,settings_info,settings_logout,about,login,hub,shop,shop_confirm,journal,activity,result --sizes=1280x720,844x390 --locales=vi-VN --band=<khối>` (một màn mỗi tiến trình), thêm `--locales=en-US --band=g4_5 --sizes=1280x720`.

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, brand_assets --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4 toàn bộ
make design-tokens-check
make godot-export-web   # preset "Web" + tools/ci/check_web_export.sh + scan_artifact.sh
node --test src/game/web/tests/
```

## 4. Bằng chứng cần nộp

- Ảnh (`screenshots/`), số test GdUnit4, kết quả lint, `design-tokens-check`, log export Web, bảng khối × màn.

## 5. Bằng chứng (2026-11-27, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/114-student-ux-pass-6/screenshots/`, 146 ảnh; `tools/capture_screens.gd`, renderer Compatibility = bản Web, fixture + fake, một màn mỗi tiến trình):

- **Trước** (mã F113): `{settings,about,settings_logout,login,hub,shop,shop_confirm,journal,activity,result}-before-1280x720-vi-{g1_3,g8_9}` (20 ảnh). Lưu ý: ảnh "trước" của các màn trong thế giới (HUD, Cài đặt, Cửa hàng, Nhật ký) ở `g8_9` thực ra có mật độ `g1_3` — lỗi CanvasLayer nêu bên dưới.
- **Sau**: `{settings,settings_graphics,settings_sound,settings_access,settings_info,settings_logout,about,login,hub,shop,shop_confirm,journal,activity,result}-after-{1280x720,844x390}-vi-{g1_3,g4_5,g6_7,g8_9}` (112 ảnh) và `…-after-1280x720-en-g4_5` (14 ảnh).

**Lỗi phát hiện khi xem ảnh và đã sửa**: (1) **Theme khối không tới UI dưới `CanvasLayer`** — HUD, cửa sổ và hộp thoại của thế giới hiện mật độ `g1_3` ở mọi khối (Godot 4 không truyền Theme của Window qua CanvasLayer; Theme dự án là `g1_3`): `ThemeService` gán Theme hiện hành cho Control gốc dưới CanvasLayer và cập nhật khi đổi khối/cỡ chữ/tương phản (test `test_canvas_layer_roots_follow_the_band_theme`); (2) công tắc vẽ bé bằng icon nút 32 px (`icon_max_width` kế thừa từ Button) → `ToggleSwitch/constants/icon_max_width = 0`; (3) nhãn mục vàng sát chữ → lề `space` × `size.space.4`; (4) câu hỏi Đăng xuất xuống dòng giữa chừng → hộp rộng `component.form_card.width`; (5) trang Giấy phép mở từ nơi khác vẫn sáng tab Ngôn ngữ → mở Giấy phép chọn tab Thông tin; (6) vòng focus hiện ngay trên tab vừa chạm bằng chuột → `grab_focus(hide_focus)` (vòng hiện khi em dùng phím/tay cầm; "Hủy" của hộp xác nhận vẫn sáng theo spec); (7) ma trận khối: nút một dòng "Xem lại hướng dẫn của Mây"/"Giấy phép" co về rộng icon (autowrap + không giãn) → bỏ autowrap.

**Bảng khối × màn** (ma trận `test_grade_band_matrix.gd` ở 1280×720 / 1024×600 / 844×390 + xem ảnh 1280×720 và 844×390):

| Màn | g1_3 | g4_5 | g6_7 | g8_9 | Ghi chú khi xem ảnh |
|---|---|---|---|---|---|
| S15 Ngôn ngữ | pass + ảnh | pass + ảnh (vi, en) | pass + ảnh | pass + ảnh | Tiêu đề song ngữ, chip vàng ✓ lún / chip trắng nổi; en "LANGUAGE / NGÔN NGỮ" |
| S15 Đồ họa | pass + ảnh (3 chip hình, không công tắc) | pass + ảnh | pass + ảnh | pass + ảnh | Công tắc Bật/Tắt viền mực ở `g4_5`–`g8_9` |
| S15 Âm thanh | pass + ảnh | pass + ảnh | pass + ảnh | pass + ảnh | −/+ vuông viền mực, "+" mờ ở 100%, chip "100%" không nhảy |
| S15 Trợ năng | pass + ảnh | pass + ảnh (en "Accessibility" vừa cột tab) | pass + ảnh | pass + ảnh | Chip cỡ chữ, 2 công tắc có chữ |
| S15 Thông tin, Giấy phép | pass + ảnh | pass + ảnh | pass + ảnh | pass + ảnh | Giấy phép trong vùng nội dung, tab Thông tin sáng; dòng kẻ 80 ký tự của giấy phép engine xuống dòng (giữ nguyên văn) |
| Hộp xác nhận Đăng xuất | pass + ảnh | pass + ảnh (en) | pass + ảnh | pass + ảnh | Tiêu đề vàng, lớp phủ lồng ≈ 70%, "Hủy" sáng, "Đăng xuất" đỏ |
| Cỡ chữ +2 (Ngôn ngữ, Âm thanh, Trợ năng, Đăng xuất) | pass | pass | pass | pass | Cột tab cuộn được, trang cuộn dọc |
| S01 Đăng nhập | pass + ảnh | pass + ảnh | pass + ảnh | pass + ảnh | Thẻ form viền mực + cạnh (bỏ bóng mờ), chip VI vàng |
| S03 HUD | pass + ảnh | pass + ảnh | pass + ảnh | pass + ảnh | Nút HUD nền xanh nhạt viền mực + cạnh; chip Xu viền mảnh; `g6_7`/`g8_9` nay nhỏ đúng mật độ |
| Cửa hàng, hộp mua | pass + ảnh | pass + ảnh | pass + ảnh | pass + ảnh | Ô vật phẩm đang chọn vàng; hộp mua có lớp phủ lồng |
| Nhật ký quest | ảnh | ảnh | ảnh | ảnh | Tab chip vàng ✓; ngoài ma trận (không đổi bố cục) |
| Câu chọn một, kết quả | pass + ảnh | pass + ảnh | pass + ảnh | pass + ảnh | Thẻ đáp án/đề bài không đổi (tầng 3); nút Thoát/Đọc to/Gợi ý có cạnh; bảng kết quả viền mực |
| Mọi màn khác của ma trận F098–F113 | pass | pass | pass | pass | Không màn nào tràn sau khi thêm cạnh dưới (tổng chiều cao nút không đổi) |

**AC → bằng chứng**:

| AC | Bằng chứng |
|---|---|
| F114-AC-01 | `make design-tokens-check` (57/57 test, tương phản 712/712 AA, generated 116 file, semver `0.17.0` so với origin/main); `test_outline_style.gd::test_tokens_hold_the_confirmed_values`, `test_every_band_theme_declares_the_new_variants` |
| F114-AC-02 | `test_tier1_buttons_have_ink_outline_edge_and_sink_when_pressed`, `test_tier1_slider_and_switch_are_outlined` |
| F114-AC-03 | `test_frames_have_an_edge_and_information_surfaces_a_thin_outline` |
| F114-AC-04 | `test_tier3_variants_keep_their_tokens_0_16_0_values` (fixture `tests/fixtures/theme/tier3_tokens_0_16_0.json`, 6 Theme) |
| F114-AC-05 | `test_settings_panel.gd::test_chosen_language_and_preset_are_yellow_chips_with_a_check`; ảnh Cài đặt, Cửa hàng, Nhật ký, S01 |
| F114-AC-06 | `test_high_contrast_outlines_are_three_px_in_every_band` |
| F114-AC-07 | `test_scrims_and_the_opening_zoom_follow_reduce_motion`; ảnh `settings_logout`, `shop_confirm` |
| F114-AC-08 | `test_window_has_title_bar_tabs_and_logout_at_the_foot` |
| F114-AC-09 | `test_volume_rows_step_by_ten_percent_and_save` |
| F114-AC-10 | `test_graphics_page_hides_switches_for_the_youngest_band`; ảnh `settings_graphics` |
| F114-AC-11 | `test_accessibility_page_applies_text_size_contrast_and_motion` |
| F114-AC-12 | `test_language_and_about_pages`, `test_licences_page_lists_godot_mit_and_third_parties` |
| F114-AC-13 | `test_logout_confirmation_and_back_order` |
| F114-AC-14 | `test_grade_band_matrix.gd::test_settings_fits_every_band`, `test_settings_fits_every_band_with_the_largest_text`, `test_settings_panel.gd::test_keyboard_reaches_the_page_and_controls_are_big_enough`; `lint_theme_overrides`, `extract_pot --check` |
| F114-AC-15 | `test_grade_band_matrix.gd` 24/24 |
| F114-AC-16 | 146 ảnh, bảng trên; GdUnit4, lint, i18n, export Web xanh (gate dưới) |

**Gate** (local): GdUnit4 toàn bộ `-a res://tests` → 1109 test case, 0 lỗi, 0 fail, 0 flaky, 11 skip (integration cần backend), 0 orphan (196/196 suite); test mới/đổi: `test_outline_style` 7, `test_settings_panel` 11, `test_theme_service` 8 (+1), `test_grade_band_matrix` 24 (+2). `check_typed` OK (564), `lint_theme_overrides` OK (466), `lint_layers` OK (269), `lint_signals` OK (339), `lint_art` OK (378), `bake_art --check` 0 stale, `brand_assets --check` OK (6), `extract_pot --check` OK (416), `pseudo_locale --check` OK, `gen_fixtures --check` OK (78), `gen_greybox_zones --check` 12 vùng, `check_no_admin_ui` OK, `scan_artifact_test` 22/22; `make design-tokens-check` OK; export Web preset "Web" exit 0 (`index.pck` 7,9 MB, `index.wasm` 39,5 MB), `check_web_export` + `scan_artifact` OK; `node --test src/game/web/tests/` 21 test: 20 pass, 1 skip (WebKit chưa cài; Playwright lấy từ `src/cms/frontend` qua `EW_PLAYWRIGHT_FROM`); UiGallery baseline `tests/scene/baselines/windows/` chụp lại (13 ảnh, Theme `0.17.0`); check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`. `gdlint` chưa cài trên máy local (chạy ở CI). Không kiểm trên điện thoại thật.
