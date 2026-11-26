# Data Model: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

> Feature chỉ có cấu hình hiển thị và trạng thái RAM phía client; không bảng, không event, không dữ liệu học sinh, không storage trình duyệt.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `display/window/handheld/orientation` | `project.godot` | `4` (sensor landscape) | cấu hình dự án |
| `OrientationPolicy` | `scripts/core/orientation_policy.gd` (tĩnh) | `is_portrait(window: Vector2) -> bool` (cao > rộng), `prompt_scale(view: Vector2, base_short: float) -> float` (cạnh ngắn / cạnh ngắn cơ sở, ≥ 1), `prompt_content_scale(window: Vector2, base: Vector2) -> float` (hệ số `content_scale_factor` khi màn nhắc hiện: 16/9 cho mọi cửa sổ dọc giới hạn bởi bề ngang), `base_short()`, `base_size()` | — |
| `OrientationGuard` | autoload `scripts/core/orientation_guard.gd` | `prompt_visible: bool`, `apply(window)`, `set_paused`/`is_paused`, `set_content_scale`/`get_content_scale`, `lock_landscape` (Callable tiêm được); Theme màn nhắc = `ThemeService` mật độ `g1_3` + cỡ chữ/tương phản của người dùng; tín hiệu `prompt_shown()`, `prompt_hidden()` | RAM |
| `RotatePrompt` | `scenes/ui/rotate_prompt/RotatePrompt.tscn` + `rotate_prompt.gd` | view: `SkyBackdrop`, `SpeechBubble` (tiêu đề, lời phụ, dòng máy tính), Mây, icon `rotate_phone` | RAM |
| Shell `#ew-rotate` | `src/game/web/shell.html` | `data-portrait` trên `#ew-shell`; `STRINGS.{vi,en}.rotate_{title,hint}`; `isPortrait(w, h)`, `tryLockLandscape(win)` | DOM |
| Icon mới | `design/icons/game/rotate_phone.svg` | điện thoại + mũi tên xoay | `resources/ui/icons/` sinh |
| Khóa dịch | `locale/{vi,en}.po` | `ui.rotate.title`, `ui.rotate.hint`, `ui.rotate.desktop` | `.po` |
| `BandLayoutProbe.SIZES`, `PORTRAIT_SIZES` | `tests/fakes/band_layout_probe.gd` (TEST-ONLY) | ngang: `1280x720`, `1024x600`, `844x390`; dọc (chỉ màn nhắc): `720x1280`, `390x844` | — |

## 2. Vòng đời / trạng thái

- Cửa sổ đổi cỡ (`root.size_changed`) → `apply(root.size)`:
  - ngang → dọc: nhớ `content_scale_factor` hiện tại, đặt hệ số `prompt_content_scale`, hiện màn nhắc (CanvasLayer lớp 120, neo toàn khung), nếu SceneTree chưa dừng thì dừng và nhớ "tôi đã dừng", Web: thử khóa ngang (im lặng), phát `prompt_shown`.
  - dọc → dọc (đổi cỡ): tính lại hệ số, chỉ ghi khi khác (đặt hệ số làm cửa sổ phát `size_changed` lần nữa).
  - dọc → ngang: ẩn màn nhắc, trả `content_scale_factor` cũ, bỏ dừng chỉ khi chính guard đã dừng, phát `prompt_hidden`.
- Shell: cửa sổ ≥ 2:1 → Mây nhỏ (1/3 `--ew-component-mascot-size`), khoảng cách 8; `checking`/`loading`/`ready`/`starting` + `data-portrait="true"` → lời nhắc phủ bảng chờ; `unsupported`/`error` → không lời nhắc; "Chơi" → (cảm ứng) thử toàn màn hình + khóa ngang → khởi động engine.
