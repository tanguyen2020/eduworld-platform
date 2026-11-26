# Data Model: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

> Feature chỉ có cấu hình hiển thị và trạng thái RAM phía client; không bảng, không event, không dữ liệu học sinh, không storage trình duyệt.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `display/window/handheld/orientation` | `project.godot` | `4` (sensor landscape) | cấu hình dự án |
| `OrientationPolicy` | `scripts/core/orientation_policy.gd` (tĩnh) | `is_portrait(view: Vector2) -> bool` (cao > rộng), `prompt_scale(view: Vector2, base_short: float) -> float` (cạnh ngắn / cạnh ngắn cơ sở, ≥ 1), `base_short() -> float` (`display/window/size/viewport_height`) | — |
| `OrientationGuard` | autoload `scripts/core/orientation_guard.gd` | `prompt_visible: bool`, `apply(view)`, `set_paused`/`is_paused` (Callable tiêm được), `lock_landscape` (Callable, Web); tín hiệu `prompt_shown()`, `prompt_hidden()` | RAM |
| `RotatePrompt` | `scenes/ui/rotate_prompt/RotatePrompt.tscn` + `rotate_prompt.gd` | view: `SkyBackdrop`, `SpeechBubble` (tiêu đề, lời phụ, dòng máy tính), Mây, icon `rotate_phone` | RAM |
| Shell `#ew-rotate` | `src/game/web/shell.html` | `data-portrait` trên `#ew-shell`; `STRINGS.{vi,en}.rotate_{title,hint}`; `isPortrait(w, h)`, `tryLockLandscape(win)` | DOM |
| Icon mới | `design/icons/game/rotate_phone.svg` | điện thoại + mũi tên xoay | `resources/ui/icons/` sinh |
| Khóa dịch | `locale/{vi,en}.po` | `ui.rotate.title`, `ui.rotate.hint`, `ui.rotate.desktop` | `.po` |
| `BandLayoutProbe.SIZES`, `PORTRAIT_SIZES` | `tests/fakes/band_layout_probe.gd` (TEST-ONLY) | ngang: `1280x720`, `1024x600`, `844x390`; dọc (chỉ màn nhắc): `720x1280`, `390x844` | — |

## 2. Vòng đời / trạng thái

- Cửa sổ đổi cỡ (`root.size_changed`) → `apply(view)`:
  - ngang → dọc: hiện màn nhắc (CanvasLayer lớp cao, `scale` = `prompt_scale`), nếu SceneTree chưa dừng thì dừng và nhớ "tôi đã dừng", Web: thử khóa ngang (im lặng), phát `prompt_shown`.
  - dọc → dọc (đổi cỡ): cập nhật phóng/kích thước màn nhắc.
  - dọc → ngang: ẩn màn nhắc, bỏ dừng chỉ khi chính guard đã dừng, phát `prompt_hidden`.
- Shell: `checking`/`loading`/`ready`/`starting` + `data-portrait="true"` → lời nhắc phủ bảng chờ; `unsupported`/`error` → không lời nhắc; "Chơi" → (cảm ứng) thử toàn màn hình + khóa ngang → khởi động engine.
