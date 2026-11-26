# Implementation Plan: F114 — Student UX pass 6: phong cách viền hoạt hình phân tầng và màn Cài đặt mới

**Branch**: `114-student-ux-pass-6` · **Date**: 2026-11-26 · **Spec**: [spec.md](spec.md)

## Summary

Hai phần, chỉ phía client, trên token và component chung của F096–F099/F113:

- **Viền hoạt hình phân tầng qua token (US1, US3)**: `tokens@0.17.0` thêm `color.outline` (= `palette.ink.900`), `color.overlay.{scrim,scrim_nested}` (mực 50% / 35%), density `density.<band>.outline.{interactive,info}` (3/3/2/2 · 2), `density.<band>.depth.{rest,pressed}` (5/4/3/3 · 1), tương phản cao viền 3 px mọi khối (`high-contrast.json`), primitive `palette.yellow.{100,400,900}`, `palette.red.800`, `palette.shadow.{scrim,scrim-nested}`, `size.border.{1,5}`, `number.ratio.096`; component `component.choice_chip.*`, `section_tag.*`, `button.danger.*`, `toggle.*`, `slider.*`, `settings.*`, `window.pop_{scale,duration}`; viền panel/khung/thông tin trỏ về `color.outline`. Bộ sinh Theme (`design/sd/formats/godot-tres.mjs`) dựng **tầng 1** bằng hàm `raised()`: viền mực `o`, cạnh dưới `o + d`, nội dung căn giữa mặt nút và tổng cao = `density.<band>.button.min_height`; trạng thái nhấn: cạnh `o + 1`, `expand_margin_top = −(d − 1)` (mặt nút vẽ thấp xuống), `content_margin_top` tăng đúng `d − 1` (nội dung lún theo, chiều cao không đổi). Khung cửa sổ (Panel/PanelContainer, `FormCard`, `WindowTitleBar`, `ConfirmTitleBar`) có viền + cạnh, không nhấn, bỏ bóng mờ. **Tầng 2** (`HudPanel`, `HudChip` mới cho chip Xu, `SpeechBubble`, `InfoBannerPanel`, `InlineErrorPanel`, `ProgressBar`, `SectionTag`, `ValueChip`): viền `outline.info` bốn phía. **Tầng 3** (`StemLabel`, `QuestionCard`, `AnswerTile*`, `TextButton`) giữ nguyên byte: test so với fixture lấy từ Theme `tokens@0.16.0`. `ChoiceChip`/`PictureTile` đang chọn = nền vàng + chữ mực + ✓ (icon trắng nhuộm mực qua `icon_pressed_color`), mặt nút lún (trạng thái pressed của nút bật/tắt). `ThemeService._apply_contrast` chép bề dày viền của Theme tương phản cao, cạnh dưới giữ độ sâu của khối. Biến thể mới: `DangerButton`, `HudChip`, `SectionTag`, `ValueChip`, `Scrim`, `ScrimNested`, `WindowTitleBar`/`WindowTitleLabel`, `ConfirmTitleBar`/`ConfirmTitleLabel`, `SettingsTabColumn`, `TabButton`/`TabButtonSelected`, `ToggleSwitch` (CheckButton, icon `toggle_on`/`toggle_off`), `HSlider` (rãnh + phần đã kéo viền mực, núm `slider_knob`). Lớp phủ: `UiBuild.scrim(nested)` sau mọi cửa sổ `UiBuild.window` (F069), hộp mua (lồng) và Lên cấp; `UiMotion.pop_in` (0,96 → 1 trong `motion.default` 0,2 s, tắt khi Giảm chuyển động) cho Cài đặt và hộp xác nhận.
- **S15 Cài đặt (US2)**: `SettingsPanel.tscn` dựng lại: `Scrim` + `Window` (khung tầng 1) → `TitleBar` (`WindowTitleLabel` in hoa "CÀI ĐẶT" + `Close` `DangerButton` icon ✕) → `Body`: `TabColumn` (`SettingsTabColumn`, rộng `component.settings.tab_column_width`; `TabScroll` cuộn 5 tab icon + chữ, `Logout` `DangerButton` ở đáy) + `Content` (ScrollContainer; trang `PageLanguage`, `PageGraphics` = `QualitySettings`, `PageSound` = 3 `VolumeRow`, `PageAccess` = chip cỡ chữ + 2 `ToggleSwitch`, `PageInfo` = Xem lại hướng dẫn, Giấy phép, phiên bản) + `LicensesPage` (thay `Content` khi mở). `current_tab` = `language` lần đầu, giữ tab dùng gần nhất trong phiên (panel sống cùng WorldRoot). `go_back()` (Esc/Back): hộp xác nhận → trang Giấy phép → cửa sổ. `focus_neighbor_right` của tab trỏ vào control đầu của trang. `VolumeRow`: icon + tên, −/+ (`Settings.nudge_volume`, `VOLUME_STEP` 0,1), `HSlider` bước 0,1, nhãn `ValueChip` "70%", −/+ vô hiệu ở 0%/100%, `accessibility_name` "Giảm/Tăng âm lượng {tên}". `ToggleSwitch` (CheckButton) viết chữ "Bật"/"Tắt" làm text và `accessibility_name` "{tên}: {trạng thái}". `QualitySettings`: 3 chip preset có hình (1/2/3 cột), công tắc bóng đổ/khử răng cưa ẩn ở `g1_3`. `LogoutConfirm` thành Control toàn khung: `ScrimNested` + `Dialog` (khung tầng 1, rộng `component.form_card.width`) → `ConfirmTitleBar` vàng "ĐĂNG XUẤT?", câu hỏi, "Tiến độ của em đã được lưu rồi nhé.", `Cancel` (`SecondaryButton`, icon `cancel`, focus mặc định) + `Confirm` (`DangerButton`, icon `exit_light`) → `AuthService.logout()` như cũ.
- **Bằng chứng (US3)**: `tools/lib/screen_capture.gd` thêm màn `settings_graphics`, `settings_sound`, `settings_access`, `settings_info`, `settings_logout`; ma trận khối thêm 7 khung nhìn Cài đặt + cỡ chữ +2.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, static typing); Node 20 (pipeline token) |
| Primary Dependencies | GdUnit4; Style Dictionary 4 + fontkit (`design/`) |
| Storage | `user://settings.cfg` hiện có (không trường mới) |
| Testing | GdUnit4 headless (unit, scene); `pnpm test` + `checks/run-all.mjs` (`design/`); capture có renderer |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | StyleBoxFlat phẳng (không bóng mờ, không shader); hiệu ứng mở cửa sổ 1 tween ≤ 0,2 s, không mỗi khung |
| Constraints | AGENTS R1, R19, R32, R33, R34 · ADR-0047, ADR-0057, ADR-0086 |
| Scale/Scope | 4 Theme khối + tương phản cao, 14 biến thể mới, 17 icon, 1 màn dựng lại, 1 hộp thoại, 3 component |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec + Clarifications [Confirmed] (owner duyệt preview bản A); không contract mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; giá trị cài đặt ở `Settings`/`QualityManager` (thêm `Settings.nudge_volume`), scene chỉ hiển thị (R1) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu mới, không trường cài đặt mới, không gửi server |
| IV Server-authoritative | ✅ | Không đổi; đăng xuất vẫn qua `AuthService.logout()` |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | "Hủy" là mặc định an toàn (focus), đỏ chỉ cho rời đi, không đỏ cho phản hồi học tập (R19) |
| VII Một nguồn sự thật, cấu hình | ✅ | Bề dày viền/cạnh sở hữu ở `DESIGN_SYSTEM` §5.2; quy tắc tầng/chip vàng/lớp phủ ở §5.1; bố cục Cài đặt và bước âm lượng ở `STUDENT_GAME_UX_SOCIAL` §10 |
| VIII UI tách logic, token | ✅ | Màu/cỡ chỉ từ token; Theme/UiTokens/CSS shell sinh bằng `make design-tokens`; không override |
| IX Test có bằng chứng | ✅ | Mỗi F114-AC có test hoặc bằng chứng ảnh (quickstart §5) |

