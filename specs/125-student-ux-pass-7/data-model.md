# Data Model: F125 — Student UX pass 7: phóng to giao diện trên điện thoại và nút đóng đỏ cho mọi cửa sổ

> Feature chỉ có token, Theme và trạng thái RAM phía client; không bảng, không event, không dữ liệu học sinh, không trường cài đặt người dùng mới.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| Ngưỡng `compact` | `src/game/project.godot` | `eduworld/ui/compact_short_side` = 500 (cạnh ngắn, đơn vị độc lập mật độ; sở hữu `STUDENT_GAME_UX` §2) | cài đặt dự án |
| Token `compact` | `design/game-tokens/component.json`, `density/compact.json`, `primitive.json` | `component.compact.logical_height` = `{size.layout.540}`; `density.compact.button.min_height` = `{size.control.60}`; `density.compact.font.small` = `{size.font.18}` | `tokens@0.21.0` |
| Theme `compact` | `resources/ui/theme/eduworld_theme_<band>_compact.tres` (sinh, `compactMap`) | Theme của khối với `button.min_height` và `font.small` = max(khối, compact) | Theme |
| `UiTokens` | `scripts/core/ui_tokens.gd` (sinh) | `COMPONENT_COMPACT_LOGICAL_HEIGHT`, `DENSITY_COMPACT_BUTTON_MIN_HEIGHT`, `DENSITY_COMPACT_FONT_SMALL`, `COMPACT_THEME_PATHS` | — |
| `CompactPolicy` | `scripts/core/compact_policy.gd` (thuần) | `short_side_limit()`, `logical_height()`, `is_compact(window, density, keyboard)`, `content_scale(window, base, compact)`, `logical_size(window, base, factor)`, `physical_scale(window, base, factor)`, `button_target(band, compact)`, `font_floor(band, compact)` | — |
| `OrientationGuard` (autoload) | `scripts/core/orientation_guard.gd` | `compact`, `mode_scale(window)`; seam `get_density`, `get_keyboard_height`, `set_compact`; bỏ `_saved_content_scale` | RAM |
| `AppState` (autoload) | `scripts/core/app_state.gd` | `compact`, `set_compact(value)`, tín hiệu `compact_changed(compact)` | RAM |
| `ThemeService` | `scripts/core/theme_service.gd` | `compact`, `set_compact(enabled)` (Theme `compact`, khóa cache thêm `compact`, phát `density_changed`) | RAM |
| `UiBuild` | `scenes/ui/components/ui_build.gd` | `target()` theo chế độ, `target_for(band)`, `title_bar(title_key, icon) -> {bar,row,icon,title,close}`, `close_button()`, `fit_icon(rect)`; `window()` dùng `title_bar` (`header` = hàng của thanh tiêu đề) | — |
| `BandLayoutProbe` (TEST-ONLY) | `tests/fakes/band_layout_probe.gd` | `SIZES` (1280×720, 1280×750, 1168×540, 1110×540), `WINDOWS`, `LEGACY_PHONE`, `MIN_TEXT_CSS_PX` 12, `MIN_TOUCH_CSS_PX` 40, `compact_of(name)`, `css_scale_of(name)`, `check(..., compact, css_scale)`, `last_min_text_px`, `last_min_touch_px` | — |

## 2. Vòng đời / trạng thái

- Khởi động / mỗi lần cửa sổ đổi cỡ (`root.size_changed`) → `OrientationGuard.apply(window)`: `CompactPolicy.is_compact(window, density, keyboard)` → nếu đổi: `AppState.set_compact()` → `ThemeService.set_compact()` (Theme `compact` của khối, `density_changed`) → `compact_changed` (HUD đặt lại vùng chạm).
- Cửa sổ ngang: ẩn màn "Xoay ngang" (nếu đang hiện), ghi `content_scale_factor = mode_scale(window)` nếu khác (4/3 trên điện thoại, 1 trên máy tính/tablet).
- Cửa sổ dọc: hiện "Xoay ngang" với hệ số của nó (F113); quay lại ngang → hệ số của chế độ hiện hành.
- Cửa sổ: nút đỏ "✕ Đóng" hoặc Esc/Back → `close()` → `closed` (Nhóm: chỉ đóng cửa sổ, nhóm còn).
