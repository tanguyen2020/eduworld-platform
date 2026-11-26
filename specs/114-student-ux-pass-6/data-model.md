# Data Model: F114 — Student UX pass 6: phong cách viền hoạt hình phân tầng và màn Cài đặt mới

> Feature chỉ có token, Theme và trạng thái RAM phía client; không bảng, không event, không dữ liệu học sinh, không trường cài đặt mới (giá trị vẫn ở `user://settings.cfg` như F060/F062).

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| Token viền | `design/game-tokens/semantic.json`, `density/*.json`, `high-contrast.json` | `color.outline` (`palette.ink.900`; tương phản cao `palette.black`), `color.overlay.scrim` (mực 50%), `color.overlay.scrim_nested` (mực 35%), `density.<band>.outline.interactive` 3/3/2/2, `.outline.info` 2, `.depth.rest` 5/4/3/3, `.depth.pressed` 1; tương phản cao `outline.*` = 3 mọi khối | `tokens@0.17.0` |
| Token component | `design/game-tokens/component.json` | `choice_chip.{bg,text,selected_bg,selected_text}`, `section_tag.{bg,text,border}`, `button.danger.{bg,bg_pressed,text}`, `toggle.{track_on,track_off,knob,border,text}`, `slider.{track,fill}`, `settings.{tab_column_bg,tab_column_width,tab_text,tab_hover_bg,tab_icon,tab_selected_bg,tab_selected_text,title_bg,title_text,confirm_title_bg,confirm_title_text,value_bg,value_text}`, `window.{pop_scale,pop_duration}`; `button.secondary.bg` → `palette.sky.100`; `panel/form_card/speech_bubble/info_banner/inline_error.border` → `color.outline` | `tokens@0.17.0` |
| Biến thể Theme | `resources/ui/theme/*.tres` (sinh) | Tầng 1: `Button`, `SecondaryButton`, `DangerButton`, `IconButton`, `HintButton`, `ChoiceChip`, `PictureTile`, `TabButton`/`TabButtonSelected`, `ToggleSwitch`, `HSlider`, khung `Panel`/`PanelContainer`, `FormCard`, `WindowTitleBar`, `ConfirmTitleBar`, `SettingsTabColumn`; tầng 2: `HudPanel`, `HudChip`, `SpeechBubble`, `InfoBannerPanel`, `InlineErrorPanel`, `ProgressBar`, `SectionTag`, `ValueChip`; `Scrim`, `ScrimNested`; tầng 3 không đổi | Theme |
| `UiTokens` | `scripts/core/ui_tokens.gd` (sinh) | hằng mới (`COLOR_OUTLINE`, `COLOR_OVERLAY_*`, `COMPONENT_*`), `DENSITY_OUTLINE_INTERACTIVE`, `DENSITY_OUTLINE_INFO`, `DENSITY_DEPTH_REST`, `DENSITY_DEPTH_PRESSED` | — |
| `ThemeService` | `scripts/core/theme_service.gd` | `_apply_outline(target, source)`: tương phản cao chép bề dày viền, cạnh dưới += phần chênh | RAM |
| `Settings` | `scripts/core/settings.gd` (autoload) | `VOLUME_STEP = 0.1`, `nudge_volume(bus_key, steps)` (làm tròn theo bước, 0…1, lưu như `set_volume`) | `user://settings.cfg` `[audio]` (có sẵn) |
| `UiMotion` | `scenes/ui/components/ui_motion.gd` | `POP_SCALE` 0,96, `POP_DURATION` 0,2 s, `pop_in(window, reduce_motion) -> Tween` (null khi Giảm chuyển động) | — |
| `ToggleSwitch` | `scenes/ui/components/toggle_switch.gd` (CheckButton) | `setting_key`, `show_state(on)`, text `ui.settings.on/off`, `accessibility_name` "{tên}: {Bật/Tắt}" | RAM |
| `VolumeRow` | `scenes/ui/components/volume_row.gd` | `bus_key`, `minus`, `plus`, `slider`, `value_label`; `setup(key, settings)`, `render()` | RAM |
| `SettingsPanel` | `scenes/ui/settings/SettingsPanel.tscn` + `settings_panel.gd` | `current_tab` (`language` lần đầu, rồi tab dùng gần nhất), `open_tween`, `show_tab(tab)`, `tab_button(tab)`, `page(tab)`, `font_chips()`, `volume_row(key)`, `show_licenses()`, `show_main()`, `go_back()`, `confirm_visible()`, `licenses_visible()` | RAM (panel sống cùng WorldRoot) |
| `QualitySettings` | `scenes/ui/settings/QualitySettings.tscn` + `quality_settings.gd` | 3 chip preset có hình (`quality_low/medium/high`), `Toggles` (ẩn ở `g1_3`), `toggles_shown()` | QualityManager (có sẵn) |
| `LogoutConfirm` | `scenes/ui/dialogs/LogoutConfirm.tscn` + `logout_confirm.gd` | Control toàn khung: `ScrimNested`, `Dialog`, `Cancel` (focus), `Confirm`; `open()`, `cancel()`, tín hiệu `dismissed` | RAM |
| Icon mới | `design/icons/game/*.svg` | `tab_{language,graphics,sound,access,info}` (trắng, nhuộm qua Theme), `exit_light`, `cancel`, `minus`, `plus`, `music`, `voice`, `quality_{low,medium,high}`, `toggle_{on,off}`, `slider_knob` | `resources/ui/icons/` sinh |
| Khóa dịch | `locale/{vi,en}.po` | `ui.settings.tab.*`, `ui.settings.volume.{title,music,sfx,voice,down,up}`, `ui.settings.font_scale{,.normal,.large,.larger}`, `ui.settings.display`, `ui.settings.high_contrast`, `ui.settings.reduce_motion`, `ui.settings.on/off`, `ui.settings.logout_title`, `ui.settings.logout_saved`; msgstr đổi: `ui.settings.language` (song ngữ), `ui.settings.logout_confirm`, `ui.settings.logout_cancel` ("Hủy") | `.po` |
| Fixture tầng 3 | `tests/fixtures/theme/tier3_tokens_0_16_0.json` (TEST-ONLY) | giá trị StyleBox/màu/cỡ chữ/hằng của 12 biến thể tầng 3 trong 6 Theme của `tokens@0.16.0` | — |

## 2. Vòng đời / trạng thái

- Mở Cài đặt (`EventBus.ui_requested(&"settings")` → `open()`): trang chính (đóng trang Giấy phép), dựng lại giá trị từ `Settings`/`QualityManager`, hiện tab `current_tab` (tab chọn = `TabButtonSelected`, trang tương ứng hiện, focus vào tab), phóng cửa sổ 0,96 → 1 (bỏ khi Giảm chuyển động).
- Chạm tab → `current_tab` đổi; đóng rồi mở lại trong phiên → mở ở tab đó.
- −/+ → `Settings.nudge_volume` (±10%, làm tròn) → `volume_changed` → `AudioDirector`; kéo thanh → `Settings.set_volume`; 0% / 100% → nút −/+ vô hiệu.
- Đổi cỡ chữ / tương phản cao → `Settings` → `ThemeService` dựng Theme mới cho cửa sổ; Cài đặt giữ tab đang mở.
- Đăng xuất → `LogoutConfirm.open()` (lớp phủ lồng, focus "Hủy") → "Hủy"/Esc: `cancel()` → `dismissed` → focus về nút Đăng xuất; "Đăng xuất": `AuthService.logout()` (không đổi).
- Esc/Back: hộp xác nhận → trang Giấy phép → đóng cửa sổ (`closed`).