## Project Structure

```text
design/game-tokens/{primitive,semantic,component,high-contrast}.json, design/game-tokens/density/g{1_3,4_5,6_7,8_9}.json
design/sd/formats/{godot-tres,gdscript-tokens}.mjs, design/package.json (0.17.0), design/CHANGELOG.md
design/icons/game/{tab_language,tab_graphics,tab_sound,tab_access,tab_info,exit_light,cancel,minus,plus,music,voice,quality_low,quality_medium,quality_high,toggle_on,toggle_off,slider_knob}.svg
src/game/resources/ui/theme/* + resources/ui/icons/* (sinh), src/game/scripts/core/ui_tokens.gd (sinh), src/game/web/shell_tokens.css (sinh)
src/game/scripts/core/{theme_service,settings}.gd
src/game/scenes/ui/components/{ui_motion,toggle_switch,volume_row,ui_build}.gd
src/game/scenes/ui/settings/{SettingsPanel.tscn,settings_panel.gd,QualitySettings.tscn,quality_settings.gd}
src/game/scenes/ui/dialogs/{LogoutConfirm.tscn,logout_confirm.gd}
src/game/scenes/ui/hud/currency_chip.gd, scenes/inventory/inventory_shop_screen.gd, scenes/progression/level_up_modal.gd
src/game/locale/{vi,en}.po, messages.pot, xx_PS.po; src/game/assets/LICENSES.md
src/game/tools/lib/screen_capture.gd
src/game/tests/{unit/core/test_outline_style.gd,scene/settings/test_settings_panel.gd,scene/test_grade_band_matrix.gd,fixtures/theme/tier3_tokens_0_16_0.json}
```

