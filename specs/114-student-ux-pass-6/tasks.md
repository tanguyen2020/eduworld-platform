# Tasks: F114 — Student UX pass 6: phong cách viền hoạt hình phân tầng và màn Cài đặt mới

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F096, F097, F098, F099, F113 · ADR-0047, ADR-0057

## Phase 1: Setup

- [x] T001 Worktree `feat/f114`, `pnpm install` trong `design/`, Godot `--import` hai lần; công cụ chụp thêm màn `settings_{graphics,sound,access,info,logout}`; chụp ảnh "trước" (mã F113: Cài đặt, Giấy phép, hộp Đăng xuất, S01, HUD, Cửa hàng, hộp mua, Nhật ký quest, câu chọn một, kết quả; `g1_3`, `g8_9`) — F114-AC-16
- [x] T002 Fixture `tests/fixtures/theme/tier3_tokens_0_16_0.json` lấy từ Theme `tokens@0.16.0` trước khi đổi token (12 biến thể tầng 3 × 6 Theme) — F114-AC-04

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Token `tokens@0.17.0`: primitive (`palette.yellow.{100,400,900}`, `palette.red.800`, `palette.shadow.{scrim,scrim-nested}`, `size.border.{1,5}`, `number.ratio.096`), semantic `color.outline`, `color.overlay.*`, density `outline.*`/`depth.*`, tương phản cao `outline.*` = 3, component `choice_chip`, `section_tag`, `button.danger`, `toggle`, `slider`, `settings`, `window`; `contrastWith` cho mọi cặp mới; `design/CHANGELOG.md` — F114-AC-01
- [x] T004 Bộ sinh Theme: `raised()` (tầng 1 rest/pressed), khung, tầng 2, biến thể mới, icon theme `toggle_on/off`, `slider_knob`; `UiTokens` thêm `DENSITY_OUTLINE_*`, `DENSITY_DEPTH_*`; 17 icon trong `design/icons/game/` (MIT, `assets/LICENSES.md`); `make design-tokens` — F114-AC-01…AC-03, AC-05
- [x] T005 `ThemeService._apply_outline`: tương phản cao chép bề dày viền, cạnh dưới giữ độ sâu khối — F114-AC-06

## Phase 3: User Story 1 — Nhìn là biết cái gì bấm được (P1) 🎯 MVP

- [x] T006 [P] [US1] `tests/unit/core/test_outline_style.gd`: giá trị token, biến thể mới mọi khối, tầng 1 rest/pressed (viền, cạnh, mặt lún, nội dung dịch, cùng chiều cao), thanh trượt/công tắc, khung + tầng 2, tầng 3 so fixture, tương phản cao 3 px — F114-AC-01, F114-AC-02, F114-AC-03, F114-AC-04, F114-AC-06
- [x] T007 [US1] Chip Xu `HudChip` (tầng 2); `UiBuild.scrim()` sau `UiBuild.window`, hộp mua (lồng), Lên cấp (thay `ColorRect` alpha literal); `UiMotion.pop_in` — F114-AC-03, F114-AC-07
- [x] T008 [US1] Chip đang chọn vàng + ✓ (icon nhuộm mực bằng Theme), mọi hàng chip dùng `UiBuild.mark_chips` — F114-AC-05

## Phase 4: User Story 2 — Màn Cài đặt dễ tìm, dễ chỉnh (P1)

- [x] T009 [P] [US2] `tests/scene/settings/test_settings_panel.gd`: cửa sổ + tab + Đăng xuất ở đáy, tab đầu/tab gần nhất, chip vàng ✓, lớp phủ + phóng (Giảm chuyển động tắt), âm lượng −/+ 10% + % + lưu + đầu mút, Đồ họa `g1_3`, Trợ năng (cỡ chữ, Bật/Tắt), Ngôn ngữ song ngữ + Thông tin, hộp xác nhận + thứ tự Esc, phím Phải vào trang + chiều cao control, F096 (Giấy phép, xem lại hướng dẫn) — F114-AC-05, F114-AC-07, F114-AC-08, F114-AC-09, F114-AC-10, F114-AC-11, F114-AC-12, F114-AC-13, F114-AC-14
- [x] T010 [US2] Component `ToggleSwitch`, `VolumeRow`; `Settings.nudge_volume` + `VOLUME_STEP` — F114-AC-09, F114-AC-11
- [x] T011 [US2] `SettingsPanel.tscn` + `settings_panel.gd` dựng lại (thanh tiêu đề, cột tab, trang, Giấy phép trong vùng nội dung, `go_back`), `QualitySettings` (chip có hình, công tắc ẩn ở `g1_3`), `LogoutConfirm` (tiêu đề vàng, lớp phủ lồng, Hủy focus, Đăng xuất đỏ) — F114-AC-08…F114-AC-13
- [x] T012 [US2] Khóa dịch `ui.settings.*` vi/en, `extract_pot`, `pseudo_locale`, `test_font_coverage` — F114-AC-14

## Phase 5: User Story 3 — Cả game cùng một phong cách (P2)

- [x] T013 [US3] `test_grade_band_matrix.gd`: 7 khung nhìn Cài đặt × 4 khối × 3 cửa sổ + cỡ chữ +2; ma trận các màn hiện có vẫn xanh; sửa bố cục nếu tràn (nút một dòng trang Thông tin bỏ autowrap) — F114-AC-14, F114-AC-15
- [x] T014 [US3] GdUnit4 toàn bộ xanh — F114-AC-16

## Phase 6: Bằng chứng

- [x] T015 Ảnh "sau": mỗi tab Cài đặt, hộp Đăng xuất, Giấy phép; màn mẫu tầng 1/2 (S01, HUD, Cửa hàng, hộp mua, Nhật ký quest, câu chọn một, kết quả); 4 khối × 1280×720 + 844×390 `vi`, `en` `g4_5` 1280×720; xem ảnh theo khối, sửa lỗi, bảng khối × màn ở quickstart §5 — F114-AC-16

## Phase 7: Polish

- [x] T016 Tài liệu: DESIGN_SYSTEM §5.1 (tầng, biến thể, chip vàng thay quy tắc F096, lớp phủ, hiệu ứng mở), §5.2 (bề dày viền/cạnh theo khối), STUDENT_GAME_UX_SOCIAL §10 (bố cục S15, tab, âm lượng −/+ 10%, hộp Đăng xuất), GODOT_ARCHITECTURE §10 (component), CONFIG_DEFAULTS, CODE_PLAN, TRACEABILITY_MATRIX, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md`
- [x] T017 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `bake_art --check`, `brand_assets --check`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `check_no_admin_ui`, `make design-tokens-check`, export Web + `check_web_export`, `node --test src/game/web/tests/`, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 (ảnh "trước", fixture tầng 3 trước khi đổi token) → Phase 2 (token, bộ sinh, ThemeService) → US1 → US2 → US3 → Phase 6 → Phase 7.

## Definition of Done

- Mọi F114-AC-* có test hoặc bằng chứng ảnh pass (quickstart §5); ảnh trong `screenshots/`.
- Gate T017 pass; không literal màu/cỡ; không đổi contract; tầng 3 giữ nguyên giá trị.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