## Phase 0: Research

- `StyleBox.get_minimum_size()` dùng content margin khi đã đặt (bộ sinh luôn đặt) → cạnh dưới dày không làm control cao thêm nếu `content_margin_bottom` ≥ cạnh; chiều cao giữ bằng cách chia lại khoảng đệm trên/dưới.
- `StyleBoxFlat.expand_margin_top` nhận giá trị âm (kiểm bằng Godot 4.7.2: −4.0 giữ nguyên) → mặt nút nhấn vẽ thấp xuống mà control không đổi kích thước; bộ sinh ghi trực tiếp vào `.tres`.
- Nút bật/tắt (`toggle_mode`) được Godot vẽ bằng stylebox `pressed`/`hover_pressed` khi đang bật → chip đang chọn dùng hình "lún" (vàng, cạnh 1 px), thống nhất với cảm giác "đã được bấm".
- Icon Button được nhân màu (`icon_*_color`): icon trắng (`check.svg`, `tab_*`) nhuộm mực trên chip vàng và màu nhấn trên tab chưa mở; icon có màu riêng giữ nguyên (modulate trắng).
- `ThemeService` chỉ chép màu của Theme tương phản cao (sinh ở mật độ `g6_7`) → thêm chép bề dày viền trái/trên/phải và cộng phần chênh vào cạnh dưới; hộp tầng 3 có cùng bề dày ở hai Theme nên không đổi.
- Theme tương phản cao hiện chỉ có một tệp; token `high-contrast.json` thêm nhóm `density.<band>.outline.*` = 3 px.
- Label có `uppercase` (Godot 4.7) → tiêu đề in hoa không cần chuỗi in hoa riêng; `Control.accessibility_name` có sẵn cho trình đọc màn hình.
- Ma trận khối: nút `size_flags_horizontal = 0` + `autowrap` co về rộng icon (F098) → bỏ autowrap ở nút một dòng của trang Thông tin; văn bản giấy phép của engine có dòng kẻ 80 ký tự (giữ nguyên văn) — ma trận bỏ qua riêng lỗi "từ bị cắt" của các dòng kẻ đó.

## Phase 1: Design & Contracts

- Không contract API/event mới ([contracts/README.md](contracts/README.md)). Model hiển thị: [data-model.md](data-model.md). Quy trình duyệt: [quickstart.md](quickstart.md).
